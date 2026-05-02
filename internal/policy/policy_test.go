package policy

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-safe-agent-gateway/internal/tool"
)

type fakeLimiter struct {
	count int64
	err   error
}

func (f *fakeLimiter) IncrWithTTL(context.Context, string, time.Duration) (int64, error) {
	if f.err != nil {
		return 0, f.err
	}
	f.count++
	return f.count, nil
}
func (f *fakeLimiter) Set(context.Context, string, any, time.Duration) error { return nil }
func (f *fakeLimiter) Get(context.Context, string) (string, error)           { return "", nil }

func TestPolicy_WhenPermissionSensitiveForUser_ShouldReject(t *testing.T) {
	engine := NewEngine(Config{AllowedTools: []string{"query_mysql_readonly"}})
	decision, err := engine.Check(context.Background(), &PolicyRequest{
		UserID:     "user-1",
		ToolName:   "query_mysql_readonly",
		Permission: tool.PermissionSensitive,
		Input:      map[string]any{"sql": "SELECT * FROM orders LIMIT 10"},
	})
	if err != nil {
		t.Fatalf("policy check: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected permission rejection")
	}
}

func TestSQLPolicy_WhenDeleteStatement_ShouldReject(t *testing.T) {
	engine := NewEngine(Config{AllowedTools: []string{"query_mysql_readonly"}, RequireSQLLimit: true})
	decision, err := engine.Check(context.Background(), &PolicyRequest{
		UserID:     "admin",
		ToolName:   "query_mysql_readonly",
		Permission: tool.PermissionSensitive,
		Input:      map[string]any{"sql": "DELETE FROM users LIMIT 1"},
	})
	if err != nil {
		t.Fatalf("policy check: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected dangerous SQL rejection")
	}
}

func TestSQLPolicy_WhenLimitMissing_ShouldReject(t *testing.T) {
	engine := NewEngine(Config{AllowedTools: []string{"query_mysql_readonly"}, RequireSQLLimit: true})
	decision, err := engine.Check(context.Background(), &PolicyRequest{
		UserID:     "admin",
		ToolName:   "query_mysql_readonly",
		Permission: tool.PermissionSensitive,
		Input:      map[string]any{"sql": "SELECT * FROM orders"},
	})
	if err != nil {
		t.Fatalf("policy check: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected missing LIMIT rejection")
	}
}

func TestPolicy_WhenRateLimitExceeded_ShouldReject(t *testing.T) {
	limiter := &fakeLimiter{count: 1}
	engine := NewEngine(Config{AllowedTools: []string{"calculator"}, RateLimiter: limiter, RateLimitPerMinute: 1})
	decision, err := engine.Check(context.Background(), &PolicyRequest{
		UserID:     "user-1",
		ToolName:   "calculator",
		Permission: tool.PermissionReadOnly,
		Input:      map[string]any{},
	})
	if err != nil {
		t.Fatalf("policy check: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected rate-limit rejection")
	}
}

func TestPolicy_WhenRateLimiterFailsClosed_ShouldReject(t *testing.T) {
	limiter := &fakeLimiter{err: errors.New("redis unavailable")}
	engine := NewEngine(Config{AllowedTools: []string{"calculator"}, RateLimiter: limiter, RateLimitFailClosed: true})
	decision, err := engine.Check(context.Background(), &PolicyRequest{
		UserID:     "user-1",
		ToolName:   "calculator",
		Permission: tool.PermissionReadOnly,
		Input:      map[string]any{},
	})
	if err != nil {
		t.Fatalf("policy check: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected fail-closed rate-limit rejection")
	}
}

func TestURLPolicy_WhenLocalhost_ShouldReject(t *testing.T) {
	engine := NewEngine(Config{AllowedTools: []string{"http_get"}, URLAllowlist: []string{"example.com"}})
	decision, err := engine.Check(context.Background(), &PolicyRequest{
		UserID:     "user-1",
		ToolName:   "http_get",
		Permission: tool.PermissionReadOnly,
		Input:      map[string]any{"url": "http://127.0.0.1/admin"},
	})
	if err != nil {
		t.Fatalf("policy check: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected localhost rejection")
	}
}

func TestPolicy_WhenInputHasSensitiveField_ShouldMask(t *testing.T) {
	engine := NewEngine(Config{AllowedTools: []string{"calculator"}})
	decision, err := engine.Check(context.Background(), &PolicyRequest{
		UserID:     "user-1",
		ToolName:   "calculator",
		Permission: tool.PermissionReadOnly,
		Input:      map[string]any{"api_key": "secret-value", "x": 1},
	})
	if err != nil {
		t.Fatalf("policy check: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("expected allowed decision: %+v", decision)
	}
	if decision.SanitizedInput["api_key"] != "***" {
		t.Fatalf("expected api_key mask, got %+v", decision.SanitizedInput)
	}
}
