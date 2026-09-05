package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"uuid"
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
	APIKeys          []uuid.UUID                `json:"api_keys,omitempty"`
	AdminKeys        []uuid.UUID                `json:"admin_keys,omitempty"`
	APIKeyHashes     []string                   `json:"-"`
	AdminKeyHashes   []string                   `json:"-"`
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
	if err := config.validateAuth(); err != nil {
		return nil, err
	}
	return &config, nil
}

// CanonicalKey parses s as a UUID. The nil UUID is rejected.
func CanonicalKey(s string) (uuid.UUID, error) {
	u, err := uuid.Parse(strings.TrimSpace(s))
	if err != nil {
		return uuid.Nil(), fmt.Errorf("must be a UUID")
	}
	if u == uuid.Nil() {
		return uuid.Nil(), fmt.Errorf("nil UUID is not allowed")
	}
	return u, nil
}

// HashKey returns the SHA-256 hex digest of the canonical UUID string.
func HashKey(key uuid.UUID) string {
	sum := sha256.Sum256([]byte(key.String()))
	return hex.EncodeToString(sum[:])
}

func (c *Config) validateAuth() error {
	apiHashes, err := normalizeKeyList("api_keys", c.APIKeys)
	if err != nil {
		return err
	}
	adminHashes, err := normalizeKeyList("admin_keys", c.AdminKeys)
	if err != nil {
		return err
	}
	if len(apiHashes)+len(adminHashes) == 0 {
		return fmt.Errorf("api_keys or admin_keys: at least one key is required")
	}

	adminSet := make(map[string]struct{}, len(adminHashes))
	for _, h := range adminHashes {
		adminSet[h] = struct{}{}
	}
	for _, h := range apiHashes {
		if _, ok := adminSet[h]; ok {
			return fmt.Errorf("api_keys and admin_keys must not overlap")
		}
	}

	c.APIKeyHashes = apiHashes
	c.AdminKeyHashes = adminHashes
	return nil
}

func normalizeKeyList(field string, keys []uuid.UUID) ([]string, error) {
	seen := make(map[uuid.UUID]struct{}, len(keys))
	hashes := make([]string, 0, len(keys))
	for i, key := range keys {
		if key == uuid.Nil() {
			return nil, fmt.Errorf("%s[%d]: nil UUID is not allowed", field, i)
		}
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("%s: duplicate key", field)
		}
		seen[key] = struct{}{}
		hashes = append(hashes, HashKey(key))
	}
	return hashes, nil
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
