package tool

import (
	"context"
	"testing"
	"time"
)

type testTool struct {
	name string
}

func (t testTool) Name() string                { return t.name }
func (t testTool) Description() string         { return "test tool" }
func (t testTool) InputSchema() map[string]any { return map[string]any{"type": "object"} }
func (t testTool) Permission() PermissionLevel { return PermissionReadOnly }
func (t testTool) Timeout() time.Duration      { return time.Second }
func (t testTool) IsAsync() bool               { return false }
func (t testTool) Execute(context.Context, map[string]any) (*ToolResult, error) {
	return &ToolResult{ToolName: t.name, Success: true}, nil
}

func TestRegistry_WhenDuplicateTool_ShouldReturnError(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(testTool{name: "calculator"}); err != nil {
		t.Fatalf("register first tool: %v", err)
	}
	if err := reg.Register(testTool{name: "calculator"}); err == nil {
		t.Fatal("expected duplicate registration error")
	}
}

func TestRegistry_WhenToolMissing_ShouldReturnFalse(t *testing.T) {
	reg := NewRegistry()
	if _, ok := reg.Get("missing"); ok {
		t.Fatal("expected missing tool lookup to return false")
	}
}

func TestRegistry_WhenListed_ShouldExposeMetadataOnly(t *testing.T) {
	reg := NewRegistry()
	if err := reg.Register(testTool{name: "calculator"}); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	list := reg.List()
	if len(list) != 1 {
		t.Fatalf("expected one tool, got %d", len(list))
	}
	if list[0].Name != "calculator" || list[0].TimeoutMs == 0 {
		t.Fatalf("unexpected metadata: %+v", list[0])
	}
}
