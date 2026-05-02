package rag

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode"
)

type DeterministicEmbedder struct {
	dim int
}

func NewDeterministicEmbedder(dim int) *DeterministicEmbedder {
	if dim <= 0 {
		dim = 64
	}
	return &DeterministicEmbedder{dim: dim}
}

func (e *DeterministicEmbedder) Dimension() int { return e.dim }

func (e *DeterministicEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for _, text := range texts {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		vec := make([]float32, e.dim)
		for _, token := range tokenize(text) {
			hash := sha256.Sum256([]byte(token))
			idx := int(binary.BigEndian.Uint64(hash[:8]) % uint64(e.dim))
			sign := float32(1)
			if hash[8]%2 == 1 {
				sign = -1
			}
			vec[idx] += sign
		}
		normalize(vec)
		out = append(out, vec)
	}
	return out, nil
}

type OpenAIEmbeddingClient struct {
	baseURL    string
	apiKey     string
	model      string
	dimension  int
	httpClient *http.Client
}

func NewOpenAIEmbeddingClient(baseURL, apiKey, model string, dimension int, timeout time.Duration) *OpenAIEmbeddingClient {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &OpenAIEmbeddingClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		model:      model,
		dimension:  dimension,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *OpenAIEmbeddingClient) Dimension() int { return c.dimension }

func (c *OpenAIEmbeddingClient) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if c.apiKey == "" {
		return nil, errors.New("embedding api key is empty")
	}
	payload := embeddingRequest{Model: c.model, Input: texts}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding request failed with status %d", resp.StatusCode)
	}
	var out embeddingResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	vectors := make([][]float32, 0, len(out.Data))
	for _, item := range out.Data {
		if c.dimension > 0 && len(item.Embedding) != c.dimension {
			return nil, fmt.Errorf("embedding dimension mismatch: got %d want %d", len(item.Embedding), c.dimension)
		}
		vectors = append(vectors, item.Embedding)
	}
	if len(vectors) != len(texts) {
		return nil, fmt.Errorf("embedding response count mismatch")
	}
	return vectors, nil
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func tokenize(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}

func normalize(vec []float32) {
	var sum float64
	for _, v := range vec {
		sum += float64(v * v)
	}
	if sum == 0 {
		return
	}
	norm := float32(math.Sqrt(sum))
	for i := range vec {
		vec[i] /= norm
	}
}
