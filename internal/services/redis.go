package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
)

// RedisService implements the Storage interface using Redis
type RedisService struct {
	client *redis.Client
	logger *slog.Logger
}

// Ensure RedisService implements Storage interface
var _ Storage = (*RedisService)(nil)

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

// Storage interface implementation methods

// SaveGameState saves a gamestate with the given UUID
func (r *RedisService) SaveGameState(ctx context.Context, uuid uuid.UUID, gamestate *state.GameState) error {
	// Update the UpdatedAt timestamp
	gamestate.UpdatedAt = time.Now()

	// Marshal gamestate to JSON
	data, err := json.Marshal(gamestate)
	if err != nil {
		r.logger.Error("Failed to marshal gamestate", "uuid", uuid, "error", err)
		return fmt.Errorf("failed to marshal gamestate: %w", err)
	}

	// Use gamestate: prefix for gamestate keys
	key := "gamestate:" + uuid.String()
	if err := r.Set(ctx, key, string(data), time.Hour); err != nil {
		r.logger.Error("Failed to save gamestate", "uuid", uuid, "error", err)
		return fmt.Errorf("failed to save gamestate: %w", err)
	}

	r.logger.Debug("Gamestate saved successfully", "uuid", uuid)
	return nil
}

// LoadGameState retrieves a gamestate by UUID
func (r *RedisService) LoadGameState(ctx context.Context, uuid uuid.UUID) (*state.GameState, error) {
	key := "gamestate:" + uuid.String()
	data, err := r.Get(ctx, key)
	if err != nil {
		r.logger.Error("Failed to load gamestate", "uuid", uuid, "error", err)
		return nil, fmt.Errorf("failed to load gamestate: %w", err)
	}

	if data == "" {
		r.logger.Debug("Gamestate not found", "uuid", uuid)
		return nil, nil // Return nil for not found
	}

	// Unmarshal to the specific GameState type
	var gamestate state.GameState
	if err := json.Unmarshal([]byte(data), &gamestate); err != nil {
		r.logger.Error("Failed to unmarshal gamestate", "uuid", uuid, "error", err)
		return nil, fmt.Errorf("failed to unmarshal gamestate: %w", err)
	}

	r.logger.Debug("Gamestate loaded successfully", "uuid", uuid)
	return &gamestate, nil
}

// DeleteGameState removes a gamestate by UUID
func (r *RedisService) DeleteGameState(ctx context.Context, uuid uuid.UUID) error {
	key := "gamestate:" + uuid.String()
	if err := r.Del(ctx, key); err != nil {
		r.logger.Error("Failed to delete gamestate", "uuid", uuid, "error", err)
		return fmt.Errorf("failed to delete gamestate: %w", err)
	}

	r.logger.Debug("Gamestate deleted successfully", "uuid", uuid)
	return nil
}

// ListScenarios returns a map of scenario names to filenames from the filesystem
func (r *RedisService) ListScenarios(ctx context.Context) (map[string]string, error) {
	scenariosDir := "./data/scenarios"
	scenarios := make(map[string]string)

	err := filepath.WalkDir(scenariosDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
			return nil // skip non-json files and errors
		}

		file, err := os.ReadFile(path)
		if err != nil {
			r.logger.Warn("Failed to read scenario file", "path", path, "error", err)
			return nil
		}

		var s scenario.Scenario
		if err := json.Unmarshal(file, &s); err != nil {
			r.logger.Warn("Failed to unmarshal scenario file", "path", path, "error", err)
			return nil
		}

		filename := filepath.Base(path)
		scenarios[s.Name] = filename
		return nil
	})

	if err != nil {
		r.logger.Error("Failed to walk scenarios directory", "error", err)
		return nil, fmt.Errorf("failed to list scenarios: %w", err)
	}

	r.logger.Debug("Listed scenarios", "count", len(scenarios))
	return scenarios, nil
}

// GetScenario retrieves a scenario by its filename from the filesystem
func (r *RedisService) GetScenario(ctx context.Context, filename string) (*scenario.Scenario, error) {
	key := "scenario:" + filename
	data, err := r.Get(ctx, key)
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get scenario: %w", err)
	}

	if data != "" {
		var cachedScenario scenario.Scenario
		if err := json.Unmarshal([]byte(data), &cachedScenario); err != nil {
			return nil, fmt.Errorf("failed to unmarshal cached scenario: %w", err)
		}
		r.logger.Debug("Cache hit:", "filename", filename)
		return &cachedScenario, nil
	}
	r.logger.Debug("Cache miss:", "filename", filename)

	path := filepath.Join("./data/scenarios", filename)
	file, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("scenario not found: %s", filename)
		}
		return nil, fmt.Errorf("failed to read scenario file: %w", err)
	}

	var s scenario.Scenario
	if err := json.Unmarshal(file, &s); err != nil {
		return nil, fmt.Errorf("failed to unmarshal scenario: %w", err)
	}

	// Cache the scenario
	if err := r.Set(ctx, key, string(file), 24*time.Hour); err != nil {
		// Log the error but allow a successful return
		r.logger.Error("Failed to cache scenario", "filename", filename, "error", err)
	}
	return &s, nil
}
