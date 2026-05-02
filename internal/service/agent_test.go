package service

import (
	"context"
	"testing"
	"time"

	"go-safe-agent-gateway/internal/executor"
	"go-safe-agent-gateway/internal/llm"
	"go-safe-agent-gateway/internal/model"
	"go-safe-agent-gateway/internal/policy"
	"go-safe-agent-gateway/internal/repository"
	"go-safe-agent-gateway/internal/tool"
)

func TestAgentService_WhenExecuteTool_ShouldUseExecutor(t *testing.T) {
	store := repository.NewMemoryStore()
	reg := tool.NewRegistry()
	if err := reg.Register(tool.CalculatorTool{}); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	exec := executor.New(reg, policy.NewEngine(policy.Config{AllowedTools: []string{"calculator"}}), store, executor.Options{DefaultTimeout: time.Second})
	svc := NewAgentService(store, reg, exec, llm.MockClient{}, nil)

	out, err := svc.ExecuteTool(context.Background(), ExecuteToolRequest{
		UserID:   "user-1",
		ToolName: "calculator",
		Input:    map[string]any{"operation": "multiply", "a": 4.0, "b": 5.0},
	})
	if err != nil {
		t.Fatalf("execute tool: %v", err)
	}
	resp := out.(*executor.ExecuteResponse)
	if !resp.Success {
		t.Fatalf("expected success response: %+v", resp)
	}
}

func TestAgentService_WhenChat_ShouldPersistMessages(t *testing.T) {
	store := repository.NewMemoryStore()
	reg := tool.NewRegistry()
	if err := reg.Register(tool.CalculatorTool{}); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	exec := executor.New(reg, policy.NewEngine(policy.Config{AllowedTools: []string{"calculator"}}), store, executor.Options{DefaultTimeout: time.Second})
	svc := NewAgentService(store, reg, exec, llm.MockClient{}, nil)

	resp, err := svc.Chat(context.Background(), ChatRequest{UserID: "user-1", Message: "calculate"})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	messages, err := store.ListMessages(context.Background(), resp.SessionID, 10, 0)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected user and assistant messages, got %d", len(messages))
	}
}

func TestAgentService_WhenGetAsyncTask_ShouldReturnTask(t *testing.T) {
	store := repository.NewMemoryStore()
	task := &model.AsyncTask{TaskID: "task-1", UserID: "user-1", ToolName: "query_logs", Status: "success"}
	if err := store.CreateAsyncTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	svc := NewAgentService(store, tool.NewRegistry(), nil, llm.MockClient{}, nil)
	got, err := svc.GetAsyncTask(context.Background(), "task-1")
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Status != "success" {
		t.Fatalf("unexpected task: %+v", got)
	}
}
