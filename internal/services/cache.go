package services

import (
	"context"
	"time"
)

// Cache defines the interface for caching operations
type Cache interface {
	// Ping tests the cache connection
	Ping(ctx context.Context) error

	// Set stores a key-value pair with optional expiration
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error

	// Get retrieves a value by key
	Get(ctx context.Context, key string) (string, error)

	// Del deletes one or more keys
	Del(ctx context.Context, keys ...string) error

	// Exists checks if keys exist
	Exists(ctx context.Context, keys ...string) (bool, error)

	// Close closes the cache connection
	Close() error

	// WaitForConnection waits for cache to be available with retries
	WaitForConnection(ctx context.Context) error
}
