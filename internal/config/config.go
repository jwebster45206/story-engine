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
	// BackendModel is the non-narrator model; falls back to Model when empty.
	BackendModel string `json:"backend_model,omitempty"`
	// BackendVendor, when set and different from Vendor, runs referee/reducer
	// on a second vendor. Requires BackendModel and BackendAPIKey.
	BackendVendor string `json:"backend_vendor,omitempty"`
	// BackendAPIKey is the credential for BackendVendor. Required when
	// BackendVendor differs from Vendor; rejected when unused.
	BackendAPIKey string `json:"backend_api_key,omitempty"`
	// APIKey is the provider credential.
	APIKey string `json:"api_key,omitempty"`
}

// SplitBackend reports whether referee/reducer use a different vendor than the narrator.
func (p *ProviderConfig) SplitBackend() bool {
	if p == nil {
		return false
	}
	return p.BackendVendor != "" && p.BackendVendor != p.Vendor
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
		vendor, err := parseVendor(name, "vendor", p.Vendor)
		if err != nil {
			return err
		}
		p.Vendor = vendor
		if strings.TrimSpace(p.Model) == "" {
			return fmt.Errorf("providers[%q]: model is required", name)
		}
		if strings.TrimSpace(p.APIKey) == "" {
			return fmt.Errorf("providers[%q]: api_key is required", name)
		}
		if err := p.validateBackend(name); err != nil {
			return err
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

func parseVendor(name, field, value string) (string, error) {
	vendor := strings.ToLower(strings.TrimSpace(value))
	switch vendor {
	case VendorAnthropic, VendorVenice:
		return vendor, nil
	default:
		return "", fmt.Errorf("providers[%q]: unknown %s %q (supported: %s, %s)", name, field, value, VendorAnthropic, VendorVenice)
	}
}

func (p *ProviderConfig) validateBackend(name string) error {
	backendVendorRaw := strings.TrimSpace(p.BackendVendor)
	backendKey := strings.TrimSpace(p.BackendAPIKey)
	if backendVendorRaw == "" {
		if backendKey != "" {
			return fmt.Errorf("providers[%q]: backend_api_key is unused without backend_vendor", name)
		}
		p.BackendVendor = ""
		return nil
	}
	backendVendor, err := parseVendor(name, "backend_vendor", backendVendorRaw)
	if err != nil {
		return err
	}
	p.BackendVendor = backendVendor
	if backendVendor == p.Vendor {
		if backendKey != "" {
			return fmt.Errorf("providers[%q]: backend_api_key is unused when backend_vendor matches vendor", name)
		}
		return nil
	}
	if strings.TrimSpace(p.BackendModel) == "" {
		return fmt.Errorf("providers[%q]: backend_model is required when backend_vendor differs from vendor", name)
	}
	if backendKey == "" {
		return fmt.Errorf("providers[%q]: backend_api_key is required when backend_vendor differs from vendor", name)
	}
	p.BackendAPIKey = backendKey
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
