package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
		"port", cfg.Port,
		"environment", cfg.Environment,
		"llm_provider", cfg.LLMProvider,
		"model_name", cfg.ModelName)

	var llmService services.LLMService
	switch strings.ToLower(cfg.LLMProvider) {
	case "anthropic":
		// Initialize Anthropic LLM service
		if cfg.AnthropicAPIKey == "" {
			log.Error("Anthropic API key is required when using anthropic provider")
			os.Exit(1)
		}
		llmService = services.NewAnthropicService(cfg.AnthropicAPIKey, cfg.ModelName, cfg.BackendModelName, log)
		log.Info("Using Anthropic LLM provider")
	case "venice":
		// Initialize Venice LLM service
		if cfg.VeniceAPIKey == "" {
			log.Error("Venice API key is required when using venice provider")
			os.Exit(1)
		}
		llmService = services.NewVeniceService(cfg.VeniceAPIKey, cfg.ModelName, cfg.BackendModelName)
		log.Info("Using Venice LLM provider")
	// case "ollama": // TODO: Support for Ollama self-hosted LLM
	default:
		log.Error("Invalid LLM provider specified", "provider", cfg.LLMProvider, "supported", []string{"anthropic", "venice"})
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
	defer queueClient.Close()

	chatQueue := queue.NewChatQueue(queueClient)
	log.Info("Queue service initialized successfully")

	// Initialize the model on startup
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := llmService.InitModel(ctx, cfg.ModelName); err != nil {
		log.Error("Failed to initialize LLM model", "error", err, "model", cfg.ModelName)
		os.Exit(1)
	}

	mux := http.NewServeMux()

	healthHandler := handlers.NewHealthHandler(log, storageService, llmService)
	mux.Handle("/health", healthHandler)

	chatHandler := handlers.NewChatHandler(log, storageService, llmService, chatQueue)
	mux.Handle("/v1/chat", chatHandler)

	gameStateHandler := handlers.NewGameStateHandler(log, cfg.ModelName, storageService)
	mux.Handle("/v1/gamestate", gameStateHandler)
	mux.Handle("/v1/gamestate/", gameStateHandler)

	scenarioHandler := handlers.NewScenarioHandler(log, storageService)
	mux.Handle("/v1/scenarios", scenarioHandler)
	mux.Handle("/v1/scenarios/", scenarioHandler)

	pcHandler := handlers.NewPCHandler(log, storageService)
	mux.Handle("/v1/pcs", pcHandler)
	mux.Handle("/v1/pcs/", pcHandler)

	narratorHandler := handlers.NewNarratorHandler(log, storageService)
	mux.Handle("/v1/narrators", narratorHandler)
	mux.Handle("/v1/narrators/", narratorHandler)

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
