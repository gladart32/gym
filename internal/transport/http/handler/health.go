package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
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
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
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

	status := "ok"
	code := http.StatusOK
	if !healthy {
		status = "unavailable"
		code = http.StatusServiceUnavailable
	}

	c.JSON(code, gin.H{"status": status, "checks": checks})
}
