package tasks

import (
	"time"

	"github.com/hibiken/asynq"
)

// registerSchedules registers periodic tasks. The scheduler must run as a
// single instance (it runs inside the worker process).
func (w *Worker) registerSchedules() error {
	cleanup := asynq.NewTask(TypeDBCleanup, nil)

	_, err := w.scheduler.Register(
		"0 3 * * *", // every day at 03:00
		cleanup,
		asynq.Queue(QueueMaintenance),
		asynq.Unique(24*time.Hour),
	)
	if err != nil {
		return err
	}

	return nil
}
