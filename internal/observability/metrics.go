package observability

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Metrics struct {
	AgentRequests     *prometheus.CounterVec
	LLMRequests       *prometheus.CounterVec
	ToolCalls         *prometheus.CounterVec
	ToolErrors        *prometheus.CounterVec
	PolicyRejects     *prometheus.CounterVec
	AsyncTasks        *prometheus.CounterVec
	LLMDuration       *prometheus.HistogramVec
	ToolDuration      *prometheus.HistogramVec
	RAGDuration       *prometheus.HistogramVec
	PolicyDuration    *prometheus.HistogramVec
	HTTPDuration      *prometheus.HistogramVec
	AsyncQueueSize    prometheus.Gauge
	ActiveWorkerCount prometheus.Gauge
}

func NewMetrics() *Metrics {
	m := &Metrics{
		AgentRequests:     prometheus.NewCounterVec(prometheus.CounterOpts{Name: "agent_requests_total", Help: "Agent chat requests."}, []string{"status"}),
		LLMRequests:       prometheus.NewCounterVec(prometheus.CounterOpts{Name: "llm_requests_total", Help: "LLM requests."}, []string{"status"}),
		ToolCalls:         prometheus.NewCounterVec(prometheus.CounterOpts{Name: "tool_calls_total", Help: "Tool calls."}, []string{"tool_name", "status"}),
		ToolErrors:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "tool_errors_total", Help: "Tool errors."}, []string{"tool_name", "error_type"}),
		PolicyRejects:     prometheus.NewCounterVec(prometheus.CounterOpts{Name: "policy_reject_total", Help: "Policy rejects."}, []string{"tool_name"}),
		AsyncTasks:        prometheus.NewCounterVec(prometheus.CounterOpts{Name: "async_tasks_total", Help: "Async tasks."}, []string{"status"}),
		LLMDuration:       prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "llm_request_duration_seconds", Help: "LLM duration."}, []string{"status"}),
		ToolDuration:      prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "tool_call_duration_seconds", Help: "Tool duration."}, []string{"tool_name", "status"}),
		RAGDuration:       prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "rag_search_duration_seconds", Help: "RAG duration."}, []string{"status"}),
		PolicyDuration:    prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "policy_check_duration_seconds", Help: "Policy duration."}, []string{"status"}),
		HTTPDuration:      prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "HTTP duration."}, []string{"method", "route", "status"}),
		AsyncQueueSize:    prometheus.NewGauge(prometheus.GaugeOpts{Name: "async_task_queue_size", Help: "Async queue size."}),
		ActiveWorkerCount: prometheus.NewGauge(prometheus.GaugeOpts{Name: "active_worker_count", Help: "Active workers."}),
	}
	prometheus.MustRegister(m.AgentRequests, m.LLMRequests, m.ToolCalls, m.ToolErrors, m.PolicyRejects, m.AsyncTasks, m.LLMDuration, m.ToolDuration, m.RAGDuration, m.PolicyDuration, m.HTTPDuration, m.AsyncQueueSize, m.ActiveWorkerCount)
	return m
}

func InitTracer(ctx context.Context) (func(context.Context) error, error) {
	exp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp), sdktrace.WithResource(resource.Default()))
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

func Observe(h *prometheus.HistogramVec, labels ...string) func() {
	start := time.Now()
	return func() { h.WithLabelValues(labels...).Observe(time.Since(start).Seconds()) }
}
