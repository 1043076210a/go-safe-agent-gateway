package service

import (
	"context"
	"errors"

	"go-safe-agent-gateway/internal/executor"
	"go-safe-agent-gateway/internal/llm"
	"go-safe-agent-gateway/internal/model"
	"go-safe-agent-gateway/internal/rag"
	"go-safe-agent-gateway/internal/repository"
	"go-safe-agent-gateway/internal/tool"
)

type AgentService struct {
	store    repository.Store
	registry tool.Registry
	executor executor.Executor
	llm      llm.Client
	rag      *rag.Service
}

func NewAgentService(store repository.Store, registry tool.Registry, exec executor.Executor, llmClient llm.Client, ragService *rag.Service) *AgentService {
	return &AgentService{store: store, registry: registry, executor: exec, llm: llmClient, rag: ragService}
}

type ExecuteToolRequest struct {
	UserID    string         `json:"user_id"`
	SessionID string         `json:"session_id"`
	MessageID string         `json:"message_id"`
	ToolName  string         `json:"tool_name"`
	Input     map[string]any `json:"input"`
	Async     bool           `json:"async"`
}

type ChatRequest struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

type ChatResponse struct {
	SessionID  string                    `json:"session_id"`
	MessageID  string                    `json:"message_id"`
	ToolCall   *llm.ToolCall             `json:"tool_call,omitempty"`
	ToolResult *executor.ExecuteResponse `json:"tool_result,omitempty"`
	Answer     string                    `json:"answer"`
}

func (s *AgentService) Health(ctx context.Context) error {
	return s.store.Health(ctx)
}

func (s *AgentService) ListTools(ctx context.Context) []tool.ToolMeta {
	return s.registry.List()
}

func (s *AgentService) ExecuteTool(ctx context.Context, req ExecuteToolRequest) (any, error) {
	if req.ToolName == "" {
		return nil, errors.New("tool_name is required")
	}
	if req.Input == nil {
		req.Input = map[string]any{}
	}
	execReq := &executor.ExecuteRequest{
		UserID:    req.UserID,
		SessionID: req.SessionID,
		MessageID: req.MessageID,
		ToolName:  req.ToolName,
		Input:     req.Input,
	}
	if req.Async {
		return s.executor.SubmitAsync(ctx, execReq)
	}
	return s.executor.Execute(ctx, execReq)
}

func (s *AgentService) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	if req.Message == "" {
		return nil, errors.New("message is required")
	}
	sessionID := req.SessionID
	if sessionID == "" {
		session := &model.Session{UserID: req.UserID, Title: "agent session"}
		if err := s.store.CreateSession(ctx, session); err != nil {
			return nil, err
		}
		sessionID = session.ID
	}
	msg := &model.Message{SessionID: sessionID, Role: "user", Content: req.Message}
	if err := s.store.CreateMessage(ctx, msg); err != nil {
		return nil, err
	}
	call, err := s.llm.GenerateToolCall(ctx, llm.ToolCallRequest{UserMessage: req.Message, Tools: s.registry.List()})
	if err != nil {
		return nil, err
	}
	if call == nil {
		answer := "No backend tool call was generated."
		_ = s.store.CreateMessage(ctx, &model.Message{SessionID: sessionID, Role: "assistant", Content: answer})
		return &ChatResponse{SessionID: sessionID, MessageID: msg.ID, Answer: answer}, nil
	}
	toolResp, err := s.executor.Execute(ctx, &executor.ExecuteRequest{
		UserID:    req.UserID,
		SessionID: sessionID,
		MessageID: msg.ID,
		ToolName:  call.Name,
		Input:     call.Input,
	})
	if err != nil && toolResp == nil {
		return nil, err
	}
	answer, finalErr := s.llm.GenerateFinalAnswer(ctx, llm.FinalAnswerRequest{UserMessage: req.Message, ToolCall: call, ToolResult: toolResp})
	if finalErr != nil {
		answer = "The tool executed, but final answer generation failed: " + finalErr.Error()
	}
	if err := s.store.CreateMessage(ctx, &model.Message{SessionID: sessionID, Role: "assistant", Content: answer}); err != nil {
		return nil, err
	}
	return &ChatResponse{SessionID: sessionID, MessageID: msg.ID, ToolCall: call, ToolResult: toolResp, Answer: answer}, nil
}

func (s *AgentService) IndexDocument(ctx context.Context, req rag.IndexRequest) (*rag.IndexResponse, error) {
	if s.rag == nil {
		return nil, errors.New("rag service is not configured")
	}
	return s.rag.IndexDocument(ctx, req)
}

func (s *AgentService) CreateSession(ctx context.Context, userID, title string) (*model.Session, error) {
	session := &model.Session{UserID: userID, Title: title}
	if err := s.store.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *AgentService) ListMessages(ctx context.Context, sessionID string, limit, offset int) ([]model.Message, error) {
	return s.store.ListMessages(ctx, sessionID, limit, offset)
}

func (s *AgentService) ListToolCalls(ctx context.Context, limit, offset int) ([]model.ToolCallAudit, error) {
	return s.store.ListToolCalls(ctx, limit, offset)
}

func (s *AgentService) ListPolicyRejects(ctx context.Context, limit, offset int) ([]model.PolicyReject, error) {
	return s.store.ListPolicyRejects(ctx, limit, offset)
}

func (s *AgentService) GetAsyncTask(ctx context.Context, taskID string) (*model.AsyncTask, error) {
	if taskID == "" {
		return nil, errors.New("task id is required")
	}
	return s.store.GetAsyncTask(ctx, taskID)
}
