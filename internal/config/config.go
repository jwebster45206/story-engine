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
	LLMURL      string     `json:"llm_url"`
}

func Load() *Config {
	// Determine config file to load based on environment variable
	configFile := getEnv("CONFIG_FILE", "config.docker.json")

	// Read config file
	data, err := os.ReadFile(configFile)
	if err != nil {
		// Fallback to default config if file doesn't exist
		return &Config{
			Port:        "8080",
			Environment: "development",
			LogLevel:    slog.LevelInfo,
			LogLevelStr: "info",
			LLMURL:      "http://localhost:11434",
		}
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		panic(fmt.Sprintf("Failed to parse config file %s: %v", configFile, err))
	}

	// Parse log level from string
	config.LogLevel = parseLogLevel(config.LogLevelStr)

	return &config
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
