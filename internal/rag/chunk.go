package rag

import (
	"strings"
	"unicode/utf8"
)

const (
	defaultChunkSize    = 900
	defaultChunkOverlap = 120
)

type rawChunk struct {
	Content    string
	TokenCount int
}

func SplitMarkdown(content string) []rawChunk {
	sections := splitByHeading(content)
	var chunks []rawChunk
	for _, section := range sections {
		for _, paragraph := range splitParagraphs(section) {
			chunks = append(chunks, splitLongText(paragraph, defaultChunkSize, defaultChunkOverlap)...)
		}
	}
	return chunks
}

func splitByHeading(content string) []string {
	lines := strings.Split(content, "\n")
	var sections []string
	var current []string
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") && len(current) > 0 {
			sections = append(sections, strings.TrimSpace(strings.Join(current, "\n")))
			current = current[:0]
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		sections = append(sections, strings.TrimSpace(strings.Join(current, "\n")))
	}
	return nonEmpty(sections)
}

func splitParagraphs(content string) []string {
	parts := strings.Split(content, "\n\n")
	return nonEmpty(parts)
}

func splitLongText(text string, size, overlap int) []rawChunk {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if utf8.RuneCountInString(text) <= size {
		return []rawChunk{{Content: text, TokenCount: tokenCount(text)}}
	}
	runes := []rune(text)
	step := size - overlap
	if step <= 0 {
		step = size
	}
	var out []rawChunk
	for start := 0; start < len(runes); start += step {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			out = append(out, rawChunk{Content: chunk, TokenCount: tokenCount(chunk)})
		}
		if end == len(runes) {
			break
		}
	}
	return out
}

func nonEmpty(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func tokenCount(text string) int {
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return utf8.RuneCountInString(text)
	}
	return len(tokens)
}
