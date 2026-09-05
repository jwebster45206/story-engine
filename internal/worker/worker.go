package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/internal/events"
	"github.com/jwebster45206/story-engine/internal/llm"
	"github.com/jwebster45206/story-engine/internal/queue"
	"github.com/jwebster45206/story-engine/pkg/chat"
	queuePkg "github.com/jwebster45206/story-engine/pkg/queue"
	"github.com/redis/go-redis/v9"
)

const (
	workerTimeout = 5 * time.Second

	// llmRequestTimeout is one DeltaUpdate attempt.
	llmRequestTimeout = llm.HTTPClientTimeout

	// gameLockTTL is the Redis lock crash ceiling.
	gameLockTTL = llmRequestTimeout + 15*time.Second
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

	result, err := w.redisClient.SetArgs(w.ctx, lockKey, w.id, redis.SetArgs{
		TTL:  gameLockTTL,
		Mode: "NX",
	}).Result()
	if err != nil {
		if err == redis.Nil {
			return false, nil // Key already exists — lock held by another worker
		}
		return false, err
	}

	return result == "OK", nil
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

	if err := script.Run(context.Background(), w.redisClient, []string{lockKey}, w.id).Err(); err != nil {
		w.log.Error("Failed to release game lock", "error", err, "game_state_id", gameStateID.String())
	}
}

func (w *Worker) publishFailed(gameStateID uuid.UUID, requestID, errMsg string) {
	if pubErr := w.broadcaster.PublishRequestFailed(context.Background(), gameStateID, requestID, errMsg); pubErr != nil {
		w.log.Error("Failed to publish failure event", "error", pubErr)
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

	gs, err := w.processor.GetGameState(w.ctx, req.GameStateID)
	if err != nil {
		w.log.Error("Failed to load game state",
			"error", err,
			"request_id", req.RequestID,
		)
		w.publishFailed(req.GameStateID, req.RequestID, err.Error())
		return fmt.Errorf("failed to load game state: %w", err)
	}

	var userMessage string
	switch req.Type {
	case queuePkg.RequestTypeChat:
		// Format message with PC name prefix if available
		userMessage = req.Message
		if gs.PC != nil && gs.PC.Spec != nil && gs.PC.Spec.Name != "" {
			userMessage = chat.FormatWithPCName(req.Message, gs.PC.Spec.Name)
		}
	case queuePkg.RequestTypeStoryEvent:
		userMessage = req.EventPrompt
	default:
		userMessage = ""
	}

	// Publish processing event with formatted user message
	if err := w.broadcaster.PublishRequestProcessing(w.ctx, req.GameStateID, req.RequestID, string(req.Type), userMessage); err != nil {
		w.log.Error("Failed to publish processing event", "error", err)
		// Don't fail the request just because event publishing failed
	}

	switch req.Type {
	case queuePkg.RequestTypeChat:
		chatReq := chat.ChatRequest{
			GameStateID: req.GameStateID,
			Message:     userMessage,
		}

		fullMessage, err := w.consumeStream(chatReq, req, "failed to process chat request")
		if err != nil {
			return err
		}

		if err := w.processor.UpdateGameStateAfterStream(context.Background(), gs, userMessage, fullMessage, false); err != nil {
			w.log.Error("Failed to update game state after stream",
				"error", err,
				"request_id", req.RequestID,
			)
			w.publishFailed(req.GameStateID, req.RequestID, err.Error())
			return fmt.Errorf("failed to update game state: %w", err)
		}

		w.log.Info("Chat request processed successfully",
			"worker_id", w.id,
			"request_id", req.RequestID,
			"duration_ms", time.Since(start).Milliseconds(),
		)

		result := map[string]interface{}{
			"message":     fullMessage,
			"duration_ms": time.Since(start).Milliseconds(),
		}
		if err := w.broadcaster.PublishRequestCompleted(w.ctx, req.GameStateID, req.RequestID, result); err != nil {
			w.log.Error("Failed to publish completion event", "error", err)
		}

	case queuePkg.RequestTypeStoryEvent:
		storyEventMessage := req.EventPrompt
		chatReq := chat.ChatRequest{
			GameStateID: req.GameStateID,
			Message:     storyEventMessage,
		}

		fullMessage, err := w.consumeStream(chatReq, req, "failed to process story event")
		if err != nil {
			return err
		}

		gs, err := w.processor.GetGameState(w.ctx, req.GameStateID)
		if err != nil {
			w.log.Error("Failed to load game state for update",
				"error", err,
				"request_id", req.RequestID,
			)
			w.publishFailed(req.GameStateID, req.RequestID, err.Error())
			return fmt.Errorf("failed to load game state: %w", err)
		}

		if err := w.processor.UpdateGameStateAfterStream(context.Background(), gs, storyEventMessage, fullMessage, true); err != nil {
			w.log.Error("Failed to update game state after stream",
				"error", err,
				"request_id", req.RequestID,
			)
			w.publishFailed(req.GameStateID, req.RequestID, err.Error())
			return fmt.Errorf("failed to update game state: %w", err)
		}

		w.log.Info("Story event processed successfully",
			"worker_id", w.id,
			"request_id", req.RequestID,
			"duration_ms", time.Since(start).Milliseconds(),
		)

		result := map[string]interface{}{
			"message":     fullMessage,
			"duration_ms": time.Since(start).Milliseconds(),
		}
		if err := w.broadcaster.PublishRequestCompleted(w.ctx, req.GameStateID, req.RequestID, result); err != nil {
			w.log.Error("Failed to publish completion event", "error", err)
		}

	default:
		return fmt.Errorf("unknown request type: %s", req.Type)
	}

	return nil
}

func (w *Worker) consumeStream(chatReq chat.ChatRequest, req *queuePkg.Request, errWrap string) (string, error) {
	streamChan, err := w.processor.ProcessChatStream(w.ctx, chatReq)
	if err != nil {
		w.log.Error("Failed to start stream",
			"error", err,
			"request_id", req.RequestID,
			"game_state_id", req.GameStateID.String(),
			"type", req.Type,
		)
		w.publishFailed(req.GameStateID, req.RequestID, err.Error())
		return "", fmt.Errorf("%s: %w", errWrap, err)
	}

	var fullMessage string
	var streamErr error
	var done bool
	for chunk := range streamChan {
		if chunk.Error != nil {
			streamErr = chunk.Error
			w.log.Error("Error in stream",
				"error", chunk.Error,
				"request_id", req.RequestID,
				"type", req.Type,
			)
			break
		}

		fullMessage += chunk.Content
		if err := w.broadcaster.PublishChatChunk(w.ctx, req.GameStateID, req.RequestID, chunk.Content, chunk.Done); err != nil {
			w.log.Error("Failed to publish chat chunk", "error", err)
		}

		if chunk.Done {
			done = true
			break
		}
	}

	if streamErr == nil && !done && w.ctx.Err() != nil {
		streamErr = w.ctx.Err()
	}
	if streamErr != nil {
		w.publishFailed(req.GameStateID, req.RequestID, streamErr.Error())
		return "", fmt.Errorf("%s: %w", errWrap, streamErr)
	}
	return fullMessage, nil
}
