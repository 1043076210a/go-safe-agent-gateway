package repository

import (
	"context"
	"strings"
	"sync"
	"time"

	"go-safe-agent-gateway/internal/model"
)

type MemoryStore struct {
	mu       sync.RWMutex
	sessions map[string]model.Session
	messages []model.Message
	calls    []model.ToolCallAudit
	rejects  []model.PolicyReject
	docs     map[string]model.Document
	chunks   []model.DocumentChunk
	tasks    map[string]model.AsyncTask
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{sessions: map[string]model.Session{}, docs: map[string]model.Document{}, tasks: map[string]model.AsyncTask{}}
}

func (m *MemoryStore) Health(context.Context) error { return nil }

func (m *MemoryStore) CreateSession(ctx context.Context, s *model.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	if s.ID == "" {
		s.ID = NewID()
	}
	s.CreatedAt, s.UpdatedAt = now, now
	if s.Status == "" {
		s.Status = "active"
	}
	m.sessions[s.ID] = *s
	return nil
}

func (m *MemoryStore) GetSession(ctx context.Context, id string) (*model.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &s, nil
}

func (m *MemoryStore) CreateMessage(ctx context.Context, msg *model.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if msg.ID == "" {
		msg.ID = NewID()
	}
	msg.CreatedAt = time.Now()
	m.messages = append(m.messages, *msg)
	return nil
}

func (m *MemoryStore) ListMessages(ctx context.Context, sessionID string, limit, offset int) ([]model.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []model.Message
	for _, msg := range m.messages {
		if msg.SessionID == sessionID {
			out = append(out, msg)
		}
	}
	return page(out, limit, offset), nil
}

func (m *MemoryStore) SaveToolAudit(ctx context.Context, a model.ToolCallAudit) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, a)
	return nil
}

func (m *MemoryStore) ListToolCalls(ctx context.Context, limit, offset int) ([]model.ToolCallAudit, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return page(m.calls, limit, offset), nil
}

func (m *MemoryStore) SavePolicyReject(ctx context.Context, r model.PolicyReject) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r.CreatedAt = time.Now()
	m.rejects = append(m.rejects, r)
	return nil
}

func (m *MemoryStore) ListPolicyRejects(ctx context.Context, limit, offset int) ([]model.PolicyReject, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return page(m.rejects, limit, offset), nil
}

func (m *MemoryStore) CreateDocument(ctx context.Context, d *model.Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	if d.ID == "" {
		d.ID = NewID()
	}
	d.Status = "created"
	d.CreatedAt, d.UpdatedAt = now, now
	m.docs[d.ID] = *d
	return nil
}

func (m *MemoryStore) GetDocument(ctx context.Context, id string) (*model.Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	d, ok := m.docs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return &d, nil
}

func (m *MemoryStore) SaveChunks(ctx context.Context, chunks []model.DocumentChunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.chunks = append(m.chunks, chunks...)
	return nil
}

func (m *MemoryStore) CreateAsyncTask(ctx context.Context, task *model.AsyncTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	task.CreatedAt, task.UpdatedAt = now, now
	m.tasks[task.TaskID] = *task
	return nil
}

func (m *MemoryStore) UpdateAsyncTask(ctx context.Context, task *model.AsyncTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	task.UpdatedAt = time.Now()
	m.tasks[task.TaskID] = *task
	return nil
}

func (m *MemoryStore) GetAsyncTask(ctx context.Context, taskID string) (*model.AsyncTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tasks[taskID]
	if !ok {
		return nil, ErrNotFound
	}
	return &t, nil
}

func (m *MemoryStore) QueryReadonly(ctx context.Context, sqlText string, maxRows int) ([]map[string]any, error) {
	return []map[string]any{{"mock": true, "sql_preview": strings.TrimSpace(sqlText)}}, nil
}

func page[T any](in []T, limit, offset int) []T {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(in) {
		return []T{}
	}
	end := offset + limit
	if end > len(in) {
		end = len(in)
	}
	out := make([]T, end-offset)
	copy(out, in[offset:end])
	return out
}
