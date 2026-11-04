package server

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/gladart32/gym/internal/config"
	"github.com/gladart32/gym/internal/transport/http/handler"
	"github.com/gladart32/gym/internal/transport/http/middleware"
)

func NewRouter(log *slog.Logger, cfg *config.Config, health *handler.Health, tasksH *handler.Tasks) *gin.Engine {
	if cfg.Env == "dev" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(
		middleware.RequestID(),
		middleware.Logger(log),
		middleware.Recover(log),
	)

	r.GET("/healthz", health.Liveness)
	r.GET("/ready", health.Readiness)
	r.Static("/media", "media")

	api := r.Group("/api/v1")
	{
		api.POST("/tasks/example", tasksH.EnqueueExample)
		api.POST("/tasks/db-cleanup", tasksH.EnqueueDBCleanup)
	}

	return r
}
