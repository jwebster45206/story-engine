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

	// Initialize Venice AI service (hardcoded as primary provider)
	if cfg.VeniceAPIKey == "" {
		log.Error("Venice API key is required")
		os.Exit(1)
	}
	llmService := services.NewVeniceService(cfg.VeniceAPIKey, cfg.ModelName)
	log.Info("Using Venice AI as LLM provider")

	// Initialize cache service (Redis implementation)
	var cache services.Cache = services.NewRedisService(cfg.RedisURL, log)

	// Wait for cache to be available
	cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cacheCancel()

	if err := cache.WaitForConnection(cacheCtx); err != nil {
		log.Error("Failed to connect to cache", "error", err)
		// Don't exit on cache failure in development
		if cfg.Environment == "production" {
			os.Exit(1)
		} else {
			log.Warn("Continuing without cache connection in non-production environment")
		}
	} else {
		log.Info("Cache connection established successfully")
	}

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

	healthHandler := handlers.NewHealthHandler(cache, llmService, log)
	mux.Handle("/health", healthHandler)

	// Create chat handler with LLM service
	chatHandler := handlers.NewChatHandler(llmService, log)
	mux.Handle("/chat", chatHandler)

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

	// Close cache connection
	if err := cache.Close(); err != nil {
		log.Error("Error closing cache connection", "error", err)
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
