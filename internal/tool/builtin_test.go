package tool

import (
	"context"
	"testing"

	"go-safe-agent-gateway/internal/rag"
)

type fakeSearcher struct {
	result *rag.SearchResponse
}

func (f fakeSearcher) Search(context.Context, string, int) (*rag.SearchResponse, error) {
	return f.result, nil
}

func TestCalculatorTool_WhenAdd_ShouldReturnResult(t *testing.T) {
	result, err := CalculatorTool{}.Execute(context.Background(), map[string]any{"operation": "add", "a": 2.0, "b": 3.0})
	if err != nil {
		t.Fatalf("execute calculator: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["result"] != 5.0 {
		t.Fatalf("expected 5, got %+v", data)
	}
}

func TestCalculatorTool_WhenExpression_ShouldRespectPrecedence(t *testing.T) {
	result, err := CalculatorTool{}.Execute(context.Background(), map[string]any{"expression": "2 + 3 * (4 - 1)"})
	if err != nil {
		t.Fatalf("execute calculator expression: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["result"] != 11.0 {
		t.Fatalf("expected 11, got %+v", data)
	}
}

func TestQueryLogsTool_WhenAsyncMetadata_ShouldBeAsync(t *testing.T) {
	if !(QueryLogsTool{}).IsAsync() {
		t.Fatal("query_logs should support async execution")
	}
}

func TestKnowledgeBaseTool_WhenNoChunksMatched_ShouldReturnEmptyContext(t *testing.T) {
	result, err := KnowledgeBaseTool{Searcher: fakeSearcher{result: &rag.SearchResponse{Chunks: []rag.SearchChunk{}, NoContext: true, Message: "no relevant context found"}}}.Execute(context.Background(), map[string]any{"query": "missing", "top_k": 5})
	if err != nil {
		t.Fatalf("search knowledge base: %v", err)
	}
	data := result.Data.(*rag.SearchResponse)
	if !data.NoContext {
		t.Fatalf("expected no_context, got %+v", data)
	}
}

func TestKnowledgeBaseTool_WhenChunksMatched_ShouldReturnCitationMetadata(t *testing.T) {
	result, err := KnowledgeBaseTool{Searcher: fakeSearcher{result: &rag.SearchResponse{Chunks: []rag.SearchChunk{{
		Content:       "gateway policy engine",
		Score:         0.91,
		DocumentTitle: "Gateway",
		ChunkIndex:    1,
		SourcePath:    "docs/gateway.md",
	}}}}}.Execute(context.Background(), map[string]any{"query": "policy", "top_k": 5})
	if err != nil {
		t.Fatalf("search knowledge base: %v", err)
	}
	data := result.Data.(*rag.SearchResponse)
	if len(data.Chunks) != 1 || data.Chunks[0].DocumentTitle != "Gateway" {
		t.Fatalf("unexpected chunks: %+v", data.Chunks)
	}
}
