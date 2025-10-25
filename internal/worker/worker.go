package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/internal/services/queue"
	queuePkg "github.com/jwebster45206/story-engine/pkg/queue"
)

// Worker processes requests from the unified queue
type Worker struct {
	id          string
	queue       *queue.ChatQueue
	redisClient *redis.Client
	log         *slog.Logger
	ctx         context.Context
	cancel      context.CancelFunc
}

// Config holds worker configuration
type Config struct {
	WorkerID           string
	ConcurrentWorkers  int
	PollIntervalMs     int
	GameLockTTLSeconds int
}

// New creates a new worker instance
func New(queueClient *queue.ChatQueue, redisClient *redis.Client, log *slog.Logger, cfg *Config) *Worker {
	ctx, cancel := context.WithCancel(context.Background())

	workerID := cfg.WorkerID
	if workerID == "" {
		workerID = fmt.Sprintf("worker-%s", uuid.New().String()[:8])
	}

	return &Worker{
		id:          workerID,
		queue:       queueClient,
		redisClient: redisClient,
		log:         log,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start begins processing requests from the queue
func (w *Worker) Start() error {
	w.log.Info("Worker starting", "worker_id", w.id)

	for {
		select {
		case <-w.ctx.Done():
			w.log.Info("Worker shutting down", "worker_id", w.id)
			return nil
		default:
			if err := w.processNextRequest(); err != nil {
				w.log.Error("Error processing request", "error", err, "worker_id", w.id)
				// Continue processing even on error
				time.Sleep(1 * time.Second)
			}
		}
	}
}

// Stop gracefully shuts down the worker
func (w *Worker) Stop() {
	w.log.Info("Worker stop requested", "worker_id", w.id)
	w.cancel()
}

// processNextRequest pulls the next request from the queue and processes it
func (w *Worker) processNextRequest() error {
	// Block waiting for next request (timeout after 5 seconds to check for shutdown)
	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
	defer cancel()

	req, err := w.queue.BlockingDequeueRequest(ctx, 5)
	if err != nil {
		// Check if it's a timeout or context cancellation (normal)
		if err == context.DeadlineExceeded || err == context.Canceled {
			return nil
		}
		return fmt.Errorf("failed to dequeue request: %w", err)
	}

	if req == nil {
		// No request available (shouldn't happen with blocking dequeue)
		return nil
	}

	w.log.Info("Received request from queue",
		"worker_id", w.id,
		"request_id", req.RequestID,
		"type", req.Type,
		"game_state_id", req.GameStateID.String(),
	)

	// Try to acquire game lock
	locked, err := w.acquireGameLock(req.GameStateID)
	if err != nil {
		return fmt.Errorf("failed to acquire game lock: %w", err)
	}

	if !locked {
		// Another worker is processing this game
		// Re-queue at the end and try next request
		w.log.Info("Game already locked, re-queueing request",
			"worker_id", w.id,
			"request_id", req.RequestID,
			"game_state_id", req.GameStateID.String(),
		)

		if err := w.queue.EnqueueRequest(w.ctx, req); err != nil {
			return fmt.Errorf("failed to re-queue request: %w", err)
		}
		return nil
	}

	// Process the request
	defer w.releaseGameLock(req.GameStateID)

	return w.processRequest(req)
}

// acquireGameLock attempts to acquire a lock for a game
// Returns true if lock was acquired, false if already locked
func (w *Worker) acquireGameLock(gameStateID uuid.UUID) (bool, error) {
	lockKey := fmt.Sprintf("game-lock:%s", gameStateID.String())

	// Try to set the lock with 5 minute TTL
	result, err := w.redisClient.SetNX(w.ctx, lockKey, w.id, 5*time.Minute).Result()
	if err != nil {
		return false, err
	}

	return result, nil
}

// releaseGameLock releases the lock for a game
func (w *Worker) releaseGameLock(gameStateID uuid.UUID) {
	lockKey := fmt.Sprintf("game-lock:%s", gameStateID.String())

	// Only delete if we own the lock
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`)

	if err := script.Run(w.ctx, w.redisClient, []string{lockKey}, w.id).Err(); err != nil {
		w.log.Error("Failed to release game lock", "error", err, "game_state_id", gameStateID.String())
	}
}

// processRequest processes a single request (skeleton implementation)
func (w *Worker) processRequest(req *queuePkg.Request) error {
	w.log.Info("Processing request (SKELETON - no actual processing yet)",
		"worker_id", w.id,
		"request_id", req.RequestID,
		"type", req.Type,
		"game_state_id", req.GameStateID.String(),
		"message", req.Message,
		"actor", req.Actor,
		"event_prompt", req.EventPrompt,
		"enqueued_at", req.EnqueuedAt.Format(time.RFC3339),
	)

	// Simulate some processing time
	time.Sleep(500 * time.Millisecond)

	switch req.Type {
	case queuePkg.RequestTypeChat:
		w.log.Info("Would process chat request",
			"request_id", req.RequestID,
			"game_state_id", req.GameStateID.String(),
			"message", req.Message,
			"actor", req.Actor,
		)
	case queuePkg.RequestTypeStoryEvent:
		w.log.Info("Would process story event",
			"request_id", req.RequestID,
			"game_state_id", req.GameStateID.String(),
			"event_prompt", req.EventPrompt,
		)
	default:
		return fmt.Errorf("unknown request type: %s", req.Type)
	}

	w.log.Info("Request processing complete (SKELETON)",
		"worker_id", w.id,
		"request_id", req.RequestID,
		"duration_ms", time.Since(req.EnqueuedAt).Milliseconds(),
	)

	return nil
}
