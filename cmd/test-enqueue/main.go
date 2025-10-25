package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/pkg/queue"
)

func main() {
	// Connect to Redis
	redisOpts, err := redis.ParseURL("redis://localhost:6379")
	if err != nil {
		log.Fatal("Failed to parse Redis URL:", err)
	}
	client := redis.NewClient(redisOpts)
	defer client.Close()

	ctx := context.Background()

	// Test connection
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatal("Failed to connect to Redis:", err)
	}

	fmt.Println("Connected to Redis successfully!")

	// Create a test chat request
	chatReq := &queue.Request{
		RequestID:   uuid.New().String(),
		Type:        queue.RequestTypeChat,
		GameStateID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Test game state ID
		Message:     "Hello, this is a test message!",
		Actor:       "test-player",
		EnqueuedAt:  time.Now(),
	}

	// Enqueue the chat request
	data, err := json.Marshal(chatReq)
	if err != nil {
		log.Fatal("Failed to marshal request:", err)
	}

	if err := client.RPush(ctx, "requests", data).Err(); err != nil {
		log.Fatal("Failed to enqueue request:", err)
	}

	fmt.Printf("✅ Enqueued chat request: %s\n", chatReq.RequestID)

	// Create a test story event request
	storyReq := &queue.Request{
		RequestID:   uuid.New().String(),
		Type:        queue.RequestTypeStoryEvent,
		GameStateID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), // Same game state ID
		EventPrompt: "A mysterious figure appears in the shadows.",
		EnqueuedAt:  time.Now(),
	}

	// Enqueue the story event request
	data, err = json.Marshal(storyReq)
	if err != nil {
		log.Fatal("Failed to marshal request:", err)
	}

	if err := client.RPush(ctx, "requests", data).Err(); err != nil {
		log.Fatal("Failed to enqueue request:", err)
	}

	fmt.Printf("✅ Enqueued story event request: %s\n", storyReq.RequestID)

	// Check queue depth
	depth, err := client.LLen(ctx, "requests").Result()
	if err != nil {
		log.Fatal("Failed to get queue depth:", err)
	}

	fmt.Printf("\n📊 Queue depth: %d requests\n", depth)
	fmt.Println("\n💡 Now start the worker to see it process these requests!")
	fmt.Println("   Run: go run cmd/worker/main.go")
}
