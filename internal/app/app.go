// Package app wires all application dependencies manually (no DI frameworks).
package app

import (
	"log/slog"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/gladart32/gym/internal/config"
	"github.com/gladart32/gym/internal/database"
	"github.com/gladart32/gym/internal/logger"
	"github.com/gladart32/gym/internal/server"
	"github.com/gladart32/gym/internal/tasks"
	"github.com/gladart32/gym/internal/transport/http/handler"
)

type App struct {
	Config *config.Config
	Log    *slog.Logger
	DB     *sqlx.DB
	Redis  *redis.Client
	Tasks  *tasks.Client
}

func New(cfg *config.Config) (*App, error) {
	log := logger.New(cfg.Logger)

	db, err := database.NewMySQL(cfg.MySQL)
	if err != nil {
		return nil, err
	}

	rdb, err := database.NewRedis(cfg.Redis)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	tasksClient := tasks.NewClient(database.AsynqRedisConnOpt(cfg.Redis))

	log.Info("application initialized", "env", cfg.Env)

	return &App{
		Config: cfg,
		Log:    log,
		DB:     db,
		Redis:  rdb,
		Tasks:  tasksClient,
	}, nil
}

// Close releases resources in reverse dependency order.
func (a *App) Close() {
	if err := a.Tasks.Close(); err != nil {
		a.Log.Error("close asynq client", "error", err)
	}
	if err := a.Redis.Close(); err != nil {
		a.Log.Error("close redis", "error", err)
	}
	if err := a.DB.Close(); err != nil {
		a.Log.Error("close mysql", "error", err)
	}
}

// HTTPServer builds the API server with all handlers wired.
func (a *App) HTTPServer() *server.Server {
	healthH := handler.NewHealth(a.Log, a.DB, a.Redis)
	tasksH := handler.NewTasks(a.Log, a.Tasks)
	return server.New(a.Config, a.Log, healthH, tasksH)
}

// TaskWorker builds the background worker (asynq server + scheduler).
func (a *App) TaskWorker() *tasks.Worker {
	return tasks.NewWorker(a.Config, a.Log, a.DB)
}
