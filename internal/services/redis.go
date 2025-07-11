package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisService implements the Cache interface using Redis
type RedisService struct {
	client *redis.Client
	logger *slog.Logger
}

// Ensure RedisService implements Cache interface
var _ Cache = (*RedisService)(nil)

// NewRedisService creates a new Redis service instance
func NewRedisService(redisURL string, logger *slog.Logger) *RedisService {
	rdb := redis.NewClient(&redis.Options{
		Addr: redisURL,
	})

	return &RedisService{
		client: rdb,
		logger: logger,
	}
}

func (r *RedisService) Ping(ctx context.Context) error {
	cmd := r.client.Ping(ctx)
	if err := cmd.Err(); err != nil {
		return fmt.Errorf("redis ping failed: %w", err)
	}

	r.logger.Debug("Redis ping successful", "result", cmd.Val())
	return nil
}

func (r *RedisService) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	cmd := r.client.Set(ctx, key, value, expiration)
	if err := cmd.Err(); err != nil {
		r.logger.Error("Redis SET failed", "key", key, "error", err)
		return fmt.Errorf("redis set failed: %w", err)
	}

	r.logger.Debug("Redis SET successful", "key", key)
	return nil
}

func (r *RedisService) Get(ctx context.Context, key string) (string, error) {
	cmd := r.client.Get(ctx, key)
	if err := cmd.Err(); err != nil {
		if err == redis.Nil {
			r.logger.Debug("Redis key not found", "key", key)
			return "", nil // Return empty string for not found, not an error
		}
		r.logger.Error("Redis GET failed", "key", key, "error", err)
		return "", fmt.Errorf("redis get failed: %w", err)
	}

	value := cmd.Val()
	r.logger.Debug("Redis GET successful", "key", key, "value_length", len(value))
	return value, nil
}

func (r *RedisService) Del(ctx context.Context, keys ...string) error {
	cmd := r.client.Del(ctx, keys...)
	if err := cmd.Err(); err != nil {
		r.logger.Error("Redis DEL failed", "keys", keys, "error", err)
		return fmt.Errorf("redis del failed: %w", err)
	}

	deleted := cmd.Val()
	r.logger.Debug("Redis DEL successful", "keys", keys, "deleted_count", deleted)
	return nil
}

func (r *RedisService) Exists(ctx context.Context, keys ...string) (bool, error) {
	cmd := r.client.Exists(ctx, keys...)
	if err := cmd.Err(); err != nil {
		r.logger.Error("Redis EXISTS failed", "keys", keys, "error", err)
		return false, fmt.Errorf("redis exists failed: %w", err)
	}

	exists := cmd.Val() > 0
	r.logger.Debug("Redis EXISTS check", "keys", keys, "exists", exists)
	return exists, nil
}

func (r *RedisService) Close() error {
	if err := r.client.Close(); err != nil {
		r.logger.Error("Failed to close Redis connection", "error", err)
		return err
	}

	r.logger.Info("Redis connection closed")
	return nil
}

func (r *RedisService) GetClient() *redis.Client {
	return r.client
}

func (r *RedisService) WaitForConnection(ctx context.Context) error {
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
