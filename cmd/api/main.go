package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jwebster45206/roleplay-agent/internal/config"
	"github.com/jwebster45206/roleplay-agent/internal/handlers"
	"github.com/jwebster45206/roleplay-agent/internal/logger"
	"github.com/jwebster45206/roleplay-agent/internal/middleware"
	"github.com/jwebster45206/roleplay-agent/internal/services"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	log := logger.Setup(cfg)

	log.Info("Starting roleplay-agent",
		"port", cfg.Port,
		"environment", cfg.Environment,
		"model_name", cfg.ModelName)

	if cfg.VeniceAPIKey == "" {
		log.Error("Venice API key is required")
		os.Exit(1)
	}
	llmService := services.NewVeniceService(cfg.VeniceAPIKey, cfg.ModelName)

	// Initialize storage service (Redis implementation)
	var storage services.Storage = services.NewRedisService(cfg.RedisURL, log)

	// Wait for storage to be available
	storageCtx, storageCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer storageCancel()

	if err := storage.Ping(storageCtx); err != nil {
		log.Error("Failed to connect to storage", "error", err)
		os.Exit(1)
	}
	log.Info("Storage connection established successfully")

	// Initialize the model on startup
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := llmService.InitModel(ctx, cfg.ModelName); err != nil {
		log.Error("Failed to initialize LLM model", "error", err, "model", cfg.ModelName)
		// Don't exit on model initialization failure in development
		if cfg.Environment == "production" {
			os.Exit(1)
		} else {
			log.Warn("Continuing without model initialization in non-production environment")
		}
	}
	mux := http.NewServeMux()

	healthHandler := handlers.NewHealthHandler(storage, llmService, log)
	mux.Handle("/health", healthHandler)

	chatHandler := handlers.NewChatHandler(llmService, log, storage)
	mux.Handle("/chat", chatHandler)

	gameStateHandler := handlers.NewGameStateHandler(storage, log)
	mux.Handle("/gamestate", gameStateHandler)
	mux.Handle("/gamestate/", gameStateHandler)

	handler := middleware.Logger(mux)
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
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
	if err := storage.Close(); err != nil {
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
