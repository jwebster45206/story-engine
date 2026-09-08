package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jwebster45206/story-engine/internal/auth"
	"github.com/jwebster45206/story-engine/internal/config"
	"github.com/jwebster45206/story-engine/internal/handlers"
	"github.com/jwebster45206/story-engine/internal/llm"
	"github.com/jwebster45206/story-engine/internal/logger"
	"github.com/jwebster45206/story-engine/internal/middleware"
	"github.com/jwebster45206/story-engine/internal/queue"
	"github.com/jwebster45206/story-engine/internal/storage"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	pub, err := auth.LoadPublicKey(dir)
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

	registry, err := llm.NewRegistry(cfg, log)
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

	mux.Handle("/health", middleware.Instrument("health", handlers.NewHealthHandler(log, storageService)))
	mux.Handle("/metrics", promhttp.Handler())

	mux.Handle("/v1/chat", middleware.Protected("chat", pub, handlers.NewChatHandler(chatQueue, storageService, log)))
	mux.Handle("/v1/events/gamestate/", middleware.ProtectedSSE(pub, handlers.NewEventsHandler(redisClient, storageService, log)))

	gameState := middleware.Protected("gamestate", pub, handlers.NewGameStateHandler(log, registry, storageService))
	mux.Handle("/v1/gamestate", gameState)
	mux.Handle("/v1/gamestate/", gameState)

	mux.Handle("/v1/providers", middleware.Protected("providers", pub, handlers.NewProvidersHandler(log, registry)))

	scenarios := middleware.Protected("scenarios", pub, handlers.NewScenarioHandler(log, storageService))
	mux.Handle("/v1/scenarios", scenarios)
	mux.Handle("/v1/scenarios/", scenarios)

	pcs := middleware.Protected("pcs", pub, handlers.NewPCHandler(log, storageService))
	mux.Handle("/v1/pcs", pcs)
	mux.Handle("/v1/pcs/", pcs)

	narrators := middleware.Protected("narrators", pub, handlers.NewNarratorHandler(log, storageService))
	mux.Handle("/v1/narrators", narrators)
	mux.Handle("/v1/narrators/", narrators)

	monsters := middleware.Protected("monsters", pub, handlers.NewMonsterHandler(log, storageService))
	mux.Handle("/v1/monsters", monsters)
	mux.Handle("/v1/monsters/", monsters)

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
