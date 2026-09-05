package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jwebster45206/story-engine/internal/config"
	"github.com/jwebster45206/story-engine/internal/llm"
	"github.com/jwebster45206/story-engine/internal/logger"
	"github.com/jwebster45206/story-engine/internal/queue"
	"github.com/jwebster45206/story-engine/internal/storage"
	"github.com/jwebster45206/story-engine/internal/worker"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	log := logger.Setup(cfg)

	log.Info("Starting Story Engine Worker",
		"config", os.Getenv("CONFIG"),
		"environment", cfg.Environment,
		"redis_url", cfg.RedisURL,
		"default_provider", cfg.DefaultProvider,
		"providers", len(cfg.Providers))

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

	// Initialize storage service
	storageService := storage.NewRedisStorage(cfg.RedisURL, "./data", log)
	storageCtx, storageCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer storageCancel()

	if err := storageService.Ping(storageCtx); err != nil {
		log.Error("Failed to connect to storage", "error", err)
		os.Exit(1)
	}
	log.Info("Storage service initialized successfully")

	registry, err := llm.NewRegistry(cfg, log)
	if err != nil {
		log.Error("Failed to initialize LLM providers", "error", err)
		os.Exit(1)
	}
	log.Info("LLM providers initialized successfully", "default", registry.Default(), "count", len(registry.Names()))

	// Create ChatProcessor
	processor := worker.NewChatProcessor(storageService, registry, chatQueue, log, cfg.ChatHistoryLimit)
	log.Info("Chat processor initialized successfully")

	// Create a separate Redis client for worker locking
	// (separate from queue client to avoid connection conflicts)
	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.RedisURL,
	})

	// Test Redis connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Error("Failed to close Redis client", "error", err)
		}
	}()

	log.Info("Redis connection established successfully")

	// Create and start worker with processor
	w := worker.New(chatQueue, processor, redisClient, log, os.Getenv("WORKER_ID"))

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
