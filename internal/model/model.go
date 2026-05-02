package model

import "time"

type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type ToolCallAudit struct {
	CallID             string
	SessionID          string
	MessageID          string
	UserID             string
	ToolName           string
	InputJSON          string
	SanitizedInputJSON string
	Status             string
	DurationMs         int64
	ErrorMessage       string
}

type PolicyReject struct {
	UserID    string    `json:"user_id"`
	SessionID string    `json:"session_id"`
	ToolName  string    `json:"tool_name"`
	Reason    string    `json:"reason"`
	InputJSON string    `json:"input_json"`
	CreatedAt time.Time `json:"created_at"`
}

type Document struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	SourceType string    `json:"source_type"`
	SourcePath string    `json:"source_path"`
	Status     string    `json:"status"`
	Content    string    `json:"content,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type DocumentChunk struct {
	ID            string    `json:"id"`
	DocumentID    string    `json:"document_id"`
	ChunkIndex    int       `json:"chunk_index"`
	Content       string    `json:"content"`
	TokenCount    int       `json:"token_count"`
	QdrantPointID string    `json:"qdrant_point_id"`
	Metadata      string    `json:"metadata"`
	CreatedAt     time.Time `json:"created_at"`
}

type AsyncTask struct {
	TaskID       string     `json:"task_id"`
	UserID       string     `json:"user_id"`
	ToolName     string     `json:"tool_name"`
	Status       string     `json:"status"`
	InputJSON    string     `json:"input_json"`
	ResultJSON   string     `json:"result_json,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	RetryCount   int        `json:"retry_count"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
}
