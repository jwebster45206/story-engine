package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testAPIUUID    = "22222222-2222-4222-8222-222222222222"
	testSharedUUID = "33333333-3333-4333-8333-333333333333"
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
		"redis_url":"localhost:6379",
		"admin_keys":["11111111-1111-4111-8111-111111111111"]
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
		"redis_url":"localhost:6379",
		"admin_keys":["11111111-1111-4111-8111-111111111111"]
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
		"redis_url":"localhost:6379",
		"admin_keys":["11111111-1111-4111-8111-111111111111"]
	}`)
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error for unknown vendor")
	}
}

func TestLoad_MissingModel(t *testing.T) {
	path := writeConfig(t, `{
		"providers":{"x":{"vendor":"anthropic","api_key":"k"}},
		"redis_url":"localhost:6379",
		"admin_keys":["11111111-1111-4111-8111-111111111111"]
	}`)
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error for missing model")
	}
}

func TestLoad_MissingAPIKey(t *testing.T) {
	path := writeConfig(t, `{
		"providers":{"x":{"vendor":"anthropic","model":"m"}},
		"redis_url":"localhost:6379",
		"admin_keys":["11111111-1111-4111-8111-111111111111"]
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
		"redis_url":"localhost:6379",
		"admin_keys":["11111111-1111-4111-8111-111111111111"]
	}`)
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error when default_provider missing with multiple providers")
	}
}

func TestLoad_BadDefaultProvider(t *testing.T) {
	path := writeConfig(t, `{
		"default_provider":"missing",
		"providers":{"a":{"vendor":"anthropic","api_key":"k","model":"m"}},
		"redis_url":"localhost:6379",
		"admin_keys":["11111111-1111-4111-8111-111111111111"]
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

func validProviderJSON(auth string) string {
	return `{
		"providers":{"only":{"vendor":"venice","api_key":"k","model":"m"}},
		"redis_url":"localhost:6379",
		` + auth + `
	}`
}

func TestLoad_MissingAuthKeys(t *testing.T) {
	path := writeConfig(t, `{
		"providers":{"only":{"vendor":"venice","api_key":"k","model":"m"}},
		"redis_url":"localhost:6379"
	}`)
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error when no api_keys or admin_keys")
	}
}

func TestLoad_InvalidUUID(t *testing.T) {
	path := writeConfig(t, validProviderJSON(`"api_keys":["not-a-uuid"]`))
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error for non-UUID api_key")
	}
}

func TestLoad_NilUUID(t *testing.T) {
	path := writeConfig(t, validProviderJSON(`"api_keys":["00000000-0000-0000-0000-000000000000"]`))
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error for nil UUID")
	}
}

func TestLoad_DuplicateKey(t *testing.T) {
	path := writeConfig(t, validProviderJSON(`"api_keys":["`+testAPIUUID+`","`+testAPIUUID+`"]`))
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error for duplicate api_key")
	}
}

func TestLoad_DuplicateCanonicalForm(t *testing.T) {
	path := writeConfig(t, validProviderJSON(`"api_keys":["`+testAPIUUID+`","{22222222-2222-4222-8222-222222222222}"]`))
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("expected error for duplicate canonical UUID")
	}
}

func TestLoad_APIKeyOnly(t *testing.T) {
	path := writeConfig(t, validProviderJSON(`"api_keys":["`+testAPIUUID+`"]`))
	cfg, err := loadFrom(t, path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.APIKeyHashes) != 1 || len(cfg.AdminKeyHashes) != 0 {
		t.Fatalf("hashes api=%d admin=%d", len(cfg.APIKeyHashes), len(cfg.AdminKeyHashes))
	}
	canon, err := CanonicalKey(testAPIUUID)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKeyHashes[0] != HashKey(canon) {
		t.Fatal("api key hash mismatch")
	}
}

func TestLoad_KeyInBothLists(t *testing.T) {
	path := writeConfig(t, validProviderJSON(`"api_keys":["`+testSharedUUID+`"],"admin_keys":["`+testSharedUUID+`"]`))
	cfg, err := loadFrom(t, path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.APIKeyHashes) != 1 || len(cfg.AdminKeyHashes) != 1 {
		t.Fatalf("hashes api=%d admin=%d", len(cfg.APIKeyHashes), len(cfg.AdminKeyHashes))
	}
}

func TestCanonicalKey(t *testing.T) {
	canon, err := CanonicalKey(strings.ToUpper(testAPIUUID))
	if err != nil {
		t.Fatal(err)
	}
	if canon != testAPIUUID {
		t.Fatalf("canon = %q", canon)
	}
	if _, err := CanonicalKey("not-a-uuid"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCanonicalKey_RejectsPlaceholder(t *testing.T) {
	if _, err := CanonicalKey("YOUR_UUID_HERE"); err == nil {
		t.Fatal("placeholder must not parse as a UUID")
	}
}

func TestCommittedTemplate_RejectsPlaceholderKeys(t *testing.T) {
	path := filepath.Join("..", "..", "config.template.json")
	if _, err := loadFrom(t, path); err == nil {
		t.Fatal("config.template.json must not load until YOUR_UUID_HERE is replaced")
	}
}
