package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"
)

// Example processes tasks:example — a demo task that just logs its payload.
type Example struct {
	Log *slog.Logger
}

func (h *Example) ProcessTask(_ context.Context, t *asynq.Task) error {
	var p ExamplePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("unmarshal example payload: %v: %w", err, asynq.SkipRetry)
	}

	h.Log.Info("example task processed", "message", p.Message)
	return nil
}
