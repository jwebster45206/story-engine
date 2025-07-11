package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Port        string     `json:"port"`
	Environment string     `json:"environment"`
	LogLevel    slog.Level `json:"-"`
	LogLevelStr string     `json:"log_level"`
	ModelURL    string     `json:"model_url"`
	ModelName   string     `json:"model_name"` // only tinyllama is supported
}

func Load() (*Config, error) {

	configFile := getEnv("ROLEPLAY_CONFIG", "")
	if configFile == "" {
		return nil, fmt.Errorf("ROLEPLAY_CONFIG environment variable is not set")
	}

	// Read config file
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %v", configFile, err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %v", configFile, err)
	}

	// Parse log level from string
	config.LogLevel = parseLogLevel(config.LogLevelStr)
	return &config, nil
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
