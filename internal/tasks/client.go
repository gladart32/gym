package tasks

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
)

// Client is a thin wrapper around asynq.Client used by the API layer.
type Client struct {
	client *asynq.Client
}

func NewClient(redisConnOpt asynq.RedisConnOpt) *Client {
	return &Client{client: asynq.NewClient(redisConnOpt)}
}

func (c *Client) EnqueueExample(ctx context.Context, payload ExamplePayload) (*asynq.TaskInfo, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return c.client.EnqueueContext(ctx, asynq.NewTask(TypeExample, data))
}

func (c *Client) EnqueueDBCleanup(ctx context.Context) (*asynq.TaskInfo, error) {
	return c.client.EnqueueContext(ctx,
		asynq.NewTask(TypeDBCleanup, nil),
		asynq.Queue(QueueMaintenance),
	)
}

func (c *Client) Close() error {
	return c.client.Close()
}
