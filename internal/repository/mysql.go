package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"go-safe-agent-gateway/internal/model"
)

type MySQLStore struct {
	db *sql.DB
}

func (s *MySQLStore) Health(ctx context.Context) error { return s.db.PingContext(ctx) }

func (s *MySQLStore) CreateSession(ctx context.Context, m *model.Session) error {
	now := time.Now()
	if m.ID == "" {
		m.ID = NewID()
	}
	if m.Status == "" {
		m.Status = "active"
	}
	m.CreatedAt, m.UpdatedAt = now, now
	_, err := s.db.ExecContext(ctx, `INSERT INTO agent_sessions(id,user_id,title,status,created_at,updated_at) VALUES(?,?,?,?,?,?)`, m.ID, m.UserID, m.Title, m.Status, m.CreatedAt, m.UpdatedAt)
	return err
}

func (s *MySQLStore) GetSession(ctx context.Context, id string) (*model.Session, error) {
	var m model.Session
	err := s.db.QueryRowContext(ctx, `SELECT id,user_id,title,status,created_at,updated_at FROM agent_sessions WHERE id=?`, id).Scan(&m.ID, &m.UserID, &m.Title, &m.Status, &m.CreatedAt, &m.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return &m, err
}

func (s *MySQLStore) CreateMessage(ctx context.Context, m *model.Message) error {
	if m.ID == "" {
		m.ID = NewID()
	}
	m.CreatedAt = time.Now()
	_, err := s.db.ExecContext(ctx, `INSERT INTO agent_messages(id,session_id,role,content,created_at) VALUES(?,?,?,?,?)`, m.ID, m.SessionID, m.Role, m.Content, m.CreatedAt)
	return err
}

func (s *MySQLStore) ListMessages(ctx context.Context, sessionID string, limit, offset int) ([]model.Message, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,session_id,role,content,created_at FROM agent_messages WHERE session_id=? ORDER BY created_at ASC LIMIT ? OFFSET ?`, sessionID, normLimit(limit), offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Message
	for rows.Next() {
		var m model.Message
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *MySQLStore) SaveToolAudit(ctx context.Context, a model.ToolCallAudit) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO agent_tool_calls(call_id,session_id,message_id,user_id,tool_name,input_json,sanitized_input_json,status,duration_ms,error_message,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, a.CallID, a.SessionID, a.MessageID, a.UserID, a.ToolName, a.InputJSON, a.SanitizedInputJSON, a.Status, a.DurationMs, a.ErrorMessage, time.Now())
	return err
}

func (s *MySQLStore) ListToolCalls(ctx context.Context, limit, offset int) ([]model.ToolCallAudit, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT call_id,session_id,message_id,user_id,tool_name,input_json,sanitized_input_json,status,duration_ms,error_message FROM agent_tool_calls ORDER BY created_at DESC LIMIT ? OFFSET ?`, normLimit(limit), offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ToolCallAudit
	for rows.Next() {
		var a model.ToolCallAudit
		if err := rows.Scan(&a.CallID, &a.SessionID, &a.MessageID, &a.UserID, &a.ToolName, &a.InputJSON, &a.SanitizedInputJSON, &a.Status, &a.DurationMs, &a.ErrorMessage); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *MySQLStore) SavePolicyReject(ctx context.Context, r model.PolicyReject) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO policy_reject_logs(user_id,session_id,tool_name,reason,input_json,created_at) VALUES(?,?,?,?,?,?)`, r.UserID, r.SessionID, r.ToolName, r.Reason, r.InputJSON, time.Now())
	return err
}

func (s *MySQLStore) ListPolicyRejects(ctx context.Context, limit, offset int) ([]model.PolicyReject, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id,session_id,tool_name,reason,input_json,created_at FROM policy_reject_logs ORDER BY created_at DESC LIMIT ? OFFSET ?`, normLimit(limit), offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.PolicyReject
	for rows.Next() {
		var r model.PolicyReject
		if err := rows.Scan(&r.UserID, &r.SessionID, &r.ToolName, &r.Reason, &r.InputJSON, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *MySQLStore) CreateDocument(ctx context.Context, d *model.Document) error {
	now := time.Now()
	if d.ID == "" {
		d.ID = NewID()
	}
	d.Status = "created"
	d.CreatedAt, d.UpdatedAt = now, now
	_, err := s.db.ExecContext(ctx, `INSERT INTO documents(id,title,source_type,source_path,status,content,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, d.ID, d.Title, d.SourceType, d.SourcePath, d.Status, d.Content, d.CreatedAt, d.UpdatedAt)
	return err
}

func (s *MySQLStore) GetDocument(ctx context.Context, id string) (*model.Document, error) {
	var d model.Document
	err := s.db.QueryRowContext(ctx, `SELECT id,title,source_type,source_path,status,content,created_at,updated_at FROM documents WHERE id=?`, id).Scan(&d.ID, &d.Title, &d.SourceType, &d.SourcePath, &d.Status, &d.Content, &d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return &d, err
}

func (s *MySQLStore) SaveChunks(ctx context.Context, chunks []model.DocumentChunk) error {
	for _, c := range chunks {
		if c.ID == "" {
			c.ID = NewID()
		}
		_, err := s.db.ExecContext(ctx, `INSERT INTO document_chunks(id,document_id,chunk_index,content,token_count,qdrant_point_id,metadata,created_at) VALUES(?,?,?,?,?,?,?,?)`, c.ID, c.DocumentID, c.ChunkIndex, c.Content, c.TokenCount, c.QdrantPointID, c.Metadata, time.Now())
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *MySQLStore) CreateAsyncTask(ctx context.Context, task *model.AsyncTask) error {
	now := time.Now()
	task.CreatedAt, task.UpdatedAt = now, now
	_, err := s.db.ExecContext(ctx, `INSERT INTO async_tasks(task_id,user_id,tool_name,status,input_json,result_json,error_message,retry_count,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, task.TaskID, task.UserID, task.ToolName, task.Status, task.InputJSON, nullIfEmpty(task.ResultJSON), task.ErrorMessage, task.RetryCount, task.CreatedAt, task.UpdatedAt)
	return err
}

func (s *MySQLStore) UpdateAsyncTask(ctx context.Context, task *model.AsyncTask) error {
	_, err := s.db.ExecContext(ctx, `UPDATE async_tasks SET status=?,result_json=?,error_message=?,retry_count=?,updated_at=?,finished_at=? WHERE task_id=?`, task.Status, nullIfEmpty(task.ResultJSON), task.ErrorMessage, task.RetryCount, time.Now(), task.FinishedAt, task.TaskID)
	return err
}

func (s *MySQLStore) GetAsyncTask(ctx context.Context, taskID string) (*model.AsyncTask, error) {
	var t model.AsyncTask
	var resultJSON sql.NullString
	var errorMessage sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT task_id,user_id,tool_name,status,input_json,result_json,error_message,retry_count,created_at,updated_at,finished_at FROM async_tasks WHERE task_id=?`, taskID).Scan(&t.TaskID, &t.UserID, &t.ToolName, &t.Status, &t.InputJSON, &resultJSON, &errorMessage, &t.RetryCount, &t.CreatedAt, &t.UpdatedAt, &t.FinishedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if resultJSON.Valid {
		t.ResultJSON = resultJSON.String
	}
	if errorMessage.Valid {
		t.ErrorMessage = errorMessage.String
	}
	return &t, err
}

func (s *MySQLStore) QueryReadonly(ctx context.Context, sqlText string, maxRows int) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, enforceLimit(sqlText, maxRows))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := map[string]any{}
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok {
				row[c] = string(b)
			} else {
				row[c] = vals[i]
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func normLimit(limit int) int {
	if limit <= 0 || limit > 100 {
		return 20
	}
	return limit
}

func enforceLimit(q string, maxRows int) string {
	if maxRows <= 0 {
		maxRows = 100
	}
	lower := strings.ToLower(q)
	if strings.Contains(lower, " limit ") {
		return q
	}
	return fmt.Sprintf("%s LIMIT %d", strings.TrimRight(strings.TrimSpace(q), ";"), maxRows)
}

func nullIfEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
