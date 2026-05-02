package rag

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go-safe-agent-gateway/internal/repository"
)

func TestRAG_WhenIndexAndSearch_ShouldUseQdrantVectors(t *testing.T) {
	var upsertCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/collections/test_collection":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/collections/test_collection/points":
			upsertCalled = true
			var req struct {
				Points []qdrantPoint `json:"points"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode upsert: %v", err)
			}
			if len(req.Points) == 0 || len(req.Points[0].Vector) != 8 {
				t.Fatalf("unexpected points: %+v", req.Points)
			}
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/collections/test_collection/points/search":
			var req struct {
				Vector []float32 `json:"vector"`
				Limit  int       `json:"limit"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode search: %v", err)
			}
			if len(req.Vector) != 8 || req.Limit != 5 {
				t.Fatalf("unexpected search request: %+v", req)
			}
			_, _ = w.Write([]byte(`{"result":[{"id":"point-1","score":0.91,"payload":{"content":"policy engine content","document_id":"doc-1","document_title":"Gateway","chunk_index":0,"source_path":"docs/gateway.md","token_count":3}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store := repository.NewMemoryStore()
	svc := NewService(store, NewQdrantClient(server.URL, "test_collection", 8, 0), NewDeterministicEmbedder(8), 0.1)
	indexResp, err := svc.IndexDocument(context.Background(), IndexRequest{Title: "Gateway", SourcePath: "docs/gateway.md", Content: "# Gateway\npolicy engine content"})
	if err != nil {
		t.Fatalf("index document: %v", err)
	}
	if indexResp.Chunks == 0 || !upsertCalled {
		t.Fatalf("expected qdrant upsert, resp=%+v upsert=%v", indexResp, upsertCalled)
	}
	searchResp, err := svc.Search(context.Background(), "policy", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(searchResp.Chunks) != 1 || searchResp.Chunks[0].DocumentTitle != "Gateway" {
		t.Fatalf("unexpected search response: %+v", searchResp)
	}
}

func TestRAG_WhenNoChunksMatched_ShouldReturnEmptyContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/points/search") {
			_, _ = w.Write([]byte(`{"result":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	svc := NewService(repository.NewMemoryStore(), NewQdrantClient(server.URL, "test_collection", 8, 0), NewDeterministicEmbedder(8), 0.1)
	resp, err := svc.Search(context.Background(), "missing", 5)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !resp.NoContext || resp.Message == "" || len(resp.Chunks) != 0 {
		t.Fatalf("expected empty context response, got %+v", resp)
	}
}
