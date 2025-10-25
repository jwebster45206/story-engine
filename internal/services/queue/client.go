package queue

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-redis/redis/v8"
)

// Client wraps the Redis client for queue operations
type Client struct {
	rdb    *redis.Client
	logger *slog.Logger
}

// NewClient creates a new queue client
func NewClient(redisURL string, logger *slog.Logger) (*Client, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}

	rdb := redis.NewClient(opt)

	// Test connection
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	logger.Info("Connected to Redis for queue service", "url", redisURL)

	return &Client{
		rdb:    rdb,
		logger: logger,
	}, nil
}

// Close closes the Redis connection
func (c *Client) Close() error {
	return c.rdb.Close()
}

// GetRedisClient returns the underlying Redis client for direct operations
func (c *Client) GetRedisClient() *redis.Client {
	return c.rdb
}
