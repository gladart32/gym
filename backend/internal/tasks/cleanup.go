package tasks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
)

// Cleanup processes db:cleanup: removes users older than a retention period.
// Scheduled daily by the scheduler, can also be enqueued manually via API.
type Cleanup struct {
	log *slog.Logger
	db  *sqlx.DB
}

func NewCleanup(log *slog.Logger, db *sqlx.DB) *Cleanup {
	return &Cleanup{log: log, db: db}
}

func (h *Cleanup) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	const query = `DELETE FROM users WHERE created_at < DATE_SUB(UTC_TIMESTAMP(), INTERVAL 365 DAY)`

	res, err := h.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("cleanup query: %w", err)
	}

	rows, _ := res.RowsAffected()
	h.log.InfoContext(ctx, "db cleanup finished", "rows_deleted", rows)

	return nil
}
