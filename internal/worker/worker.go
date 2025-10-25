package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/internal/services/events"
	"github.com/jwebster45206/story-engine/internal/services/queue"
	"github.com/jwebster45206/story-engine/pkg/chat"
	queuePkg "github.com/jwebster45206/story-engine/pkg/queue"
)

const (
	workerTimeout = 5 * time.Second
)

// Worker processes messages in the chat queue
type Worker struct {
	id          string
	queue       *queue.ChatQueue
	processor   *ChatProcessor
	broadcaster *events.Broadcaster
	redisClient *redis.Client
	log         *slog.Logger
	ctx         context.Context
	cancel      context.CancelFunc
}

// New creates a new worker instance
func New(queueClient *queue.ChatQueue, processor *ChatProcessor, redisClient *redis.Client, log *slog.Logger, workerID string) *Worker {
	ctx, cancel := context.WithCancel(context.Background())

	if workerID == "" {
		workerID = fmt.Sprintf("worker-%s", uuid.New().String()[:8])
	}

	broadcaster := events.NewBroadcaster(redisClient, log)

	return &Worker{
		id:          workerID,
		queue:       queueClient,
		processor:   processor,
		broadcaster: broadcaster,
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
	ctx, cancel := context.WithTimeout(w.ctx, workerTimeout)
	defer cancel()

	req, err := w.queue.BlockingDequeueRequest(ctx, workerTimeout)
	if err != nil {
		// Real error (not timeout/cancellation)
		return fmt.Errorf("failed to dequeue request: %w", err)
	}

	if req == nil {
		// Queue is empty or timeout occurred - this is normal
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
		// Another worker is processing this gamestate
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

	// Process the request, blocking the worker until done
	defer w.releaseGameLock(req.GameStateID)
	return w.processRequest(req)
}

// acquireGameLock attempts to acquire a lock for a game
// Returns true if lock was acquired, false if already locked
func (w *Worker) acquireGameLock(gameStateID uuid.UUID) (bool, error) {
	lockKey := fmt.Sprintf("game-lock:%s", gameStateID.String())

	result, err := w.redisClient.SetNX(w.ctx, lockKey, w.id, 30*time.Second).Result()
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

// processRequest processes a single request using the ChatProcessor
func (w *Worker) processRequest(req *queuePkg.Request) error {
	w.log.Info("Processing request",
		"worker_id", w.id,
		"request_id", req.RequestID,
		"type", req.Type,
		"game_state_id", req.GameStateID.String(),
	)

	start := time.Now()

	// Publish processing event
	if err := w.broadcaster.PublishRequestProcessing(w.ctx, req.GameStateID, req.RequestID, string(req.Type)); err != nil {
		w.log.Error("Failed to publish processing event", "error", err)
		// Don't fail the request just because event publishing failed
	}

	switch req.Type {
	case queuePkg.RequestTypeChat:
		// Convert queue request to chat request
		chatReq := chat.ChatRequest{
			GameStateID: req.GameStateID,
			Message:     req.Message,
		}

		// Process using ChatProcessor
		response, err := w.processor.ProcessChatRequest(w.ctx, chatReq)
		if err != nil {
			w.log.Error("Failed to process chat request",
				"error", err,
				"request_id", req.RequestID,
				"game_state_id", req.GameStateID.String(),
			)

			// Publish failure event
			if pubErr := w.broadcaster.PublishRequestFailed(w.ctx, req.GameStateID, req.RequestID, err.Error()); pubErr != nil {
				w.log.Error("Failed to publish failure event", "error", pubErr)
			}

			return fmt.Errorf("failed to process chat request: %w", err)
		}

		w.log.Info("Chat request processed successfully",
			"worker_id", w.id,
			"request_id", req.RequestID,
			"duration_ms", time.Since(start).Milliseconds(),
		)

		// Publish completion event
		result := map[string]interface{}{
			"message": response.Message,
		}
		if err := w.broadcaster.PublishRequestCompleted(w.ctx, req.GameStateID, req.RequestID, result); err != nil {
			w.log.Error("Failed to publish completion event", "error", err)
		}

	case queuePkg.RequestTypeStoryEvent:
		w.log.Info("Story event processing not yet implemented",
			"request_id", req.RequestID,
			"game_state_id", req.GameStateID.String(),
			"event_prompt", req.EventPrompt,
		)
		// TODO: Implement story event processing when needed

	default:
		return fmt.Errorf("unknown request type: %s", req.Type)
	}

	return nil
}
