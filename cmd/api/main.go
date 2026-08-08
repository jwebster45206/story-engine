package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jwebster45206/story-engine/internal/config"
	"github.com/jwebster45206/story-engine/internal/handlers"
	"github.com/jwebster45206/story-engine/internal/logger"
	"github.com/jwebster45206/story-engine/internal/middleware"
	"github.com/jwebster45206/story-engine/internal/services"
	"github.com/jwebster45206/story-engine/internal/services/queue"
	"github.com/jwebster45206/story-engine/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	log := logger.Setup(cfg)

	log.Info("Starting Story Engine API",
		"config", os.Getenv("CONFIG"),
		"port", cfg.Port,
		"environment", cfg.Environment,
		"default_provider", cfg.DefaultProvider,
		"providers", len(cfg.Providers))

	registry, err := services.NewRegistry(cfg, log)
	if err != nil {
		log.Error("Failed to initialize LLM providers", "error", err)
		os.Exit(1)
	}

	storageService := storage.NewRedisStorage(cfg.RedisURL, "./data", log)
	storageCtx, storageCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer storageCancel()

	if err := storageService.Ping(storageCtx); err != nil {
		log.Error("Failed to connect to storage", "error", err)
		os.Exit(1)
	}
	log.Info("Storage connection established successfully")

	// Initialize queue service for story events
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

	// Create Redis client for SSE (reusing queue client's redis)
	redisClient := queueClient.GetRedisClient()

	mux := http.NewServeMux()

	healthHandler := handlers.NewHealthHandler(log, storageService)
	mux.Handle("/health", healthHandler)

	chatHandler := handlers.NewChatHandler(chatQueue, log)
	mux.Handle("/v1/chat", chatHandler)

	eventsHandler := handlers.NewEventsHandler(redisClient, log)
	mux.Handle("/v1/events/gamestate/", eventsHandler)

	gameStateHandler := handlers.NewGameStateHandler(log, registry, storageService)
	mux.Handle("/v1/gamestate", gameStateHandler)
	mux.Handle("/v1/gamestate/", gameStateHandler)

	providersHandler := handlers.NewProvidersHandler(log, registry)
	mux.Handle("/v1/providers", providersHandler)

	scenarioHandler := handlers.NewScenarioHandler(log, storageService)
	mux.Handle("/v1/scenarios", scenarioHandler)
	mux.Handle("/v1/scenarios/", scenarioHandler)

	pcHandler := handlers.NewPCHandler(log, storageService)
	mux.Handle("/v1/pcs", pcHandler)
	mux.Handle("/v1/pcs/", pcHandler)

	narratorHandler := handlers.NewNarratorHandler(log, storageService)
	mux.Handle("/v1/narrators", narratorHandler)
	mux.Handle("/v1/narrators/", narratorHandler)

	monsterHandler := handlers.NewMonsterHandler(log, storageService)
	mux.Handle("/v1/monsters", monsterHandler)
	mux.Handle("/v1/monsters/", monsterHandler)

	handler := middleware.Logger(mux)
	server := &http.Server{
		Addr:        ":" + cfg.Port,
		Handler:     handler,
		ReadTimeout: 15 * time.Second,
		// WriteTimeout removed to enable streaming - streaming endpoints handle their own timeouts
		IdleTimeout: 60 * time.Second,
	}

	go func() {
		log.Info("Server starting", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Server is shutting down...")

	// Close storage connection
	if err := storageService.Close(); err != nil {
		log.Error("Error closing storage connection", "error", err)
	}

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("Server forced to shutdown", "error", err)
		os.Exit(1)
	}

	log.Info("Server exited")
}
