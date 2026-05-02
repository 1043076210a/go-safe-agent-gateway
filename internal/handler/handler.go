package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"go-safe-agent-gateway/internal/rag"
	"go-safe-agent-gateway/internal/service"
	appErrors "go-safe-agent-gateway/pkg/errors"
	"go-safe-agent-gateway/pkg/response"
)

type Handler struct {
	service *service.AgentService
}

func New(service *service.AgentService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Health(c *gin.Context) {
	if err := h.service.Health(c.Request.Context()); err != nil {
		response.Error(c, http.StatusServiceUnavailable, appErrors.CodeInternal, "unhealthy")
		return
	}
	response.Success(c, gin.H{"status": "ok"})
}

func (h *Handler) ListTools(c *gin.Context) {
	response.Success(c, h.service.ListTools(c.Request.Context()))
}

func (h *Handler) ExecuteTool(c *gin.Context) {
	var req service.ExecuteToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, appErrors.CodeInvalidRequest, err.Error())
		return
	}
	out, err := h.service.ExecuteTool(c.Request.Context(), req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, appErrors.CodeUnauthorizedTool, err.Error())
		return
	}
	response.Success(c, out)
}

func (h *Handler) Chat(c *gin.Context) {
	var req service.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, appErrors.CodeInvalidRequest, err.Error())
		return
	}
	out, err := h.service.Chat(c.Request.Context(), req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, appErrors.CodeInternal, err.Error())
		return
	}
	response.Success(c, out)
}

func (h *Handler) IndexDocument(c *gin.Context) {
	var req rag.IndexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, appErrors.CodeInvalidRequest, err.Error())
		return
	}
	out, err := h.service.IndexDocument(c.Request.Context(), req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, appErrors.CodeInternal, err.Error())
		return
	}
	response.Success(c, out)
}

func (h *Handler) CreateSession(c *gin.Context) {
	var req struct {
		UserID string `json:"user_id"`
		Title  string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, appErrors.CodeInvalidRequest, err.Error())
		return
	}
	out, err := h.service.CreateSession(c.Request.Context(), req.UserID, req.Title)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, appErrors.CodeInternal, err.Error())
		return
	}
	response.Success(c, out)
}

func (h *Handler) ListMessages(c *gin.Context) {
	out, err := h.service.ListMessages(c.Request.Context(), c.Param("id"), queryInt(c, "limit", 20), queryInt(c, "offset", 0))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, appErrors.CodeInternal, err.Error())
		return
	}
	response.Success(c, out)
}

func (h *Handler) GetAsyncTask(c *gin.Context) {
	out, err := h.service.GetAsyncTask(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusNotFound, appErrors.CodeNotFound, err.Error())
		return
	}
	response.Success(c, out)
}

func (h *Handler) ListToolCalls(c *gin.Context) {
	out, err := h.service.ListToolCalls(c.Request.Context(), queryInt(c, "limit", 20), queryInt(c, "offset", 0))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, appErrors.CodeInternal, err.Error())
		return
	}
	response.Success(c, out)
}

func (h *Handler) ListPolicyRejects(c *gin.Context) {
	out, err := h.service.ListPolicyRejects(c.Request.Context(), queryInt(c, "limit", 20), queryInt(c, "offset", 0))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, appErrors.CodeInternal, err.Error())
		return
	}
	response.Success(c, out)
}

func queryInt(c *gin.Context, key string, fallback int) int {
	v, err := strconv.Atoi(c.Query(key))
	if err != nil {
		return fallback
	}
	return v
}
