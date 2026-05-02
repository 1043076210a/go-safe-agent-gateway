package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-safe-agent-gateway/internal/config"
	"go-safe-agent-gateway/internal/executor"
	"go-safe-agent-gateway/internal/handler"
	"go-safe-agent-gateway/internal/llm"
	"go-safe-agent-gateway/internal/observability"
	"go-safe-agent-gateway/internal/policy"
	"go-safe-agent-gateway/internal/rag"
	"go-safe-agent-gateway/internal/repository"
	"go-safe-agent-gateway/internal/router"
	"go-safe-agent-gateway/internal/service"
	"go-safe-agent-gateway/internal/tool"
	"go-safe-agent-gateway/pkg/logger"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := runHealthcheck(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	ctx := context.Background()
	cfg := config.Load()
	log := logger.New(cfg.AppEnv)

	shutdownTracer, err := observability.InitTracer(ctx)
	if err != nil {
		log.Error("init tracer failed", slog.String("error_type", "tracer_init"))
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = shutdownTracer(shutdownCtx)
	}()

	store := repository.NewStore(ctx, cfg.MySQLDSN)
	redisClient := repository.NewRedisStore(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	pingCtx, cancel := context.WithTimeout(ctx, time.Second)
	redisForPolicy := repository.RedisClient(redisClient)
	if err := redisClient.Ping(pingCtx); err != nil {
		redisForPolicy = nil
		log.Warn("redis unavailable; rate limit cache disabled", slog.String("error_type", "redis_unavailable"))
	}
	cancel()

	var embedder rag.Embedder
	if cfg.EnableMockEmbedding {
		embedder = rag.NewDeterministicEmbedder(cfg.EmbeddingDimension)
	} else {
		embedder = rag.NewOpenAIEmbeddingClient(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.EmbeddingModel, cfg.EmbeddingDimension, 10*time.Second)
	}
	ragService := rag.NewService(
		store,
		rag.NewQdrantClient(cfg.QdrantURL, cfg.QdrantCollection, embedder.Dimension(), 5*time.Second),
		embedder,
		cfg.RAGScoreThreshold,
	)

	reg := tool.NewRegistry()
	if err := tool.RegisterBuiltins(reg, store, ragService, cfg.MaxSQLRows); err != nil {
		log.Error("register tools failed", slog.String("error_type", "tool_registration"))
		os.Exit(1)
	}
	metrics := observability.NewMetrics()
	policies := policy.NewEngine(policy.Config{
		AllowedTools:        []string{"calculator", "query_mysql_readonly", "http_get", "search_knowledge_base", "query_logs"},
		URLAllowlist:        cfg.URLAllowlist,
		RateLimiter:         redisForPolicy,
		RateLimitPerMinute:  cfg.RateLimitPerMinute,
		RateLimitFailClosed: cfg.RateLimitFailClosed,
		RequireSQLLimit:     true,
		BlockedSQLTables:    []string{"users", "credentials", "secrets"},
	})
	exec := executor.New(reg, policies, store, executor.Options{
		DefaultTimeout: cfg.ToolDefaultTimeout,
		AsyncWorkers:   cfg.AsyncWorkers,
		AsyncQueueSize: cfg.AsyncQueueSize,
		Metrics:        metrics,
	})

	var llmClient llm.Client
	if cfg.EnableMockLLM {
		llmClient = llm.MockClient{}
	} else {
		llmClient = llm.NewOpenAICompatibleClient(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel, 10*time.Second)
	}

	svc := service.NewAgentService(store, reg, exec, llmClient, ragService)
	app := router.New(handler.New(svc), metrics, router.Options{
		APIKey:           cfg.GatewayAPIKey,
		MaxBodyBytes:     cfg.MaxBodyBytes,
		CORSAllowOrigins: cfg.CORSAllowOrigins,
	})
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: app, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		log.Info("server starting", slog.String("addr", cfg.HTTPAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server failed", slog.String("error_type", "http_server"))
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("server shutdown failed", slog.String("error_type", "http_shutdown"))
	}
}

func runHealthcheck() error {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:8080/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck status %d", resp.StatusCode)
	}
	return nil
}
