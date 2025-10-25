package queue

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
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
	redisURL := "redis://" + mr.Addr()

	client, err := NewClient(redisURL, logger)
	if err != nil {
		mr.Close()
		t.Fatalf("Failed to create queue client: %v", err)
	}

	return client, mr
}

func TestChatQueue_EnqueueAndDequeue(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	seq := NewChatQueue(client, logger)

	ctx := context.Background()
	gameStateID := uuid.New()

	// Enqueue some events
	events := []string{
		"A dragon appears on the horizon",
		"The ground trembles beneath your feet",
		"A mysterious stranger approaches",
	}

	for _, event := range events {
		err := seq.Enqueue(ctx, gameStateID, event)
		if err != nil {
			t.Fatalf("Failed to enqueue event: %v", err)
		}
	}

	// Check depth
	depth, err := seq.Depth(ctx, gameStateID)
	if err != nil {
		t.Fatalf("Failed to get depth: %v", err)
	}
	if depth != len(events) {
		t.Errorf("Expected depth %d, got %d", len(events), depth)
	}

	// Dequeue and verify
	dequeued, err := seq.Dequeue(ctx, gameStateID)
	if err != nil {
		t.Fatalf("Failed to dequeue events: %v", err)
	}

	if len(dequeued) != len(events) {
		t.Errorf("Expected %d events, got %d", len(events), len(dequeued))
	}

	for i, event := range events {
		if dequeued[i] != event {
			t.Errorf("Event %d mismatch: expected %q, got %q", i, event, dequeued[i])
		}
	}

	// Queue should be empty after dequeue
	depth, err = seq.Depth(ctx, gameStateID)
	if err != nil {
		t.Fatalf("Failed to get depth after dequeue: %v", err)
	}
	if depth != 0 {
		t.Errorf("Expected empty queue, got depth %d", depth)
	}
}

func TestChatQueue_Peek(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	seq := NewChatQueue(client, logger)

	ctx := context.Background()
	gameStateID := uuid.New()

	// Enqueue events
	events := []string{"Event 1", "Event 2", "Event 3"}
	for _, event := range events {
		seq.Enqueue(ctx, gameStateID, event)
	}

	// Peek all
	peeked, err := seq.Peek(ctx, gameStateID, 0)
	if err != nil {
		t.Fatalf("Failed to peek: %v", err)
	}
	if len(peeked) != len(events) {
		t.Errorf("Expected %d events, got %d", len(events), len(peeked))
	}

	// Peek should not remove events
	depth, _ := seq.Depth(ctx, gameStateID)
	if depth != len(events) {
		t.Errorf("Peek removed events: expected depth %d, got %d", len(events), depth)
	}

	// Peek with limit
	peeked, err = seq.Peek(ctx, gameStateID, 2)
	if err != nil {
		t.Fatalf("Failed to peek with limit: %v", err)
	}
	if len(peeked) != 2 {
		t.Errorf("Expected 2 events, got %d", len(peeked))
	}
}

func TestChatQueue_Clear(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	seq := NewChatQueue(client, logger)

	ctx := context.Background()
	gameStateID := uuid.New()

	// Enqueue events
	seq.Enqueue(ctx, gameStateID, "Event 1")
	seq.Enqueue(ctx, gameStateID, "Event 2")

	// Clear
	err := seq.Clear(ctx, gameStateID)
	if err != nil {
		t.Fatalf("Failed to clear: %v", err)
	}

	// Verify empty
	depth, _ := seq.Depth(ctx, gameStateID)
	if depth != 0 {
		t.Errorf("Expected empty queue after clear, got depth %d", depth)
	}
}

func TestChatQueue_GetFormattedEvents(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	seq := NewChatQueue(client, logger)

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

	// Enqueue events
	seq.Enqueue(ctx, gameStateID, "Dragon appears")
	seq.Enqueue(ctx, gameStateID, "Ground trembles")

	formatted, err = seq.GetFormattedEvents(ctx, gameStateID)
	if err != nil {
		t.Fatalf("Failed to get formatted events: %v", err)
	}

	expected := "STORY EVENT: Dragon appears\n\nSTORY EVENT: Ground trembles"
	if formatted != expected {
		t.Errorf("Formatted events mismatch:\nExpected: %q\nGot: %q", expected, formatted)
	}
}

func TestChatQueue_MultipleGames(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	seq := NewChatQueue(client, logger)

	ctx := context.Background()
	game1 := uuid.New()
	game2 := uuid.New()

	// Enqueue events for different games
	seq.Enqueue(ctx, game1, "Game 1 Event 1")
	seq.Enqueue(ctx, game1, "Game 1 Event 2")
	seq.Enqueue(ctx, game2, "Game 2 Event 1")

	// Verify isolation
	depth1, _ := seq.Depth(ctx, game1)
	depth2, _ := seq.Depth(ctx, game2)

	if depth1 != 2 {
		t.Errorf("Game 1 expected depth 2, got %d", depth1)
	}
	if depth2 != 1 {
		t.Errorf("Game 2 expected depth 1, got %d", depth2)
	}

	// Dequeue from game1 shouldn't affect game2
	seq.Dequeue(ctx, game1)
	depth2After, _ := seq.Depth(ctx, game2)
	if depth2After != 1 {
		t.Errorf("Game 2 depth changed after dequeuing game 1: got %d", depth2After)
	}
}
