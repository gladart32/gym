package tasks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
	"github.com/jmoiron/sqlx"
)

// Cleanup processes db:cleanup: removes stale rows past their retention
// period (docs/AUTH.md, п.8). Scheduled daily by the scheduler, can also be
// enqueued manually via API.
type Cleanup struct {
	log *slog.Logger
	db  *sqlx.DB
}

func NewCleanup(log *slog.Logger, db *sqlx.DB) *Cleanup {
	return &Cleanup{log: log, db: db}
}

func (h *Cleanup) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	steps := []struct {
		name  string
		query string
	}{
		{
			name:  "users",
			query: `DELETE FROM users WHERE created_at < DATE_SUB(UTC_TIMESTAMP(), INTERVAL 365 DAY)`,
		},
		{
			name: "login_sessions",
			query: `DELETE FROM login_sessions
				WHERE (revoked_at IS NOT NULL AND revoked_at < DATE_SUB(UTC_TIMESTAMP(), INTERVAL 30 DAY))
				   OR expires_at < DATE_SUB(UTC_TIMESTAMP(), INTERVAL 30 DAY)`,
		},
		{
			name:  "email_action_tokens",
			query: `DELETE FROM email_action_tokens WHERE expires_at < DATE_SUB(UTC_TIMESTAMP(), INTERVAL 7 DAY)`,
		},
	}

	for _, step := range steps {
		res, err := h.db.ExecContext(ctx, step.query)
		if err != nil {
			return fmt.Errorf("cleanup %s: %w", step.name, err)
		}
		rows, _ := res.RowsAffected()
		h.log.InfoContext(ctx, "db cleanup step finished",
			"table", step.name, "rows_deleted", rows)
	}

	return nil
}
