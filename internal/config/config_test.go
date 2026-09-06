package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, raw string) string {
	t.Helper()
	return writeConfigRaw(t, withJWTKey(t, raw))
}

func writeConfigRaw(t *testing.T, raw string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func testJWTPublicKey(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "auth", "testdata", "ec-p256.pub.pem"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func withJWTKey(t *testing.T, raw string) string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["jwt_public_key"]; !ok {
		b, err := json.Marshal(testJWTPublicKey(t))
		if err != nil {
			t.Fatal(err)
		}
		m["jwt_public_key"] = b
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
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

func TestLoad_MissingJWTPublicKey(t *testing.T) {
	path := writeConfigRaw(t, `{
		"providers":{"only":{"vendor":"venice","api_key":"k","model":"m"}},
		"redis_url":"localhost:6379"
	}`)
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error when jwt_public_key is missing")
	}
}

func TestLoad_InvalidJWTPublicKey(t *testing.T) {
	path := writeConfigRaw(t, `{
		"providers":{"only":{"vendor":"venice","api_key":"k","model":"m"}},
		"redis_url":"localhost:6379",
		"jwt_public_key":"YOUR_ES256_PUBLIC_KEY_PEM_HERE"
	}`)
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error for placeholder jwt_public_key")
	}
}

func TestLoad_ValidJWTPublicKey(t *testing.T) {
	path := writeConfig(t, `{
		"providers":{"only":{"vendor":"venice","api_key":"k","model":"m"}},
		"redis_url":"localhost:6379"
	}`)
	cfg, err := loadFrom(t, path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.JWTPublicKey == nil {
		t.Fatal("expected parsed JWT public key")
	}
}

func TestCommittedTemplate_RejectsPlaceholderKey(t *testing.T) {
	path := filepath.Join("..", "..", "config.template.json")
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("config.template.json must not load until jwt_public_key is a real PEM")
	}
}
