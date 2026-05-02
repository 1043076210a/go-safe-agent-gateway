package executor

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-safe-agent-gateway/internal/policy"
	"go-safe-agent-gateway/internal/repository"
	"go-safe-agent-gateway/internal/tool"
)

type fakeTool struct {
	name       string
	schema     map[string]any
	permission tool.PermissionLevel
	timeout    time.Duration
	async      bool
	fn         func(context.Context, map[string]any) (*tool.ToolResult, error)
}

func (f fakeTool) Name() string                { return f.name }
func (f fakeTool) Description() string         { return "fake" }
func (f fakeTool) InputSchema() map[string]any { return f.schema }
func (f fakeTool) Permission() tool.PermissionLevel {
	if f.permission == "" {
		return tool.PermissionReadOnly
	}
	return f.permission
}
func (f fakeTool) Timeout() time.Duration { return f.timeout }
func (f fakeTool) IsAsync() bool          { return f.async }
func (f fakeTool) Execute(ctx context.Context, input map[string]any) (*tool.ToolResult, error) {
	return f.fn(ctx, input)
}

func newTestExecutor(t *testing.T, testTool fakeTool) (*GatewayExecutor, *repository.MemoryStore) {
	t.Helper()
	reg := tool.NewRegistry()
	if err := reg.Register(testTool); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	store := repository.NewMemoryStore()
	policies := policy.NewEngine(policy.Config{AllowedTools: []string{testTool.Name()}})
	return New(reg, policies, store, Options{DefaultTimeout: time.Second, AsyncWorkers: 1, AsyncQueueSize: 2}), store
}

func TestExecutor_WhenInvalidJSONSchemaInput_ShouldReject(t *testing.T) {
	exec, _ := newTestExecutor(t, fakeTool{
		name: "calculator",
		schema: map[string]any{
			"type":       "object",
			"required":   []any{"x"},
			"properties": map[string]any{"x": map[string]any{"type": "number"}},
		},
		fn: func(context.Context, map[string]any) (*tool.ToolResult, error) {
			t.Fatal("tool should not execute")
			return nil, nil
		},
	})
	_, err := exec.Execute(context.Background(), &ExecuteRequest{UserID: "user-1", ToolName: "calculator", Input: map[string]any{"x": "bad"}})
	if err == nil {
		t.Fatal("expected schema validation error")
	}
}

func TestExecutor_WhenPolicyRejects_ShouldPersistReject(t *testing.T) {
	exec, store := newTestExecutor(t, fakeTool{
		name:       "query_mysql_readonly",
		permission: tool.PermissionSensitive,
		schema:     map[string]any{"type": "object"},
		fn: func(context.Context, map[string]any) (*tool.ToolResult, error) {
			t.Fatal("tool should not execute")
			return nil, nil
		},
	})
	_, err := exec.Execute(context.Background(), &ExecuteRequest{UserID: "user-1", ToolName: "query_mysql_readonly", Input: map[string]any{"sql": "SELECT * FROM users LIMIT 1"}})
	if err == nil {
		t.Fatal("expected policy rejection")
	}
	rejects, err := store.ListPolicyRejects(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("list rejects: %v", err)
	}
	if len(rejects) != 1 {
		t.Fatalf("expected one policy reject, got %d", len(rejects))
	}
}

func TestExecutor_WhenToolPanic_ShouldRecover(t *testing.T) {
	exec, store := newTestExecutor(t, fakeTool{
		name:   "panic_tool",
		schema: map[string]any{"type": "object"},
		fn: func(context.Context, map[string]any) (*tool.ToolResult, error) {
			panic("boom")
		},
	})
	resp, err := exec.Execute(context.Background(), &ExecuteRequest{UserID: "user-1", ToolName: "panic_tool", Input: map[string]any{}})
	if err == nil || resp.Success {
		t.Fatalf("expected recovered panic failure, resp=%+v err=%v", resp, err)
	}
	calls, err := store.ListToolCalls(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("list audits: %v", err)
	}
	if len(calls) != 1 || calls[0].ErrorMessage == "" {
		t.Fatalf("expected failed audit, got %+v", calls)
	}
}

func TestExecutor_WhenToolTimeout_ShouldReturnFailure(t *testing.T) {
	exec, _ := newTestExecutor(t, fakeTool{
		name:    "slow_tool",
		schema:  map[string]any{"type": "object"},
		timeout: 10 * time.Millisecond,
		fn: func(ctx context.Context, input map[string]any) (*tool.ToolResult, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})
	resp, err := exec.Execute(context.Background(), &ExecuteRequest{UserID: "user-1", ToolName: "slow_tool", Input: map[string]any{}})
	if err == nil || resp.Error != "tool timeout" {
		t.Fatalf("expected timeout, resp=%+v err=%v", resp, err)
	}
}

func TestExecutor_WhenToolOutputHasSensitiveField_ShouldMask(t *testing.T) {
	exec, _ := newTestExecutor(t, fakeTool{
		name:   "profile",
		schema: map[string]any{"type": "object"},
		fn: func(context.Context, map[string]any) (*tool.ToolResult, error) {
			return &tool.ToolResult{ToolName: "profile", Success: true, Data: map[string]any{"email": "user@example.com", "name": "Ada"}}, nil
		},
	})
	resp, err := exec.Execute(context.Background(), &ExecuteRequest{UserID: "user-1", ToolName: "profile", Input: map[string]any{}})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	data := resp.Data.(map[string]any)
	if data["email"] != "***" {
		t.Fatalf("expected masked output, got %+v", data)
	}
}

func TestExecutor_WhenAsyncToolSubmitted_ShouldTransitionToSuccess(t *testing.T) {
	exec, store := newTestExecutor(t, fakeTool{
		name:   "async_tool",
		async:  true,
		schema: map[string]any{"type": "object"},
		fn: func(context.Context, map[string]any) (*tool.ToolResult, error) {
			return &tool.ToolResult{ToolName: "async_tool", Success: true, Data: map[string]any{"ok": true}}, nil
		},
	})
	task, err := exec.SubmitAsync(context.Background(), &ExecuteRequest{UserID: "user-1", ToolName: "async_tool", Input: map[string]any{}})
	if err != nil {
		t.Fatalf("submit async: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		got, err := store.GetAsyncTask(context.Background(), task.TaskID)
		if err != nil {
			t.Fatalf("get task: %v", err)
		}
		if got.Status == "success" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	got, err := store.GetAsyncTask(context.Background(), task.TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	t.Fatalf("expected success task, got %+v", got)
}

func TestExecutor_WhenToolReturnsError_ShouldAuditFailure(t *testing.T) {
	exec, store := newTestExecutor(t, fakeTool{
		name:   "error_tool",
		schema: map[string]any{"type": "object"},
		fn: func(context.Context, map[string]any) (*tool.ToolResult, error) {
			return nil, errors.New("backend failed")
		},
	})
	_, err := exec.Execute(context.Background(), &ExecuteRequest{UserID: "user-1", ToolName: "error_tool", Input: map[string]any{}})
	if err == nil {
		t.Fatal("expected tool error")
	}
	calls, err := store.ListToolCalls(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("list audits: %v", err)
	}
	if len(calls) != 1 || calls[0].Status != "failed" {
		t.Fatalf("expected failed audit, got %+v", calls)
	}
}
