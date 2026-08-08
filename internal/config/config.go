package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

const (
	VendorAnthropic = "anthropic"
	VendorVenice    = "venice"
)

// ProviderConfig is one named vendor+model pairing in the providers map.
// Adding a provider is JSON-only; adding a vendor requires Go.
type ProviderConfig struct {
	// Vendor selects the wire protocol: "anthropic" or "venice".
	Vendor string `json:"vendor"`
	// DisplayName is shown in the console picker; optional.
	DisplayName string `json:"display_name,omitempty"`
	// Model is the primary chat model id.
	Model string `json:"model"`
	// BackendModel is used for delta extraction; falls back to Model when empty.
	BackendModel string `json:"backend_model,omitempty"`
	// APIKey is the provider credential.
	APIKey string `json:"api_key,omitempty"`
}

type Config struct {
	Port             string                     `json:"port"`
	Environment      string                     `json:"environment"`
	LogLevel         slog.Level                 `json:"-"`
	LogLevelStr      string                     `json:"log_level"`
	Providers        map[string]*ProviderConfig `json:"providers"`
	DefaultProvider  string                     `json:"default_provider,omitempty"`
	RedisURL         string                     `json:"redis_url"`
	ChatHistoryLimit int                        `json:"chat_history_limit"` // max past messages sent to LLM per request (0 = use default)
}

// Load reads configuration from the CONFIG environment variable.
func Load() (*Config, error) {
	configFile := getEnv("CONFIG", "")
	if configFile == "" {
		return nil, fmt.Errorf("CONFIG environment variable is not set")
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %v", configFile, err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %v", configFile, err)
	}

	config.LogLevel = parseLogLevel(config.LogLevelStr)
	if err := config.validateProviders(); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *Config) validateProviders() error {
	if len(c.Providers) == 0 {
		return fmt.Errorf("providers: at least one provider is required")
	}

	for name, p := range c.Providers {
		if p == nil {
			return fmt.Errorf("providers[%q]: entry is null", name)
		}
		vendor := strings.ToLower(strings.TrimSpace(p.Vendor))
		switch vendor {
		case VendorAnthropic, VendorVenice:
			p.Vendor = vendor
		default:
			return fmt.Errorf("providers[%q]: unknown vendor %q (supported: %s, %s)", name, p.Vendor, VendorAnthropic, VendorVenice)
		}
		if strings.TrimSpace(p.Model) == "" {
			return fmt.Errorf("providers[%q]: model is required", name)
		}
		if strings.TrimSpace(p.APIKey) == "" {
			return fmt.Errorf("providers[%q]: api_key is required", name)
		}
	}

	if c.DefaultProvider == "" {
		if len(c.Providers) == 1 {
			for name := range c.Providers {
				c.DefaultProvider = name
			}
		} else {
			return fmt.Errorf("default_provider is required when more than one provider is configured")
		}
	}
	if _, ok := c.Providers[c.DefaultProvider]; !ok {
		return fmt.Errorf("default_provider %q does not match any entry in providers", c.DefaultProvider)
	}
	return nil
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
