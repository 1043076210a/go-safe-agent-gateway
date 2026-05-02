package rag

import "context"

type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
}

type Searcher interface {
	Search(ctx context.Context, query string, topK int) (*SearchResponse, error)
}

type IndexRequest struct {
	Title      string `json:"title"`
	SourceType string `json:"source_type"`
	SourcePath string `json:"source_path"`
	Content    string `json:"content"`
}

type IndexResponse struct {
	DocumentID string `json:"document_id"`
	Chunks     int    `json:"chunks"`
}

type SearchResponse struct {
	Chunks    []SearchChunk `json:"chunks"`
	NoContext bool          `json:"no_context,omitempty"`
	Message   string        `json:"message,omitempty"`
}

type SearchChunk struct {
	Content       string  `json:"content"`
	Score         float64 `json:"score"`
	DocumentID    string  `json:"document_id"`
	DocumentTitle string  `json:"document_title"`
	ChunkIndex    int     `json:"chunk_index"`
	SourcePath    string  `json:"source_path"`
	TokenCount    int     `json:"token_count"`
}
