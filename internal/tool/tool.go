package tool

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

type PermissionLevel string

const (
	PermissionReadOnly  PermissionLevel = "readonly"
	PermissionSensitive PermissionLevel = "sensitive"
	PermissionAdmin     PermissionLevel = "admin"
)

type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]any
	Permission() PermissionLevel
	Timeout() time.Duration
	IsAsync() bool
	Execute(ctx context.Context, input map[string]any) (*ToolResult, error)
}

type ToolResult struct {
	ToolName string         `json:"tool_name"`
	Success  bool           `json:"success"`
	Data     any            `json:"data,omitempty"`
	Error    string         `json:"error,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type ToolMeta struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema map[string]any  `json:"input_schema"`
	Permission  PermissionLevel `json:"permission"`
	TimeoutMs   int64           `json:"timeout_ms"`
	Async       bool            `json:"async"`
}

type Registry interface {
	Register(tool Tool) error
	Get(name string) (Tool, bool)
	List() []ToolMeta
}

var (
	ErrDuplicateTool = errors.New("duplicate tool")
	ErrInvalidTool   = errors.New("invalid tool")
)

type StaticRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *StaticRegistry {
	return &StaticRegistry{tools: map[string]Tool{}}
}

func (r *StaticRegistry) Register(t Tool) error {
	if t == nil || t.Name() == "" {
		return ErrInvalidTool
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[t.Name()]; exists {
		return ErrDuplicateTool
	}
	r.tools[t.Name()] = t
	return nil
}

func (r *StaticRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *StaticRegistry) List() []ToolMeta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]ToolMeta, 0, len(names))
	for _, name := range names {
		t := r.tools[name]
		out = append(out, ToolMeta{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: cloneMap(t.InputSchema()),
			Permission:  t.Permission(),
			TimeoutMs:   t.Timeout().Milliseconds(),
			Async:       t.IsAsync(),
		})
	}
	return out
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if nested, ok := v.(map[string]any); ok {
			out[k] = cloneMap(nested)
			continue
		}
		out[k] = v
	}
	return out
}
