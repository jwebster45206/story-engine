package logger

import (
	"log/slog"
	"os"

	"github.com/jwebster45206/roleplay-agent/internal/config"
)

// Setup configures the global slog logger with JSON format
func Setup(cfg *config.Config) *slog.Logger {
	// Configure handler options
	opts := &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}

	handler := slog.NewJSONHandler(os.Stdout, opts)

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
