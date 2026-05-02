package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv              string
	HTTPAddr            string
	GatewayAPIKey       string
	MaxBodyBytes        int64
	CORSAllowOrigins    []string
	MySQLDSN            string
	RedisAddr           string
	RedisPassword       string
	RedisDB             int
	QdrantURL           string
	QdrantCollection    string
	EmbeddingDimension  int
	RAGScoreThreshold   float64
	LLMBaseURL          string
	LLMAPIKey           string
	LLMModel            string
	EmbeddingModel      string
	ToolDefaultTimeout  time.Duration
	RateLimitPerMinute  int
	MaxSQLRows          int
	EnableMockLLM       bool
	EnableMockEmbedding bool
	RateLimitFailClosed bool
	URLAllowlist        []string
	AsyncWorkers        int
	AsyncQueueSize      int
}

func Load() Config {
	loadEnvFile(".env")
	return Config{
		AppEnv:              env("APP_ENV", "local"),
		HTTPAddr:            env("HTTP_ADDR", ":8080"),
		GatewayAPIKey:       os.Getenv("GATEWAY_API_KEY"),
		MaxBodyBytes:        int64(envInt("MAX_BODY_BYTES", 1048576)),
		CORSAllowOrigins:    split(env("CORS_ALLOW_ORIGINS", "")),
		MySQLDSN:            env("MYSQL_DSN", ""),
		RedisAddr:           env("REDIS_ADDR", "localhost:6379"),
		RedisPassword:       env("REDIS_PASSWORD", ""),
		RedisDB:             envInt("REDIS_DB", 0),
		QdrantURL:           env("QDRANT_URL", "http://localhost:6333"),
		QdrantCollection:    env("QDRANT_COLLECTION", "agent_documents"),
		EmbeddingDimension:  envInt("EMBEDDING_DIMENSION", 64),
		RAGScoreThreshold:   envFloat("RAG_SCORE_THRESHOLD", 0.10),
		LLMBaseURL:          env("LLM_BASE_URL", "https://api.openai.com/v1"),
		LLMAPIKey:           os.Getenv("LLM_API_KEY"),
		LLMModel:            env("LLM_MODEL", "gpt-4o-mini"),
		EmbeddingModel:      env("EMBEDDING_MODEL", "text-embedding-3-small"),
		ToolDefaultTimeout:  time.Duration(envInt("TOOL_DEFAULT_TIMEOUT_MS", 5000)) * time.Millisecond,
		RateLimitPerMinute:  envInt("RATE_LIMIT_PER_MINUTE", 60),
		MaxSQLRows:          envInt("MAX_SQL_ROWS", 100),
		EnableMockLLM:       envBool("ENABLE_MOCK_LLM", true),
		EnableMockEmbedding: envBool("ENABLE_MOCK_EMBEDDING", true),
		RateLimitFailClosed: envBool("RATE_LIMIT_FAIL_CLOSED", false),
		URLAllowlist:        split(env("URL_ALLOWLIST", "example.com,httpbin.org,api.github.com")),
		AsyncWorkers:        envInt("ASYNC_WORKERS", 2),
		AsyncQueueSize:      envInt("ASYNC_QUEUE_SIZE", 32),
	}
}

func loadEnvFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" || os.Getenv(key) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes"
}

func envFloat(key string, fallback float64) float64 {
	v, err := strconv.ParseFloat(os.Getenv(key), 64)
	if err != nil {
		return fallback
	}
	return v
}

func split(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
