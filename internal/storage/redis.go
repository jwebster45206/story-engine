package storage

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisStorage implements the Storage interface using Redis for gamestate
// and filesystem for static resources (scenarios, narrators, PCs)
type RedisStorage struct {
	client  *redis.Client
	logger  *slog.Logger
	dataDir string
}

// Ensure RedisStorage implements Storage interface
var _ Storage = (*RedisStorage)(nil)

// NewRedisStorage creates a new Redis storage instance
func NewRedisStorage(redisURL string, dataDir string, logger *slog.Logger) *RedisStorage {
	rdb := redis.NewClient(&redis.Options{
		Addr: redisURL,
	})

	if dataDir == "" {
		dataDir = "./data"
	}

	return &RedisStorage{
		client:  rdb,
		logger:  logger,
		dataDir: dataDir,
	}
}

// Health and lifecycle methods

func (r *RedisStorage) Ping(ctx context.Context) error {
	cmd := r.client.Ping(ctx)
	if err := cmd.Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}
	return nil
}

func (r *RedisStorage) Close() error {
	if err := r.client.Close(); err != nil {
		r.logger.Error("Failed to close Redis connection", "error", err)
		return err
	}
	r.logger.Info("Redis connection closed")
	return nil
}

// WaitForConnection waits for Redis to become available (used during startup)
func (r *RedisStorage) WaitForConnection(ctx context.Context) error {
	maxRetries := 30
	retryDelay := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		if err := r.Ping(ctx); err != nil {
			r.logger.Debug("Redis not ready yet", "error", err, "attempt", i+1)

			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled while waiting for redis: %w", ctx.Err())
			case <-time.After(retryDelay):
				continue
			}
		}

		r.logger.Info("Redis connection established")
		return nil
	}

	return fmt.Errorf("redis did not become available after %d attempts", maxRetries)
}
