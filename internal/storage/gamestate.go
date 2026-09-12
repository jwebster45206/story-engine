package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/storage"
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

func (r *RedisStorage) CreateGameState(ctx context.Context, id uuid.UUID, gs *state.GameState) error {
	if id == uuid.Nil() || gs == nil || gs.PrincipalID == uuid.Nil() {
		return fmt.Errorf("id and principal must not be empty")
	}
	gs.UpdatedAt = time.Now()
	data, err := json.Marshal(gs)
	if err != nil {
		return fmt.Errorf("failed to marshal gamestate: %w", err)
	}
	pipe := r.client.TxPipeline()
	pipe.Set(ctx, gamestateKey(id), string(data), gameStateTTL)
	pipe.Set(ctx, gamestateOwnerKey(id), gs.PrincipalID.String(), gameStateTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("failed to create gamestate: %w", err)
	}
	return nil
}

func (r *RedisStorage) UpdateGameState(ctx context.Context, id uuid.UUID, gs *state.GameState) error {
	gs.UpdatedAt = time.Now()

	data, err := json.Marshal(gs)
	if err != nil {
		r.logger.Error("Failed to marshal gamestate", "uuid", id, "error", err)
		return fmt.Errorf("failed to marshal gamestate: %w", err)
	}

	pipe := r.client.TxPipeline()
	pipe.Set(ctx, gamestateKey(id), string(data), gameStateTTL)
	pipe.Expire(ctx, gamestateOwnerKey(id), gameStateTTL)
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
			return nil, nil
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

func (r *RedisStorage) GetOwner(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	cmd := r.client.Get(ctx, gamestateOwnerKey(id))
	if err := cmd.Err(); err != nil {
		if errors.Is(err, redis.Nil) {
			return uuid.Nil(), storage.ErrNotFound
		}
		return uuid.Nil(), fmt.Errorf("failed to load gamestate owner: %w", err)
	}
	ownerID, err := uuid.Parse(cmd.Val())
	if err != nil {
		return uuid.Nil(), fmt.Errorf("gamestate owner is not a UUID: %w", err)
	}
	return ownerID, nil
}

func (r *RedisStorage) DeleteGameState(ctx context.Context, id uuid.UUID) error {
	cmd := r.client.Del(ctx, gamestateKey(id), gamestateOwnerKey(id))
	if err := cmd.Err(); err != nil {
		r.logger.Error("Failed to delete gamestate", "uuid", id, "error", err)
		return fmt.Errorf("failed to delete gamestate: %w", err)
	}
	return nil
}
