package router

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"go-safe-agent-gateway/internal/handler"
	"go-safe-agent-gateway/internal/middleware"
	"go-safe-agent-gateway/internal/observability"
	"go-safe-agent-gateway/internal/web"
)

type Options struct {
	APIKey           string
	MaxBodyBytes     int64
	CORSAllowOrigins []string
}

func New(h *handler.Handler, metrics *observability.Metrics, opts Options) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.BodyLimit(opts.MaxBodyBytes))
	r.Use(middleware.CORS(opts.CORSAllowOrigins))
	r.Use(middleware.Metrics(metrics))
	web.Register(r)
	r.GET("/health", h.Health)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	v1 := r.Group("/v1")
	v1.Use(middleware.APIKey(opts.APIKey))
	v1.GET("/tools", h.ListTools)
	v1.POST("/tools/execute", h.ExecuteTool)
	v1.POST("/agent/chat", h.Chat)
	v1.POST("/documents", h.IndexDocument)
	v1.POST("/sessions", h.CreateSession)
	v1.GET("/sessions/:id/messages", h.ListMessages)
	v1.GET("/async-tasks/:id", h.GetAsyncTask)
	v1.GET("/audit/tool-calls", h.ListToolCalls)
	v1.GET("/audit/policy-rejects", h.ListPolicyRejects)
	return r
}
