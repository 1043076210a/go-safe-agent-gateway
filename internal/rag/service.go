package rag

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"go.opentelemetry.io/otel"

	"go-safe-agent-gateway/internal/model"
	"go-safe-agent-gateway/internal/repository"
)

type Service struct {
	store          repository.Store
	qdrant         *QdrantClient
	embedder       Embedder
	scoreThreshold float64
}

func NewService(store repository.Store, qdrant *QdrantClient, embedder Embedder, scoreThreshold float64) *Service {
	return &Service{store: store, qdrant: qdrant, embedder: embedder, scoreThreshold: scoreThreshold}
}

func (s *Service) Ensure(ctx context.Context) error {
	if s.qdrant == nil {
		return errors.New("qdrant client is nil")
	}
	ensureCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.qdrant.EnsureCollection(ensureCtx)
}

func (s *Service) IndexDocument(ctx context.Context, req IndexRequest) (*IndexResponse, error) {
	ctx, span := otel.Tracer("go-safe-agent-gateway").Start(ctx, "rag.embedding")
	defer span.End()

	if req.Title == "" {
		req.Title = "untitled"
	}
	if req.SourceType == "" {
		req.SourceType = "manual"
	}
	if req.Content == "" {
		return nil, errors.New("content is required")
	}
	if err := s.Ensure(ctx); err != nil {
		return nil, err
	}
	doc := &model.Document{Title: req.Title, SourceType: req.SourceType, SourcePath: req.SourcePath, Content: req.Content}
	if err := s.store.CreateDocument(ctx, doc); err != nil {
		return nil, err
	}
	rawChunks := SplitMarkdown(req.Content)
	texts := make([]string, 0, len(rawChunks))
	for _, chunk := range rawChunks {
		texts = append(texts, chunk.Content)
	}
	vectors, err := s.embedder.Embed(ctx, texts)
	if err != nil {
		return nil, err
	}
	points := make([]qdrantPoint, 0, len(rawChunks))
	chunks := make([]model.DocumentChunk, 0, len(rawChunks))
	for i, chunk := range rawChunks {
		pointID := repository.NewID()
		metadata := map[string]any{
			"document_id":    doc.ID,
			"document_title": doc.Title,
			"chunk_index":    i,
			"source_path":    doc.SourcePath,
			"token_count":    chunk.TokenCount,
		}
		metaJSON, _ := json.Marshal(metadata)
		points = append(points, qdrantPoint{ID: pointID, Vector: vectors[i], Payload: map[string]any{
			"content":        chunk.Content,
			"document_id":    doc.ID,
			"document_title": doc.Title,
			"chunk_index":    i,
			"source_path":    doc.SourcePath,
			"token_count":    chunk.TokenCount,
		}})
		chunks = append(chunks, model.DocumentChunk{
			ID:            repository.NewID(),
			DocumentID:    doc.ID,
			ChunkIndex:    i,
			Content:       chunk.Content,
			TokenCount:    chunk.TokenCount,
			QdrantPointID: pointID,
			Metadata:      string(metaJSON),
		})
	}
	if err := s.qdrant.Upsert(ctx, points); err != nil {
		return nil, err
	}
	if err := s.store.SaveChunks(ctx, chunks); err != nil {
		return nil, err
	}
	return &IndexResponse{DocumentID: doc.ID, Chunks: len(chunks)}, nil
}

func (s *Service) Search(ctx context.Context, query string, topK int) (*SearchResponse, error) {
	ctx, span := otel.Tracer("go-safe-agent-gateway").Start(ctx, "rag.search")
	defer span.End()

	if query == "" {
		return nil, errors.New("query is required")
	}
	if topK <= 0 {
		topK = 5
	}
	if err := s.Ensure(ctx); err != nil {
		return nil, err
	}
	vectors, err := s.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	hits, err := s.qdrant.Search(ctx, vectors[0], topK, s.scoreThreshold)
	if err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		return &SearchResponse{Chunks: []SearchChunk{}, NoContext: true, Message: "no relevant context found"}, nil
	}
	chunks := make([]SearchChunk, 0, len(hits))
	for _, hit := range hits {
		chunks = append(chunks, SearchChunk{
			Content:       stringPayload(hit.Payload, "content"),
			Score:         hit.Score,
			DocumentID:    stringPayload(hit.Payload, "document_id"),
			DocumentTitle: stringPayload(hit.Payload, "document_title"),
			ChunkIndex:    intPayload(hit.Payload, "chunk_index"),
			SourcePath:    stringPayload(hit.Payload, "source_path"),
			TokenCount:    intPayload(hit.Payload, "token_count"),
		})
	}
	return &SearchResponse{Chunks: chunks}, nil
}

func stringPayload(payload map[string]any, key string) string {
	v, _ := payload[key].(string)
	return v
}

func intPayload(payload map[string]any, key string) int {
	switch v := payload[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	default:
		return 0
	}
}
