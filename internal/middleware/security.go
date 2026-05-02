package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	appErrors "go-safe-agent-gateway/pkg/errors"
	"go-safe-agent-gateway/pkg/response"
)

func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes > 0 && c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	allowAll := false
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin == "*" {
			allowAll = true
		}
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			if allowAll {
				c.Header("Access-Control-Allow-Origin", origin)
			} else if _, ok := allowed[origin]; ok {
				c.Header("Access-Control-Allow-Origin", origin)
			}
			if c.Writer.Header().Get("Access-Control-Allow-Origin") != "" {
				c.Header("Vary", "Origin")
				c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
				c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				c.Header("Access-Control-Max-Age", strconv.Itoa(600))
			}
		}
		if c.Request.Method == http.MethodOptions {
			if c.Writer.Header().Get("Access-Control-Allow-Origin") == "" {
				response.Error(c, http.StatusForbidden, appErrors.CodeUnauthorizedTool, "origin is not allowed")
				c.Abort()
				return
			}
			c.Status(http.StatusNoContent)
			c.Abort()
			return
		}
		c.Next()
	}
}
