package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/redis/go-redis/v9"
)

const gameStateTTL = time.Hour

func gamestateKey(id uuid.UUID) string {
	return "gamestate:" + id.String()
}

func gamestateOwnerKey(id uuid.UUID) string {
	return "gamestate-owner:" + id.String()
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

	pipe := r.client.TxPipeline()
	pipe.Set(ctx, gamestateKey(id), string(data), gameStateTTL)
	pipe.Set(ctx, gamestateOwnerKey(id), gs.OwnerKeyHash, gameStateTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		r.logger.Error("Failed to save gamestate", "uuid", id, "error", err)
		return fmt.Errorf("failed to save gamestate: %w", err)
	}

	return nil
}

func (r *RedisStorage) LoadGameState(ctx context.Context, id uuid.UUID) (*state.GameState, error) {
	cmd := r.client.Get(ctx, gamestateKey(id))
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

func (r *RedisStorage) GetOwnerKeyHash(ctx context.Context, id uuid.UUID) (string, bool, error) {
	cmd := r.client.Get(ctx, gamestateOwnerKey(id))
	if err := cmd.Err(); err != nil {
		if errors.Is(err, redis.Nil) {
			return "", false, nil
		}
		r.logger.Error("Failed to load gamestate owner hash", "uuid", id, "error", err)
		return "", false, fmt.Errorf("failed to load gamestate owner hash: %w", err)
	}
	return cmd.Val(), true, nil
}

func (r *RedisStorage) DeleteGameState(ctx context.Context, id uuid.UUID) error {
	cmd := r.client.Del(ctx, gamestateKey(id), gamestateOwnerKey(id))
	if err := cmd.Err(); err != nil {
		r.logger.Error("Failed to delete gamestate", "uuid", id, "error", err)
		return fmt.Errorf("failed to delete gamestate: %w", err)
	}
	return nil
}
