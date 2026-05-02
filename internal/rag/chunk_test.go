package rag

import (
	"strings"
	"testing"
)

func TestRAG_WhenMarkdownHasHeadings_ShouldSplitBySections(t *testing.T) {
	chunks := SplitMarkdown("# A\nfirst\n\n# B\nsecond")
	if len(chunks) != 2 {
		t.Fatalf("expected two chunks, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0].Content, "first") || !strings.Contains(chunks[1].Content, "second") {
		t.Fatalf("unexpected chunks: %+v", chunks)
	}
}

func TestRAG_WhenTextIsLong_ShouldUseOverlap(t *testing.T) {
	text := strings.Repeat("a", defaultChunkSize+200)
	chunks := splitLongText(text, defaultChunkSize, defaultChunkOverlap)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
}
