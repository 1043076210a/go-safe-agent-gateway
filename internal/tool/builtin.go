package tool

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"go-safe-agent-gateway/internal/rag"
	"go-safe-agent-gateway/internal/repository"
)

type CalculatorTool struct{}

func (CalculatorTool) Name() string                { return "calculator" }
func (CalculatorTool) Description() string         { return "Run a basic arithmetic operation." }
func (CalculatorTool) Permission() PermissionLevel { return PermissionReadOnly }
func (CalculatorTool) Timeout() time.Duration      { return time.Second }
func (CalculatorTool) IsAsync() bool               { return false }
func (CalculatorTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"expression": map[string]any{"type": "string", "minLength": 1, "maxLength": 200},
			"operation":  map[string]any{"type": "string", "enum": []any{"add", "subtract", "multiply", "divide"}},
			"a":          map[string]any{"type": "number"},
			"b":          map[string]any{"type": "number"},
		},
	}
}
func (CalculatorTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	if expression, ok := input["expression"].(string); ok && strings.TrimSpace(expression) != "" {
		result, err := evalExpression(expression)
		if err != nil {
			return nil, err
		}
		return &ToolResult{ToolName: "calculator", Success: true, Data: map[string]any{"result": result, "expression": expression}}, nil
	}
	a, ok := numberInput(input, "a")
	if !ok {
		return nil, errors.New("a must be a number")
	}
	b, ok := numberInput(input, "b")
	if !ok {
		return nil, errors.New("b must be a number")
	}
	var result float64
	switch input["operation"] {
	case "add":
		result = a + b
	case "subtract":
		result = a - b
	case "multiply":
		result = a * b
	case "divide":
		if b == 0 {
			return nil, errors.New("division by zero")
		}
		result = a / b
	default:
		return nil, errors.New("unsupported operation")
	}
	if math.IsInf(result, 0) || math.IsNaN(result) {
		return nil, errors.New("invalid calculation result")
	}
	return &ToolResult{ToolName: "calculator", Success: true, Data: map[string]any{"result": result}}, nil
}

type MySQLReadonlyTool struct {
	Store   repository.Store
	MaxRows int
}

func (MySQLReadonlyTool) Name() string                { return "query_mysql_readonly" }
func (MySQLReadonlyTool) Description() string         { return "Run a read-only MySQL SELECT query." }
func (MySQLReadonlyTool) Permission() PermissionLevel { return PermissionSensitive }
func (MySQLReadonlyTool) Timeout() time.Duration      { return 3 * time.Second }
func (MySQLReadonlyTool) IsAsync() bool               { return false }
func (MySQLReadonlyTool) InputSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"required":   []any{"sql"},
		"properties": map[string]any{"sql": map[string]any{"type": "string", "minLength": 1}},
	}
}
func (t MySQLReadonlyTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	sqlText, _ := input["sql"].(string)
	rows, err := t.Store.QueryReadonly(ctx, sqlText, t.MaxRows)
	if err != nil {
		return nil, err
	}
	return &ToolResult{ToolName: "query_mysql_readonly", Success: true, Data: map[string]any{"rows": rows}}, nil
}

type HTTPGetTool struct {
	Client *http.Client
}

func (HTTPGetTool) Name() string                { return "http_get" }
func (HTTPGetTool) Description() string         { return "Fetch an allowlisted HTTP or HTTPS URL with GET." }
func (HTTPGetTool) Permission() PermissionLevel { return PermissionReadOnly }
func (HTTPGetTool) Timeout() time.Duration      { return 3 * time.Second }
func (HTTPGetTool) IsAsync() bool               { return false }
func (HTTPGetTool) InputSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"required":   []any{"url"},
		"properties": map[string]any{"url": map[string]any{"type": "string", "minLength": 1}},
	}
}
func (t HTTPGetTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	rawURL, _ := input["url"].(string)
	client := t.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, err
	}
	return &ToolResult{ToolName: "http_get", Success: true, Data: map[string]any{"status_code": resp.StatusCode, "body": string(body)}}, nil
}

type KnowledgeBaseTool struct {
	Searcher rag.Searcher
}

func (KnowledgeBaseTool) Name() string                { return "search_knowledge_base" }
func (KnowledgeBaseTool) Description() string         { return "Search indexed knowledge-base chunks." }
func (KnowledgeBaseTool) Permission() PermissionLevel { return PermissionReadOnly }
func (KnowledgeBaseTool) Timeout() time.Duration      { return 2 * time.Second }
func (KnowledgeBaseTool) IsAsync() bool               { return false }
func (KnowledgeBaseTool) InputSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"required":   []any{"query"},
		"properties": map[string]any{"query": map[string]any{"type": "string", "minLength": 1}, "top_k": map[string]any{"type": "integer", "minimum": 1, "maximum": 20, "default": 5}},
	}
}
func (t KnowledgeBaseTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	query, _ := input["query"].(string)
	topK := intInput(input, "top_k", 5)
	result, err := t.Searcher.Search(ctx, query, topK)
	if err != nil {
		return nil, err
	}
	return &ToolResult{ToolName: "search_knowledge_base", Success: true, Data: result}, nil
}

