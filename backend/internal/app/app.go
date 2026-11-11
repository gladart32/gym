// Package app wires all application dependencies manually (no DI frameworks).
package app

import (
	"io"
	"log/slog"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/gladart32/gym/internal/auth"
	"github.com/gladart32/gym/internal/config"
	"github.com/gladart32/gym/internal/database"
	"github.com/gladart32/gym/internal/exercises"
	"github.com/gladart32/gym/internal/logger"
	"github.com/gladart32/gym/internal/profile"
	"github.com/gladart32/gym/internal/programs"
	"github.com/gladart32/gym/internal/server"
	"github.com/gladart32/gym/internal/tasks"
	"github.com/gladart32/gym/internal/transport/http/handler"
)

type App struct {
	Config    *config.Config
	Log       *slog.Logger
	DB        *sqlx.DB
	Redis     *redis.Client
	Tasks     *tasks.Client
	Auth      *auth.Service
	Profile   *profile.Service
	Exercises *exercises.Service
	Programs  *programs.Service
	logCloser io.Closer
}

func New(cfg *config.Config) (*App, error) {
	log, logCloser, err := logger.New(cfg.Logger)
	if err != nil {
		return nil, err
	}

	db, err := database.NewMySQL(cfg.MySQL)
	if err != nil {
		_ = logCloser.Close()
		return nil, err
	}

	rdb, err := database.NewRedis(cfg.Redis)
	if err != nil {
		_ = db.Close()
		_ = logCloser.Close()
		return nil, err
	}

	tasksClient := tasks.NewClient(database.AsynqRedisConnOpt(cfg.Redis))

	log.Info("application initialized",
		"env", cfg.Env,
		"log_level", cfg.Logger.Level,
		"log_format", cfg.Logger.Format,
		"log_outputs", cfg.Logger.Outputs,
	)

	return &App{
		Config:    cfg,
		Log:       log,
		DB:        db,
		Redis:     rdb,
		Tasks:     tasksClient,
		Auth:      auth.NewService(db, cfg.Auth, log),
		Profile:   profile.NewService(db, log),
		Exercises: exercises.NewService(db, log),
		Programs:  programs.NewService(db, log),
		logCloser: logCloser,
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
	if a.logCloser != nil {
		if err := a.logCloser.Close(); err != nil {
			a.Log.Error("close log file", "error", err)
		}
	}
}

// HTTPServer builds the API server with all handlers wired.
func (a *App) HTTPServer() *server.Server {
	healthH := handler.NewHealth(a.Log, a.DB, a.Redis)
	tasksH := handler.NewTasks(a.Log, a.Tasks)
	authH := handler.NewAuth(a.Log, a.Config, a.Auth, a.Tasks)
	profileH := handler.NewProfile(a.Log, a.Profile)
	exercisesH := handler.NewExercises(a.Log, a.Exercises)
	programsH := handler.NewPrograms(a.Log, a.Programs)
	return server.New(a.Config, a.Log, healthH, tasksH, authH, a.Auth, profileH, a.Profile, exercisesH, programsH, a.Programs)
}

// TaskWorker builds the background worker (asynq server + scheduler).
func (a *App) TaskWorker() *tasks.Worker {
	return tasks.NewWorker(a.Config, a.Log, a.DB)
}
