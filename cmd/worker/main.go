package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jwebster45206/story-engine/internal/config"
	"github.com/jwebster45206/story-engine/internal/logger"
	"github.com/jwebster45206/story-engine/internal/services/queue"
	"github.com/jwebster45206/story-engine/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	log := logger.Setup(cfg)

	log.Info("Starting Story Engine Worker",
		"environment", cfg.Environment,
		"redis_url", cfg.RedisURL)

	// Initialize queue service
	queueClient, err := queue.NewClient(cfg.RedisURL, log)
	if err != nil {
		log.Error("Failed to create queue client", "error", err)
		os.Exit(1)
	}
	defer func() {
		err = queueClient.Close()
		if err != nil {
			log.Error("Error closing queue client", "error", err)
		}
	}()

	chatQueue := queue.NewChatQueue(queueClient)
	log.Info("Queue service initialized successfully")

	// Create a separate Redis client for worker locking
	// (separate from queue client to avoid connection conflicts)
	redisOpts, err := redis.ParseURL("redis://" + cfg.RedisURL)
	if err != nil {
		log.Error("Failed to parse Redis URL", "error", err)
		os.Exit(1)
	}
	redisClient := redis.NewClient(redisOpts)

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	log.Info("Redis connection established successfully")

	// Create and start worker
	w := worker.New(chatQueue, redisClient, log, os.Getenv("WORKER_ID"))

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start worker in goroutine
	go func() {
		if err := w.Start(); err != nil {
			log.Error("Worker error", "error", err)
			os.Exit(1)
		}
	}()

	log.Info("Worker started, waiting for requests...")

	// Wait for shutdown signal
	<-quit
	log.Info("Worker shutdown signal received")

	// Stop worker
	w.Stop()

	// Give worker time to finish current request
	time.Sleep(2 * time.Second)

	log.Info("Worker exited")
}
