package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type QdrantClient struct {
	baseURL    string
	collection string
	dimension  int
	httpClient *http.Client
}

type qdrantPoint struct {
	ID      string         `json:"id"`
	Vector  []float32      `json:"vector"`
	Payload map[string]any `json:"payload"`
}

type qdrantSearchHit struct {
	ID      any            `json:"id"`
	Score   float64        `json:"score"`
	Payload map[string]any `json:"payload"`
}

func NewQdrantClient(baseURL, collection string, dimension int, timeout time.Duration) *QdrantClient {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &QdrantClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		collection: collection,
		dimension:  dimension,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *QdrantClient) EnsureCollection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/collections/"+c.collection, nil)
	if err != nil {
		return err
	}
	_, status, err := c.do(req)
	if err != nil {
		return err
	}
	if status >= 200 && status < 300 {
		return nil
	}
	if status != http.StatusNotFound {
		return fmt.Errorf("qdrant collection check failed with status %d", status)
	}
	payload := map[string]any{
		"vectors": map[string]any{
			"size":     c.dimension,
			"distance": "Cosine",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/collections/"+c.collection, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	respBody, status, err := c.do(req)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("qdrant ensure collection failed with status %d: %s", status, string(respBody))
	}
	return nil
}

func (c *QdrantClient) Upsert(ctx context.Context, points []qdrantPoint) error {
	if len(points) == 0 {
		return nil
	}
	payload := map[string]any{"points": points}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/collections/"+c.collection+"/points?wait=true", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	respBody, status, err := c.do(req)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("qdrant upsert failed with status %d: %s", status, string(respBody))
	}
	return nil
}

func (c *QdrantClient) Search(ctx context.Context, vector []float32, topK int, scoreThreshold float64) ([]qdrantSearchHit, error) {
	if topK <= 0 {
		topK = 5
	}
	payload := map[string]any{
		"vector":       vector,
		"limit":        topK,
		"with_payload": true,
	}
	if scoreThreshold > 0 {
		payload["score_threshold"] = scoreThreshold
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/collections/"+c.collection+"/points/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	respBody, status, err := c.do(req)
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return []qdrantSearchHit{}, nil
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("qdrant search failed with status %d: %s", status, string(respBody))
	}
	var out struct {
		Result []qdrantSearchHit `json:"result"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	return out.Result, nil
}

func (c *QdrantClient) do(req *http.Request) ([]byte, int, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}
