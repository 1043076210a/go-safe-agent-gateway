package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go-safe-agent-gateway/internal/tool"
)

type Client interface {
	GenerateToolCall(ctx context.Context, req ToolCallRequest) (*ToolCall, error)
	GenerateFinalAnswer(ctx context.Context, req FinalAnswerRequest) (string, error)
}

type ToolCallRequest struct {
	UserMessage string
	Tools       []tool.ToolMeta
}

type FinalAnswerRequest struct {
	UserMessage string
	ToolCall    *ToolCall
	ToolResult  any
}

type ToolCall struct {
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

type OpenAICompatibleClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewOpenAICompatibleClient(baseURL, apiKey, model string, timeout time.Duration) *OpenAICompatibleClient {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &OpenAICompatibleClient{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, model: model, httpClient: &http.Client{Timeout: timeout}}
}

func (c *OpenAICompatibleClient) GenerateToolCall(ctx context.Context, req ToolCallRequest) (*ToolCall, error) {
	if c.apiKey == "" {
		return nil, errors.New("llm api key is empty")
	}
	tools := make([]chatTool, 0, len(req.Tools))
	for _, meta := range req.Tools {
		tools = append(tools, chatTool{
			Type: "function",
			Function: chatFunction{
				Name:        meta.Name,
				Description: meta.Description,
				Parameters:  meta.InputSchema,
			},
		})
	}
	payload := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: "You produce exactly one backend tool call when a tool is useful. Never claim to execute tools yourself."},
			{Role: "user", Content: req.UserMessage},
		},
		Tools:      tools,
		ToolChoice: "auto",
	}
	var out chatResponse
	if err := c.postChat(ctx, payload, &out); err != nil {
		return nil, err
	}
	if len(out.Choices) == 0 || len(out.Choices[0].Message.ToolCalls) == 0 {
		return nil, nil
	}
	call := out.Choices[0].Message.ToolCalls[0].Function
	input := map[string]any{}
	if strings.TrimSpace(call.Arguments) != "" {
		if err := json.Unmarshal([]byte(call.Arguments), &input); err != nil {
			return nil, fmt.Errorf("decode tool arguments: %w", err)
		}
	}
	return &ToolCall{Name: call.Name, Input: input}, nil
}

func (c *OpenAICompatibleClient) GenerateFinalAnswer(ctx context.Context, req FinalAnswerRequest) (string, error) {
	if c.apiKey == "" {
		return "", errors.New("llm api key is empty")
	}
	resultJSON, _ := json.Marshal(req.ToolResult)
	payload := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: "Answer the user using only the provided tool result. If the tool failed, explain the failure briefly."},
			{Role: "user", Content: req.UserMessage},
			{Role: "assistant", Content: fmt.Sprintf("Selected backend tool: %s %s", req.ToolCall.Name, mustJSONString(req.ToolCall.Input))},
			{Role: "user", Content: "Backend tool result JSON: " + string(resultJSON)},
		},
	}
	var out chatResponse
	if err := c.postChat(ctx, payload, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", errors.New("empty llm response")
	}
	return out.Choices[0].Message.Content, nil
}

func (c *OpenAICompatibleClient) postChat(ctx context.Context, payload chatRequest, out *chatResponse) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("llm request failed with status %d", resp.StatusCode)
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return err
	}
	return nil
}

type MockClient struct{}

func (MockClient) GenerateToolCall(ctx context.Context, req ToolCallRequest) (*ToolCall, error) {
	text := strings.ToLower(req.UserMessage)
	if strings.Contains(text, "http") {
		return &ToolCall{Name: "http_get", Input: map[string]any{"url": strings.TrimSpace(req.UserMessage)}}, nil
	}
	if strings.Contains(text, "log") || strings.Contains(text, "audit") {
		return &ToolCall{Name: "query_logs", Input: map[string]any{"kind": "tool_calls", "limit": 20}}, nil
	}
	if strings.Contains(text, "sql") || strings.Contains(text, "select") {
		return &ToolCall{Name: "query_mysql_readonly", Input: map[string]any{"sql": req.UserMessage}}, nil
	}
	if strings.Contains(text, "知识") || strings.Contains(text, "search") || strings.Contains(text, "kb") {
		return &ToolCall{Name: "search_knowledge_base", Input: map[string]any{"query": req.UserMessage, "top_k": 5}}, nil
	}
	return &ToolCall{Name: "calculator", Input: map[string]any{"expression": "1+1"}}, nil
}

func (MockClient) GenerateFinalAnswer(ctx context.Context, req FinalAnswerRequest) (string, error) {
	return "工具执行完成，结果为：" + mustJSONString(req.ToolResult), nil
}

type chatRequest struct {
	Model      string        `json:"model"`
	Messages   []chatMessage `json:"messages"`
	Tools      []chatTool    `json:"tools,omitempty"`
	ToolChoice any           `json:"tool_choice,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
	Arguments   string         `json:"arguments,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Function chatFunction `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

func mustJSONString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
