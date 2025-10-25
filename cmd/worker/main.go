package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/jwebster45206/story-engine/internal/config"
	"github.com/jwebster45206/story-engine/internal/logger"
	"github.com/jwebster45206/story-engine/internal/services"
	"github.com/jwebster45206/story-engine/internal/services/queue"
	"github.com/jwebster45206/story-engine/internal/storage"
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

	// Initialize storage service
	storageService := storage.NewRedisStorage(cfg.RedisURL, "./data", log)
	storageCtx, storageCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer storageCancel()

	if err := storageService.Ping(storageCtx); err != nil {
		log.Error("Failed to connect to storage", "error", err)
		os.Exit(1)
	}
	log.Info("Storage service initialized successfully")

	// Initialize LLM service
	var llmService services.LLMService
	switch strings.ToLower(cfg.LLMProvider) {
	case "anthropic":
		if cfg.AnthropicAPIKey == "" {
			log.Error("Anthropic API key is required when using anthropic provider")
			os.Exit(1)
		}
		llmService = services.NewAnthropicService(cfg.AnthropicAPIKey, cfg.ModelName, cfg.BackendModelName, log)
		log.Info("Using Anthropic LLM provider")
	case "venice":
		if cfg.VeniceAPIKey == "" {
			log.Error("Venice API key is required when using venice provider")
			os.Exit(1)
		}
		llmService = services.NewVeniceService(cfg.VeniceAPIKey, cfg.ModelName, cfg.BackendModelName)
		log.Info("Using Venice LLM provider")
	default:
		log.Error("Invalid LLM provider specified", "provider", cfg.LLMProvider, "supported", []string{"anthropic", "venice"})
		os.Exit(1)
	}

	// Initialize the model
	initCtx, initCancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer initCancel()
	if err := llmService.InitModel(initCtx, cfg.ModelName); err != nil {
		log.Error("Failed to initialize LLM model", "error", err, "model", cfg.ModelName)
		os.Exit(1)
	}
	log.Info("LLM service initialized successfully", "model", cfg.ModelName)

	// Create ChatProcessor
	processor := worker.NewChatProcessor(storageService, llmService, chatQueue, log)
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
