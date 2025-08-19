package services

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestRedisService_Basic(t *testing.T) {
	// Skip if no Redis available
	if testing.Short() {
		t.Skip("Skipping Redis integration test in short mode")
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Reduce noise in tests
	}))

	// Create Redis service (assumes Redis is running on localhost:6379)
	redisService := NewRedisService("localhost:6379", logger)
	defer func() {
		if err := redisService.Close(); err != nil {
			t.Errorf("Failed to close Redis service: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Test ping
	if err := redisService.Ping(ctx); err != nil {
		t.Skipf("Redis not available: %v", err)
	}

	// Test Set and Get
	key := "test:key:123"
	value := "test value"

	if err := redisService.Set(ctx, key, value, time.Minute); err != nil {
		t.Fatalf("Failed to set key: %v", err)
	}

	retrievedValue, err := redisService.Get(ctx, key)
	if err != nil {
		t.Fatalf("Failed to get key: %v", err)
	}

	if retrievedValue != value {
		t.Errorf("Expected '%s', got '%s'", value, retrievedValue)
	}

	// Test Exists
	exists, err := redisService.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Failed to check if key exists: %v", err)
	}

	if !exists {
		t.Error("Key should exist")
	}

	// Test Del
	if err := redisService.Del(ctx, key); err != nil {
		t.Fatalf("Failed to delete key: %v", err)
	}

	// Verify key is deleted
	exists, err = redisService.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Failed to check if key exists after deletion: %v", err)
	}

	if exists {
		t.Error("Key should not exist after deletion")
	}

	// Test Get on non-existent key
	retrievedValue, err = redisService.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get on non-existent key should not return error: %v", err)
	}

	if retrievedValue != "" {
		t.Errorf("Expected empty string for non-existent key, got '%s'", retrievedValue)
	}
}

func TestRedisService_WaitForConnection(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	t.Run("successful connection", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping Redis integration test in short mode")
		}

		redisService := NewRedisService("localhost:6379", logger)
		defer func() {
			if err := redisService.Close(); err != nil {
				t.Errorf("Failed to close Redis service: %v", err)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := redisService.WaitForConnection(ctx)
		if err != nil {
			t.Skipf("Redis not available: %v", err)
		}
	})

	t.Run("connection timeout", func(t *testing.T) {
		// Use a non-existent Redis instance
		redisService := NewRedisService("localhost:9999", logger)
		defer func() {
			if err := redisService.Close(); err != nil {
				t.Errorf("Failed to close Redis service: %v", err)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := redisService.WaitForConnection(ctx)
		if err == nil {
			t.Error("Expected timeout error, got nil")
		}
	})
}

func TestRedisService_GetClient(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	redisService := NewRedisService("localhost:6379", logger)
	defer func() {
		_ = redisService.Close() // Ignore error in defer for test
	}()

	client := redisService.GetClient()
	if client == nil {
		t.Error("GetClient should return non-nil client")
	}
}
