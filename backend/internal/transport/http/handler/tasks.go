package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gladart32/gym/internal/tasks"
	"github.com/gladart32/gym/internal/transport/http/middleware"
	"github.com/gladart32/gym/internal/transport/http/response"
)

// Tasks exposes endpoints for enqueueing background tasks.
type Tasks struct {
	log    *slog.Logger
	client *tasks.Client
}

func NewTasks(log *slog.Logger, client *tasks.Client) *Tasks {
	return &Tasks{log: log, client: client}
}

type enqueueExampleRequest struct {
	Message string `json:"message" binding:"required"`
}

// EnqueueExample handles POST /api/v1/tasks/example.
func (h *Tasks) EnqueueExample(c *gin.Context) {
	var req enqueueExampleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	info, err := h.client.EnqueueExample(c.Request.Context(), tasks.ExamplePayload{
		Message: req.Message,
	})
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "enqueue example task",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to enqueue task")
		return
	}

	response.Success(c, http.StatusAccepted, gin.H{
		"task_id": info.ID,
		"queue":   info.Queue,
		"type":    info.Type,
	})
}

// EnqueueDBCleanup handles POST /api/v1/tasks/db-cleanup.
func (h *Tasks) EnqueueDBCleanup(c *gin.Context) {
	info, err := h.client.EnqueueDBCleanup(c.Request.Context())
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "enqueue db cleanup task", "error", err)
		response.Error(c, http.StatusInternalServerError, "failed to enqueue task")
		return
	}

	response.Success(c, http.StatusAccepted, gin.H{
		"task_id": info.ID,
		"queue":   info.Queue,
		"type":    info.Type,
	})
}
