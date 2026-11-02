package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"

	"github.com/gladart32/gym/internal/config"
	"github.com/gladart32/gym/internal/database"
)

// Worker runs an asynq server (task processors) and the scheduler (cron).
type Worker struct {
	server    *asynq.Server
	scheduler *asynq.Scheduler
	log       *slog.Logger
	db        *sqlx.DB
}

func NewWorker(cfg *config.Config, log *slog.Logger, db *sqlx.DB) *Worker {
	redisOpt := database.AsynqRedisConnOpt(cfg.Redis)
	logger := asynqLogger{log: log}

	server := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: cfg.Asynq.Concurrency,
		Queues:      cfg.Asynq.Queues,
		Logger:      logger,
	})

	scheduler := asynq.NewScheduler(redisOpt, &asynq.SchedulerOpts{
		Logger:   logger,
		LogLevel: asynq.InfoLevel,
	})

	return &Worker{server: server, scheduler: scheduler, log: log, db: db}
}

// Run blocks until ctx is cancelled, then shuts down gracefully.
func (w *Worker) Run(ctx context.Context) error {
	mux := asynq.NewServeMux()
	mux.Handle(TypeExample, &Example{Log: w.log})
	mux.Handle(TypeDBCleanup, NewCleanup(w.log, w.db))

	if err := w.registerSchedules(); err != nil {
		return fmt.Errorf("register schedules: %w", err)
	}

	if err := w.server.Start(mux); err != nil {
		return fmt.Errorf("start asynq server: %w", err)
	}

	schedErr := make(chan error, 1)
	go func() {
		if err := w.scheduler.Run(); err != nil {
			schedErr <- fmt.Errorf("run scheduler: %w", err)
		}
	}()

	w.log.Info("task worker started")

	select {
	case err := <-schedErr:
		w.server.Shutdown()
		return err
	case <-ctx.Done():
		w.log.Info("shutting down task worker")
		w.scheduler.Shutdown()
		w.server.Shutdown()
		w.log.Info("task worker stopped")
		return nil
	}
}

// asynqLogger adapts slog to the asynq.Logger interface.
type asynqLogger struct {
	log *slog.Logger
}

func (l asynqLogger) Debug(args ...interface{}) { l.log.Debug(fmt.Sprint(args...)) }
func (l asynqLogger) Info(args ...interface{})  { l.log.Info(fmt.Sprint(args...)) }
func (l asynqLogger) Warn(args ...interface{})  { l.log.Warn(fmt.Sprint(args...)) }
func (l asynqLogger) Error(args ...interface{}) { l.log.Error(fmt.Sprint(args...)) }
func (l asynqLogger) Fatal(args ...interface{}) {
	l.log.Error(fmt.Sprint(args...))
	os.Exit(1)
}
