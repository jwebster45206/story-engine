package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/state"
)

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
