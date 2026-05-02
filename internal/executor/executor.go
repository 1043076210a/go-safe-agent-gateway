package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/xeipuuv/gojsonschema"
	"go.opentelemetry.io/otel"

	"go-safe-agent-gateway/internal/model"
	"go-safe-agent-gateway/internal/observability"
	"go-safe-agent-gateway/internal/policy"
	"go-safe-agent-gateway/internal/repository"
	"go-safe-agent-gateway/internal/tool"
)

type Executor interface {
	Execute(ctx context.Context, req *ExecuteRequest) (*ExecuteResponse, error)
	SubmitAsync(ctx context.Context, req *ExecuteRequest) (*AsyncTask, error)
}

type ExecuteRequest struct {
	UserID    string
	SessionID string
	MessageID string
	ToolName  string
	Input     map[string]any
}

type ExecuteResponse struct {
	CallID         string `json:"call_id"`
	ToolName       string `json:"tool_name"`
	Success        bool   `json:"success"`
	Data           any    `json:"data,omitempty"`
	Error          string `json:"error,omitempty"`
	DurationMs     int64  `json:"duration_ms"`
	PolicyDecision string `json:"policy_decision"`
}

type AsyncTask = model.AsyncTask

type Options struct {
	DefaultTimeout time.Duration
	AsyncWorkers   int
	AsyncQueueSize int
	Metrics        *observability.Metrics
}

type GatewayExecutor struct {
	registry       tool.Registry
	policies       policy.PolicyEngine
	store          repository.Store
	defaultTimeout time.Duration
	metrics        *observability.Metrics
	queue          chan asyncJob
}

type asyncJob struct {
	taskID string
	req    ExecuteRequest
}

var (
	ErrToolNotFound = errors.New("tool not found")
	ErrQueueFull    = errors.New("async queue is full")
)

func New(registry tool.Registry, policies policy.PolicyEngine, store repository.Store, opts Options) *GatewayExecutor {
	if opts.DefaultTimeout <= 0 {
		opts.DefaultTimeout = 5 * time.Second
	}
	exec := &GatewayExecutor{
		registry:       registry,
		policies:       policies,
		store:          store,
		defaultTimeout: opts.DefaultTimeout,
		metrics:        opts.Metrics,
	}
	if opts.AsyncWorkers > 0 && opts.AsyncQueueSize > 0 {
		exec.queue = make(chan asyncJob, opts.AsyncQueueSize)
		for i := 0; i < opts.AsyncWorkers; i++ {
			go exec.worker()
		}
	}
	return exec
}

func (e *GatewayExecutor) Execute(ctx context.Context, req *ExecuteRequest) (resp *ExecuteResponse, err error) {
	if req == nil {
		return nil, errors.New("missing execute request")
	}
	start := time.Now()
	callID := repository.NewID()
	resp = &ExecuteResponse{CallID: callID, ToolName: req.ToolName, PolicyDecision: "not_checked"}
	audit := model.ToolCallAudit{
		CallID:    callID,
		SessionID: req.SessionID,
		MessageID: req.MessageID,
		UserID:    req.UserID,
		ToolName:  req.ToolName,
		Status:    "failed",
		InputJSON: mustJSON(maskSensitive(req.Input, nil)),
	}
	defer func() {
		resp.DurationMs = time.Since(start).Milliseconds()
		audit.DurationMs = resp.DurationMs
		if resp.Error != "" {
			audit.ErrorMessage = resp.Error
		}
		err = e.saveAudit(ctx, audit, err)
	}()

	ctx, span := otel.Tracer("go-safe-agent-gateway").Start(ctx, "tool.execute")
	defer span.End()

	t, ok := e.registry.Get(req.ToolName)
	if !ok {
		resp.Error = ErrToolNotFound.Error()
		return resp, ErrToolNotFound
	}
	if validationErr := validateInput(t.InputSchema(), req.Input); validationErr != nil {
		resp.Error = validationErr.Error()
		return resp, validationErr
	}
	decision, err := e.policies.Check(ctx, &policy.PolicyRequest{
		UserID:     req.UserID,
		SessionID:  req.SessionID,
		ToolName:   req.ToolName,
		Permission: t.Permission(),
		Input:      req.Input,
	})
	if err != nil {
		resp.Error = "policy check failed"
		return resp, err
	}
	if decision == nil || !decision.Allowed {
		resp.PolicyDecision = "rejected"
		reason := "policy rejected"
		if decision != nil && decision.Reason != "" {
			reason = decision.Reason
			audit.SanitizedInputJSON = mustJSON(decision.SanitizedInput)
			if saveErr := e.store.SavePolicyReject(ctx, policy.PolicyRejectModel(&policy.PolicyRequest{
				UserID:    req.UserID,
				SessionID: req.SessionID,
				ToolName:  req.ToolName,
				Input:     req.Input,
			}, decision)); saveErr != nil {
				resp.Error = saveErr.Error()
				return resp, saveErr
			}
		}
		resp.Error = reason
		return resp, errors.New(reason)
	}
	resp.PolicyDecision = "allowed"
	audit.SanitizedInputJSON = mustJSON(decision.SanitizedInput)

	timeout := t.Timeout()
	if timeout <= 0 {
		timeout = e.defaultTimeout
	}
	toolCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, execErr := executeTool(toolCtx, t, decision.SanitizedInput)
	if execErr != nil {
		resp.Error = execErr.Error()
		if errors.Is(toolCtx.Err(), context.DeadlineExceeded) {
			resp.Error = "tool timeout"
		}
		e.recordMetrics(t.Name(), "failed", resp.Error, start)
		return resp, errors.New(resp.Error)
	}
	if result == nil {
		resp.Error = "tool returned nil result"
		e.recordMetrics(t.Name(), "failed", resp.Error, start)
		return resp, errors.New(resp.Error)
	}
	resp.Success = result.Success
	if result.Success {
		audit.Status = "success"
		resp.Data = maskSensitive(result.Data, decision.MaskFields)
		e.recordMetrics(t.Name(), "success", "", start)
		return resp, nil
	}
	resp.Error = result.Error
	if resp.Error == "" {
		resp.Error = "tool failed"
	}
	e.recordMetrics(t.Name(), "failed", resp.Error, start)
	return resp, errors.New(resp.Error)
}

