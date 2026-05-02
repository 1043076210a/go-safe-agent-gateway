package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"go-safe-agent-gateway/internal/observability"
)

func Metrics(metrics *observability.Metrics) gin.HandlerFunc {
	return func(c *gin.Context) {
		if metrics == nil {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()
		status := strconv.Itoa(c.Writer.Status())
		route := c.FullPath()
		if route == "" {
			route = "unknown"
		}
		metrics.HTTPDuration.WithLabelValues(c.Request.Method, route, status).Observe(time.Since(start).Seconds())
		if route == "/v1/agent/chat" {
			agentStatus := "success"
			if c.Writer.Status() >= 400 {
				agentStatus = "failed"
			}
			metrics.AgentRequests.WithLabelValues(agentStatus).Inc()
		}
	}
}
