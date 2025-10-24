package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/actor"
	"github.com/jwebster45206/story-engine/pkg/scenario"
	"github.com/jwebster45206/story-engine/pkg/state"
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

// GameState operations (Redis-backed)

func (r *RedisStorage) SaveGameState(ctx context.Context, id uuid.UUID, gs *state.GameState) error {
	// Update the UpdatedAt timestamp
	gs.UpdatedAt = time.Now()

	// Marshal gamestate to JSON
	data, err := json.Marshal(gs)
	if err != nil {
		r.logger.Error("Failed to marshal gamestate", "uuid", id, "error", err)
		return fmt.Errorf("failed to marshal gamestate: %w", err)
	}

	// Use gamestate: prefix for gamestate keys
	key := "gamestate:" + id.String()
	cmd := r.client.Set(ctx, key, string(data), time.Hour)
	if err := cmd.Err(); err != nil {
		r.logger.Error("Failed to save gamestate", "uuid", id, "error", err)
		return fmt.Errorf("failed to save gamestate: %w", err)
	}

	return nil
}

func (r *RedisStorage) LoadGameState(ctx context.Context, id uuid.UUID) (*state.GameState, error) {
	key := "gamestate:" + id.String()
	cmd := r.client.Get(ctx, key)
	if err := cmd.Err(); err != nil {
		if err == redis.Nil {
			r.logger.Warn("Gamestate not found", "uuid", id)
			return nil, nil // Return nil for not found
		}
		r.logger.Error("Failed to load gamestate", "uuid", id, "error", err)
		return nil, fmt.Errorf("failed to load gamestate: %w", err)
	}

	data := cmd.Val()
	if data == "" {
		r.logger.Warn("Gamestate not found", "uuid", id)
		return nil, nil
	}

	var gs state.GameState
	if err := json.Unmarshal([]byte(data), &gs); err != nil {
		r.logger.Error("Failed to unmarshal gamestate", "uuid", id, "error", err)
		return nil, fmt.Errorf("failed to unmarshal gamestate: %w", err)
	}

	return &gs, nil
}

func (r *RedisStorage) DeleteGameState(ctx context.Context, id uuid.UUID) error {
	key := "gamestate:" + id.String()
	cmd := r.client.Del(ctx, key)
	if err := cmd.Err(); err != nil {
		r.logger.Error("Failed to delete gamestate", "uuid", id, "error", err)
		return fmt.Errorf("failed to delete gamestate: %w", err)
	}
	return nil
}

// Scenario operations (filesystem-backed)

func (r *RedisStorage) ListScenarios(ctx context.Context) (map[string]string, error) {
	scenariosDir := filepath.Join(r.dataDir, "scenarios")
	scenarios := make(map[string]string)

	err := filepath.WalkDir(scenariosDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".json" {
			return nil
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

	return scenarios, nil
}

func (r *RedisStorage) GetScenario(ctx context.Context, filename string) (*scenario.Scenario, error) {
	path := filepath.Join(r.dataDir, "scenarios", filename)

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

	return &s, nil
}

// Narrator operations (filesystem-backed)

func (r *RedisStorage) GetNarrator(ctx context.Context, narratorID string) (*scenario.Narrator, error) {
	if narratorID == "" {
		return nil, nil // No narrator specified
	}

	narratorPath := filepath.Join(r.dataDir, "narrators", narratorID+".json")

	data, err := os.ReadFile(narratorPath)
	if err != nil {
		if os.IsNotExist(err) {
			absPath, _ := filepath.Abs(narratorPath)
			cwd, _ := os.Getwd()
			return nil, fmt.Errorf("narrator not found: %s (tried: %s, cwd: %s)", narratorID, absPath, cwd)
		}
		return nil, fmt.Errorf("failed to read narrator file %s: %w", narratorPath, err)
	}

	var narrator scenario.Narrator
	if err := json.Unmarshal(data, &narrator); err != nil {
		return nil, fmt.Errorf("failed to parse narrator JSON from %s: %w", narratorPath, err)
	}
	narrator.ID = narratorID // Ensure ID is set from filename

	return &narrator, nil
}

func (r *RedisStorage) ListNarrators(ctx context.Context) ([]string, error) {
	narratorsPath := filepath.Join(r.dataDir, "narrators")

	entries, err := os.ReadDir(narratorsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read narrators directory: %w", err)
	}

	var narratorIDs []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			narratorID := entry.Name()[:len(entry.Name())-5] // Remove .json extension
			narratorIDs = append(narratorIDs, narratorID)
		}
	}

	return narratorIDs, nil
}

// PC operations (filesystem-backed, returns PCSpec only)

func (r *RedisStorage) GetPCSpec(ctx context.Context, path string) (*actor.PCSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PC file: %w", err)
	}

	var spec actor.PCSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to unmarshal PC spec: %w", err)
	}

	// Filename overrides any ID in the JSON
	spec.ID = strings.TrimSuffix(filepath.Base(path), ".json")

	return &spec, nil
}

func (r *RedisStorage) ListPCs(ctx context.Context) ([]string, error) {
	pcsPath := filepath.Join(r.dataDir, "pcs")

	entries, err := os.ReadDir(pcsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read PCs directory: %w", err)
	}

	var pcIDs []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			pcID := entry.Name()[:len(entry.Name())-5] // Remove .json extension
			pcIDs = append(pcIDs, pcID)
		}
	}

	return pcIDs, nil
}
