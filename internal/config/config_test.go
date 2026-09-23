package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, raw string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func loadFrom(t *testing.T, path string) (*Config, error) {
	t.Helper()
	t.Setenv("CONFIG", path)
	return Load()
}

func TestLoad_ValidProviders(t *testing.T) {
	path := writeConfig(t, `{
		"port":"8080",
		"default_provider":"sonnet",
		"providers":{
			"sonnet":{"vendor":"anthropic","api_key":"k","model":"m1"},
			"venice":{"vendor":"venice","api_key":"k2","model":"m2"}
		},
		"redis_url":"localhost:6379"
	}`)
	cfg, err := loadFrom(t, path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DefaultProvider != "sonnet" {
		t.Fatalf("default = %q", cfg.DefaultProvider)
	}
	if cfg.Providers["sonnet"].Vendor != VendorAnthropic {
		t.Fatalf("vendor = %q", cfg.Providers["sonnet"].Vendor)
	}
}

func TestLoad_SingleProviderImplicitDefault(t *testing.T) {
	path := writeConfig(t, `{
		"providers":{"only":{"vendor":"venice","api_key":"k","model":"m"}},
		"redis_url":"localhost:6379"
	}`)
	cfg, err := loadFrom(t, path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DefaultProvider != "only" {
		t.Fatalf("default = %q, want only", cfg.DefaultProvider)
	}
}

func TestLoad_UnknownVendor(t *testing.T) {
	path := writeConfig(t, `{
		"providers":{"x":{"vendor":"groq","api_key":"k","model":"m"}},
		"redis_url":"localhost:6379"
	}`)
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error for unknown vendor")
	}
}

func TestLoad_MissingModel(t *testing.T) {
	path := writeConfig(t, `{
		"providers":{"x":{"vendor":"anthropic","api_key":"k"}},
		"redis_url":"localhost:6379"
	}`)
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error for missing model")
	}
}

func TestLoad_MissingAPIKey(t *testing.T) {
	path := writeConfig(t, `{
		"providers":{"x":{"vendor":"anthropic","model":"m"}},
		"redis_url":"localhost:6379"
	}`)
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error for missing api_key")
	}
}

func TestLoad_MissingDefaultWithMultipleProviders(t *testing.T) {
	path := writeConfig(t, `{
		"providers":{
			"a":{"vendor":"anthropic","api_key":"k","model":"m"},
			"b":{"vendor":"venice","api_key":"k","model":"m"}
		},
		"redis_url":"localhost:6379"
	}`)
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error when default_provider missing with multiple providers")
	}
}

func TestLoad_BadDefaultProvider(t *testing.T) {
	path := writeConfig(t, `{
		"default_provider":"missing",
		"providers":{"a":{"vendor":"anthropic","api_key":"k","model":"m"}},
		"redis_url":"localhost:6379"
	}`)
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error for unknown default_provider")
	}
}

func TestProviderConfigJSONRoundTrip(t *testing.T) {
	raw := []byte(`{"vendor":"anthropic","model":"m","backend_model":"b","display_name":"D"}`)
	var pc ProviderConfig
	if err := json.Unmarshal(raw, &pc); err != nil {
		t.Fatal(err)
	}
	if pc.BackendModel != "b" || pc.DisplayName != "D" {
		t.Fatalf("unexpected %#v", pc)
	}
}

func TestLoad_SameVendorBackendVendor(t *testing.T) {
	path := writeConfig(t, `{
		"providers":{"only":{"vendor":"anthropic","api_key":"k","model":"m","backend_vendor":"anthropic","backend_model":"b"}},
		"redis_url":"localhost:6379"
	}`)
	cfg, err := loadFrom(t, path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p := cfg.Providers["only"]
	if p.BackendVendor != VendorAnthropic {
		t.Fatalf("backend_vendor = %q", p.BackendVendor)
	}
	if p.SplitBackend() {
		t.Fatal("same-vendor backend_vendor should not split")
	}
}

func TestLoad_SplitBackendVendor(t *testing.T) {
	path := writeConfig(t, `{
		"providers":{"venice-rp":{
			"vendor":"venice",
			"api_key":"vk",
			"model":"rp",
			"backend_vendor":"anthropic",
			"backend_api_key":"ak",
			"backend_model":"claude-haiku-4-5"
		}},
		"redis_url":"localhost:6379"
	}`)
	cfg, err := loadFrom(t, path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	p := cfg.Providers["venice-rp"]
	if p.Vendor != VendorVenice || p.BackendVendor != VendorAnthropic {
		t.Fatalf("vendors = %q / %q", p.Vendor, p.BackendVendor)
	}
	if p.BackendAPIKey != "ak" || p.BackendModel != "claude-haiku-4-5" {
		t.Fatalf("backend fields = %#v", p)
	}
	if !p.SplitBackend() {
		t.Fatal("expected split backend")
	}
}

func TestLoad_SplitBackendMissingModel(t *testing.T) {
	path := writeConfig(t, `{
		"providers":{"x":{"vendor":"venice","api_key":"vk","model":"rp","backend_vendor":"anthropic","backend_api_key":"ak"}},
		"redis_url":"localhost:6379"
	}`)
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error for missing backend_model")
	}
}

func TestLoad_SplitBackendMissingAPIKey(t *testing.T) {
	path := writeConfig(t, `{
		"providers":{"x":{"vendor":"venice","api_key":"vk","model":"rp","backend_vendor":"anthropic","backend_model":"haiku"}},
		"redis_url":"localhost:6379"
	}`)
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error for missing backend_api_key")
	}
}

func TestLoad_UnknownBackendVendor(t *testing.T) {
	path := writeConfig(t, `{
		"providers":{"x":{"vendor":"venice","api_key":"vk","model":"rp","backend_vendor":"groq","backend_api_key":"ak","backend_model":"m"}},
		"redis_url":"localhost:6379"
	}`)
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error for unknown backend_vendor")
	}
}

func TestLoad_UnusedBackendAPIKeyWithoutVendor(t *testing.T) {
	path := writeConfig(t, `{
		"providers":{"x":{"vendor":"anthropic","api_key":"k","model":"m","backend_api_key":"ak"}},
		"redis_url":"localhost:6379"
	}`)
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error for unused backend_api_key")
	}
}

func TestLoad_UnusedBackendAPIKeySameVendor(t *testing.T) {
	path := writeConfig(t, `{
		"providers":{"x":{"vendor":"anthropic","api_key":"k","model":"m","backend_vendor":"anthropic","backend_api_key":"ak"}},
		"redis_url":"localhost:6379"
	}`)
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error for unused backend_api_key when vendors match")
	}
}

func TestProviderConfigJSONRoundTripSplit(t *testing.T) {
	raw := []byte(`{"vendor":"venice","model":"rp","backend_vendor":"anthropic","backend_api_key":"ak","backend_model":"haiku"}`)
	var pc ProviderConfig
	if err := json.Unmarshal(raw, &pc); err != nil {
		t.Fatal(err)
	}
	if pc.BackendVendor != "anthropic" || pc.BackendAPIKey != "ak" || pc.BackendModel != "haiku" {
		t.Fatalf("unexpected %#v", pc)
	}
}
