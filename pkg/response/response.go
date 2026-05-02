package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Body struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: 0, Message: "success", Data: data})
}

func Error(c *gin.Context, status, code int, message string) {
	c.JSON(status, Body{Code: code, Message: message, Data: nil})
}
