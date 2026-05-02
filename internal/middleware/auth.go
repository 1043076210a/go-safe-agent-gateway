package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	appErrors "go-safe-agent-gateway/pkg/errors"
	"go-safe-agent-gateway/pkg/response"
)

func APIKey(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiKey == "" {
			c.Next()
			return
		}
		const prefix = "Bearer "
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, prefix) || strings.TrimSpace(strings.TrimPrefix(auth, prefix)) != apiKey {
			response.Error(c, http.StatusUnauthorized, appErrors.CodeUnauthorizedTool, "unauthorized")
			c.Abort()
			return
		}
		c.Next()
	}
}
