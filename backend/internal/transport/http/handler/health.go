package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/gladart32/gym/internal/transport/http/middleware"
	"github.com/gladart32/gym/internal/transport/http/response"
)

// Health provides liveness and readiness endpoints.
type Health struct {
	log   *slog.Logger
	db    *sqlx.DB
	redis *redis.Client
}

func NewHealth(log *slog.Logger, db *sqlx.DB, rdb *redis.Client) *Health {
	return &Health{log: log, db: db, redis: rdb}
}

func (h *Health) Liveness(c *gin.Context) {
	response.Success(c, http.StatusOK, nil)
}

func (h *Health) Readiness(c *gin.Context) {
	checks := gin.H{}
	healthy := true

	if err := h.db.PingContext(c.Request.Context()); err != nil {
		checks["mysql"] = err.Error()
		healthy = false
	} else {
		checks["mysql"] = "ok"
	}

	if err := h.redis.Ping(c.Request.Context()).Err(); err != nil {
		checks["redis"] = err.Error()
		healthy = false
	} else {
		checks["redis"] = "ok"
	}

	if !healthy {
		h.log.ErrorContext(c.Request.Context(), "readiness check failed",
			"checks", checks,
			"request_id", middleware.RequestIDFrom(c),
		)
		response.Error(c, http.StatusServiceUnavailable, "service unavailable")
		return
	}

	response.Success(c, http.StatusOK, gin.H{"checks": checks})
}
