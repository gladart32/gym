package tasks_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"

	"github.com/hibiken/asynq"

	"github.com/gladart32/gym/internal/tasks"
)

func TestExampleProcessTask(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := &tasks.Example{Log: log}

	payload, err := json.Marshal(tasks.ExamplePayload{Message: "integration test"})
	if err != nil {
		t.Fatal(err)
	}

	if err := h.ProcessTask(context.Background(), asynq.NewTask(tasks.TypeExample, payload)); err != nil {
		t.Fatalf("ProcessTask: %v", err)
	}
}

func TestExampleProcessTaskBadPayload(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := &tasks.Example{Log: log}

	if err := h.ProcessTask(context.Background(), asynq.NewTask(tasks.TypeExample, []byte("{invalid"))); err == nil {
		t.Fatal("expected error for invalid payload")
	}
}
