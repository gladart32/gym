package database

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"github.com/gladart32/gym/internal/config"
)

// NewRedis creates a redis client and checks the connection.
func NewRedis(cfg config.Redis) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return rdb, nil
}

// AsynqRedisConnOpt builds connection options for asynq from the same config.
func AsynqRedisConnOpt(cfg config.Redis) asynq.RedisConnOpt {
	return asynq.RedisClientOpt{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	}
}
