package queue

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	queuePkg "github.com/jwebster45206/story-engine/pkg/queue"
)

func setupTestRedis(t *testing.T) (*Client, *miniredis.Miniredis) {
	t.Helper()

	// Start miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}

	// Create queue client
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	redisURL := mr.Addr() // Just the address, not redis:// URL

	client, err := NewClient(redisURL, logger)
	if err != nil {
		mr.Close()
		t.Fatalf("Failed to create queue client: %v", err)
	}

	return client, mr
}

func TestChatQueue_EnqueueAndDequeueRequest(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer func() {
		_ = client.Close()
	}()

	seq := NewChatQueue(client)

	ctx := context.Background()
	gameStateID := uuid.New()

	// Enqueue some requests
	requests := []*queuePkg.Request{
		{
			RequestID:   uuid.New().String(),
			Type:        queuePkg.RequestTypeStoryEvent,
			GameStateID: gameStateID,
			EventPrompt: "A dragon appears on the horizon",
			EnqueuedAt:  time.Now(),
		},
		{
			RequestID:   uuid.New().String(),
			Type:        queuePkg.RequestTypeChat,
			GameStateID: gameStateID,
			Message:     "Hello, world!",
			Actor:       "player",
			EnqueuedAt:  time.Now(),
		},
	}

	for _, req := range requests {
		err := seq.EnqueueRequest(ctx, req)
		if err != nil {
			t.Fatalf("Failed to enqueue request: %v", err)
		}
	}

	// Check depth
	depth, err := seq.RequestQueueDepth(ctx)
	if err != nil {
		t.Fatalf("Failed to get depth: %v", err)
	}
	if depth != len(requests) {
		t.Errorf("Expected depth %d, got %d", len(requests), depth)
	}

	// Dequeue and verify
	for i, expected := range requests {
		dequeued, err := seq.DequeueRequest(ctx)
		if err != nil {
			t.Fatalf("Failed to dequeue request %d: %v", i, err)
		}

		if dequeued.RequestID != expected.RequestID {
			t.Errorf("Request %d ID mismatch: expected %q, got %q", i, expected.RequestID, dequeued.RequestID)
		}
		if dequeued.Type != expected.Type {
			t.Errorf("Request %d type mismatch: expected %q, got %q", i, expected.Type, dequeued.Type)
		}
		if dequeued.GameStateID != expected.GameStateID {
			t.Errorf("Request %d GameStateID mismatch: expected %q, got %q", i, expected.GameStateID, dequeued.GameStateID)
		}
	}

	// Queue should be empty after dequeue
	depth, err = seq.RequestQueueDepth(ctx)
	if err != nil {
		t.Fatalf("Failed to get depth after dequeue: %v", err)
	}
	if depth != 0 {
		t.Errorf("Expected empty queue, got depth %d", depth)
	}
}

func TestChatQueue_DequeueEmptyQueue(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer func() {
		_ = client.Close()
	}()

	seq := NewChatQueue(client)
	ctx := context.Background()

	// Dequeue from empty queue should return nil
	req, err := seq.DequeueRequest(ctx)
	if err != nil {
		t.Fatalf("Unexpected error from empty queue: %v", err)
	}
	if req != nil {
		t.Errorf("Expected nil from empty queue, got %v", req)
	}
}

func TestChatQueue_RequestQueueDepth(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer func() {
		_ = client.Close()
	}()

	seq := NewChatQueue(client)
	ctx := context.Background()

	// Empty queue
	depth, err := seq.RequestQueueDepth(ctx)
	if err != nil {
		t.Fatalf("Failed to get depth: %v", err)
	}
	if depth != 0 {
		t.Errorf("Expected empty queue, got depth %d", depth)
	}

	// Add requests
	gameStateID := uuid.New()
	for i := 0; i < 3; i++ {
		req := &queuePkg.Request{
			RequestID:   uuid.New().String(),
			Type:        queuePkg.RequestTypeStoryEvent,
			GameStateID: gameStateID,
			EventPrompt: "Test event",
			EnqueuedAt:  time.Now(),
		}
		_ = seq.EnqueueRequest(ctx, req)
	}

	depth, err = seq.RequestQueueDepth(ctx)
	if err != nil {
		t.Fatalf("Failed to get depth: %v", err)
	}
	if depth != 3 {
		t.Errorf("Expected depth 3, got %d", depth)
	}
}

func TestChatQueue_FIFOOrdering(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer func() {
		_ = client.Close()
	}()

	seq := NewChatQueue(client)
	ctx := context.Background()
	gameStateID := uuid.New()

	// Enqueue requests in order
	requestIDs := []string{}
	for i := 0; i < 5; i++ {
		reqID := uuid.New().String()
		requestIDs = append(requestIDs, reqID)
		req := &queuePkg.Request{
			RequestID:   reqID,
			Type:        queuePkg.RequestTypeStoryEvent,
			GameStateID: gameStateID,
			EventPrompt: "Event " + reqID,
			EnqueuedAt:  time.Now(),
		}
		_ = seq.EnqueueRequest(ctx, req)
	}

	// Dequeue and verify FIFO order
	for i, expectedID := range requestIDs {
		dequeued, err := seq.DequeueRequest(ctx)
		if err != nil {
			t.Fatalf("Failed to dequeue request %d: %v", i, err)
		}
		if dequeued.RequestID != expectedID {
			t.Errorf("FIFO violation at position %d: expected %q, got %q", i, expectedID, dequeued.RequestID)
		}
	}
}

func TestChatQueue_GetFormattedEvents_LegacySupport(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer func() {
		_ = client.Close()
	}()

	seq := NewChatQueue(client)
	ctx := context.Background()
	gameStateID := uuid.New()

	// Test empty queue
	formatted, err := seq.GetFormattedEvents(ctx, gameStateID)
	if err != nil {
		t.Fatalf("Failed to get formatted events: %v", err)
	}
	if formatted != "" {
		t.Errorf("Expected empty string for empty queue, got %q", formatted)
	}
}