func (e *GatewayExecutor) SubmitAsync(ctx context.Context, req *ExecuteRequest) (*AsyncTask, error) {
	if e.queue == nil {
		return nil, errors.New("async executor is not configured")
	}
	t, ok := e.registry.Get(req.ToolName)
	if !ok {
		return nil, ErrToolNotFound
	}
	if !t.IsAsync() {
		return nil, errors.New("tool is not async")
	}
	task := &model.AsyncTask{
		TaskID:    repository.NewID(),
		UserID:    req.UserID,
		ToolName:  req.ToolName,
		Status:    "pending",
		InputJSON: mustJSON(maskSensitive(req.Input, nil)),
	}
	if err := e.store.CreateAsyncTask(ctx, task); err != nil {
		return nil, err
	}
	job := asyncJob{taskID: task.TaskID, req: *req}
	select {
	case e.queue <- job:
		if e.metrics != nil {
			e.metrics.AsyncQueueSize.Set(float64(len(e.queue)))
		}
		return task, nil
	default:
		task.Status = "failed"
		task.ErrorMessage = ErrQueueFull.Error()
		if err := e.store.UpdateAsyncTask(ctx, task); err != nil {
			return nil, err
		}
		return nil, ErrQueueFull
	}
}

func (e *GatewayExecutor) worker() {
	for job := range e.queue {
		if e.metrics != nil {
			e.metrics.AsyncQueueSize.Set(float64(len(e.queue)))
			e.metrics.ActiveWorkerCount.Inc()
		}
		e.runAsyncJob(job)
		if e.metrics != nil {
			e.metrics.ActiveWorkerCount.Dec()
		}
	}
}

func (e *GatewayExecutor) runAsyncJob(job asyncJob) {
	timeout := e.defaultTimeout + time.Minute
	if t, ok := e.registry.Get(job.req.ToolName); ok && t.Timeout() > e.defaultTimeout {
		timeout = t.Timeout() + time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	task, err := e.store.GetAsyncTask(ctx, job.taskID)
	if err != nil {
		return
	}
	task.Status = "running"
	if err := e.store.UpdateAsyncTask(ctx, task); err != nil {
		return
	}
	resp, err := e.Execute(ctx, &job.req)
	now := time.Now()
	task.FinishedAt = &now
	if err != nil {
		task.Status = "failed"
		if resp != nil && resp.Error == "tool timeout" {
			task.Status = "timeout"
		}
		task.ErrorMessage = err.Error()
	} else {
		task.Status = "success"
		task.ResultJSON = mustJSON(resp)
	}
	if err := e.store.UpdateAsyncTask(ctx, task); err != nil {
		return
	}
	if e.metrics != nil {
		e.metrics.AsyncTasks.WithLabelValues(task.Status).Inc()
	}
}

func (e *GatewayExecutor) saveAudit(ctx context.Context, audit model.ToolCallAudit, prior error) error {
	auditCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	if audit.SanitizedInputJSON == "" {
		audit.SanitizedInputJSON = audit.InputJSON
	}
	if err := e.store.SaveToolAudit(auditCtx, audit); err != nil {
		if prior != nil {
			return fmt.Errorf("%w; save audit: %v", prior, err)
		}
		return fmt.Errorf("save audit: %w", err)
	}
	return prior
}

func executeTool(ctx context.Context, t tool.Tool, input map[string]any) (result *tool.ToolResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("tool panic recovered")
		}
	}()
	return t.Execute(ctx, input)
}

func validateInput(schema map[string]any, input map[string]any) error {
	if schema == nil {
		schema = map[string]any{"type": "object"}
	}
	result, err := gojsonschema.Validate(gojsonschema.NewGoLoader(schema), gojsonschema.NewGoLoader(input))
	if err != nil {
		return err
	}
	if result.Valid() {
		return nil
	}
	return fmt.Errorf("invalid tool input: %s", result.Errors()[0].String())
}

func (e *GatewayExecutor) recordMetrics(toolName, status, errType string, start time.Time) {
	if e.metrics == nil {
		return
	}
	e.metrics.ToolCalls.WithLabelValues(toolName, status).Inc()
	e.metrics.ToolDuration.WithLabelValues(toolName, status).Observe(time.Since(start).Seconds())
	if errType != "" {
		e.metrics.ToolErrors.WithLabelValues(toolName, boundedErrorType(errType)).Inc()
	}
}

func boundedErrorType(msg string) string {
	switch {
	case msg == "tool timeout":
		return "timeout"
	case msg == "tool panic recovered":
		return "panic"
	case msg == ErrToolNotFound.Error():
		return "not_found"
	default:
		return "tool_error"
	}
}

func maskSensitive(v any, fields []string) any {
	switch typed := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for k, val := range typed {
			if policy.IsSensitiveField(k) || contains(fields, k) {
				out[k] = "***"
				continue
			}
			out[k] = maskSensitive(val, fields)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, val := range typed {
			out[i] = maskSensitive(val, fields)
		}
		return out
	default:
		return typed
	}
}

func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
