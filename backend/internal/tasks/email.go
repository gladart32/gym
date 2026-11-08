package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
)

// SendEmail processes tasks:send_email. Until SMTP is configured this is a
// stub mailer: it writes the full message (including action links) to the
// worker log.
type SendEmail struct {
	Log *slog.Logger
}

func (h *SendEmail) ProcessTask(_ context.Context, t *asynq.Task) error {
	var p SendEmailPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal send_email payload: %v: %w", err, asynq.SkipRetry)
	}

	h.Log.Info("email sent (stub mailer)",
		"to", p.To, "subject", p.Subject, "body", p.Body)
	return nil
}
