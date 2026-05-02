package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"

	"go-safe-agent-gateway/internal/model"
)

type Store interface {
	Health(ctx context.Context) error
	CreateSession(ctx context.Context, s *model.Session) error
	GetSession(ctx context.Context, id string) (*model.Session, error)
	CreateMessage(ctx context.Context, m *model.Message) error
	ListMessages(ctx context.Context, sessionID string, limit, offset int) ([]model.Message, error)
	SaveToolAudit(ctx context.Context, a model.ToolCallAudit) error
	ListToolCalls(ctx context.Context, limit, offset int) ([]model.ToolCallAudit, error)
	SavePolicyReject(ctx context.Context, r model.PolicyReject) error
	ListPolicyRejects(ctx context.Context, limit, offset int) ([]model.PolicyReject, error)
	CreateDocument(ctx context.Context, d *model.Document) error
	GetDocument(ctx context.Context, id string) (*model.Document, error)
	SaveChunks(ctx context.Context, chunks []model.DocumentChunk) error
	CreateAsyncTask(ctx context.Context, task *model.AsyncTask) error
	UpdateAsyncTask(ctx context.Context, task *model.AsyncTask) error
	GetAsyncTask(ctx context.Context, taskID string) (*model.AsyncTask, error)
	QueryReadonly(ctx context.Context, sqlText string, maxRows int) ([]map[string]any, error)
}

var ErrNotFound = errors.New("not found")

func NewStore(ctx context.Context, dsn string) Store {
	if dsn == "" {
		return NewMemoryStore()
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return NewMemoryStore()
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return NewMemoryStore()
	}
	return &MySQLStore{db: db}
}

func NewID() string { return uuid.NewString() }

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