type QueryLogsTool struct {
	Store repository.Store
}

func (QueryLogsTool) Name() string                { return "query_logs" }
func (QueryLogsTool) Description() string         { return "Query bounded audit and policy rejection logs." }
func (QueryLogsTool) Permission() PermissionLevel { return PermissionReadOnly }
func (QueryLogsTool) Timeout() time.Duration      { return 2 * time.Second }
func (QueryLogsTool) IsAsync() bool               { return true }
func (QueryLogsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"kind":   map[string]any{"type": "string", "enum": []any{"tool_calls", "policy_rejects"}, "default": "tool_calls"},
			"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": 100, "default": 20},
			"offset": map[string]any{"type": "integer", "minimum": 0, "default": 0},
		},
	}
}
func (t QueryLogsTool) Execute(ctx context.Context, input map[string]any) (*ToolResult, error) {
	kind, _ := input["kind"].(string)
	if kind == "" {
		kind = "tool_calls"
	}
	limit := intInput(input, "limit", 20)
	offset := intInput(input, "offset", 0)
	switch kind {
	case "policy_rejects":
		rows, err := t.Store.ListPolicyRejects(ctx, limit, offset)
		if err != nil {
			return nil, err
		}
		return &ToolResult{ToolName: "query_logs", Success: true, Data: map[string]any{"kind": kind, "rows": rows}}, nil
	case "tool_calls":
		rows, err := t.Store.ListToolCalls(ctx, limit, offset)
		if err != nil {
			return nil, err
		}
		return &ToolResult{ToolName: "query_logs", Success: true, Data: map[string]any{"kind": kind, "rows": rows}}, nil
	default:
		return nil, errors.New("unsupported log kind")
	}
}

func RegisterBuiltins(reg Registry, store repository.Store, searcher rag.Searcher, maxSQLRows int) error {
	builtins := []Tool{
		CalculatorTool{},
		MySQLReadonlyTool{Store: store, MaxRows: maxSQLRows},
		HTTPGetTool{Client: &http.Client{Timeout: 3 * time.Second}},
		KnowledgeBaseTool{Searcher: searcher},
		QueryLogsTool{Store: store},
	}
	for _, t := range builtins {
		if err := reg.Register(t); err != nil {
			return fmt.Errorf("register %s: %w", t.Name(), err)
		}
	}
	return nil
}

func numberInput(input map[string]any, key string) (float64, bool) {
	switch v := input[key].(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	default:
		return 0, false
	}
}

func intInput(input map[string]any, key string, fallback int) int {
	switch v := input[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return fallback
	}
}

type expressionParser struct {
	input string
	pos   int
}

func evalExpression(input string) (float64, error) {
	p := &expressionParser{input: input}
	result, err := p.parseExpression()
	if err != nil {
		return 0, err
	}
	p.skipSpace()
	if p.pos != len(p.input) {
		return 0, errors.New("invalid expression")
	}
	if math.IsInf(result, 0) || math.IsNaN(result) {
		return 0, errors.New("invalid calculation result")
	}
	return result, nil
}

func (p *expressionParser) parseExpression() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpace()
		switch p.peek() {
		case '+':
			p.pos++
			right, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			left += right
		case '-':
			p.pos++
			right, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			left -= right
		default:
			return left, nil
		}
	}
}

func (p *expressionParser) parseTerm() (float64, error) {
	left, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpace()
		switch p.peek() {
		case '*':
			p.pos++
			right, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			left *= right
		case '/':
			p.pos++
			right, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			if right == 0 {
				return 0, errors.New("division by zero")
			}
			left /= right
		default:
			return left, nil
		}
	}
}

func (p *expressionParser) parseFactor() (float64, error) {
	p.skipSpace()
	if p.peek() == '-' {
		p.pos++
		value, err := p.parseFactor()
		return -value, err
	}
	if p.peek() == '(' {
		p.pos++
		value, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		p.skipSpace()
		if p.peek() != ')' {
			return 0, errors.New("missing closing parenthesis")
		}
		p.pos++
		return value, nil
	}
	return p.parseNumber()
}

func (p *expressionParser) parseNumber() (float64, error) {
	p.skipSpace()
	start := p.pos
	dotSeen := false
	for p.pos < len(p.input) {
		r := rune(p.input[p.pos])
		if r == '.' && !dotSeen {
			dotSeen = true
			p.pos++
			continue
		}
		if !unicode.IsDigit(r) {
			break
		}
		p.pos++
	}
	if start == p.pos {
		return 0, errors.New("number expected")
	}
	return strconv.ParseFloat(p.input[start:p.pos], 64)
}

func (p *expressionParser) skipSpace() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
}

func (p *expressionParser) peek() byte {
	if p.pos >= len(p.input) {
		return 0
	}
	return p.input[p.pos]
}
