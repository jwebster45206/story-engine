package logger

import (
	"log/slog"
	"os"

	"github.com/jwebster45206/roleplay-agent/internal/config"
)

// Setup configures the global slog logger based on environment
func Setup(cfg *config.Config) *slog.Logger {
	var handler slog.Handler

	// Configure handler based on environment
	opts := &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}

	if cfg.Environment == "production" {
		// JSON format for production
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		// Text format for development
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)

	// Set as default logger
	slog.SetDefault(logger)

	return logger
}

// WithRequestID adds request ID to logger context
func WithRequestID(logger *slog.Logger, requestID string) *slog.Logger {
	return logger.With("request_id", requestID)
}

// WithError adds error to logger context
func WithError(logger *slog.Logger, err error) *slog.Logger {
	return logger.With("error", err.Error())
}
