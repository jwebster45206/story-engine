package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr string
		check   func(*testing.T, *Config)
	}{
		{
			name: "two providers with default",
			raw: `{
				"default_provider":"sonnet",
				"providers":{
					"sonnet":{"vendor":"anthropic","api_key":"k","model":"m1"},
					"venice":{"vendor":"venice","api_key":"k2","model":"m2"}
				},
				"redis_url":"localhost:6379"
			}`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.DefaultProvider != "sonnet" {
					t.Fatalf("default = %q", cfg.DefaultProvider)
				}
				if cfg.Providers["sonnet"].Vendor != VendorAnthropic {
					t.Fatalf("vendor = %q", cfg.Providers["sonnet"].Vendor)
				}
			},
		},
		{
			name: "single provider infers default",
			raw: `{
				"providers":{"only":{"vendor":"venice","api_key":"k","model":"m"}},
				"redis_url":"localhost:6379"
			}`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.DefaultProvider != "only" {
					t.Fatalf("default = %q", cfg.DefaultProvider)
				}
			},
		},
		{
			name: "normalizes vendor case",
			raw: `{
				"providers":{"x":{"vendor":"Anthropic","api_key":"k","model":"m"}},
				"redis_url":"localhost:6379"
			}`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				if cfg.Providers["x"].Vendor != VendorAnthropic {
					t.Fatalf("vendor = %q", cfg.Providers["x"].Vendor)
				}
			},
		},
		{
			name: "same-vendor backend_vendor is not a split",
			raw: `{
				"providers":{"only":{"vendor":"anthropic","api_key":"k","model":"m","backend_vendor":"anthropic","backend_model":"b"}},
				"redis_url":"localhost:6379"
			}`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
				p := cfg.Providers["only"]
				if p.BackendVendor != VendorAnthropic {
					t.Fatalf("backend_vendor = %q", p.BackendVendor)
				}
				if p.SplitBackend() {
					t.Fatal("same-vendor backend_vendor should not split")
				}
			},
		},
		{
			name: "split backend vendor",
			raw: `{
				"providers":{"venice-rp":{
					"vendor":"venice",
					"api_key":"vk",
					"model":"rp",
					"backend_vendor":"anthropic",
					"backend_api_key":"ak",
					"backend_model":"claude-haiku-4-5"
				}},
				"redis_url":"localhost:6379"
			}`,
			check: func(t *testing.T, cfg *Config) {
				t.Helper()
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
			},
		},
		{
			name:    "unknown vendor",
			raw:     `{"providers":{"x":{"vendor":"groq","api_key":"k","model":"m"}},"redis_url":"localhost:6379"}`,
			wantErr: `unknown vendor "groq"`,
		},
		{
			name:    "missing model",
			raw:     `{"providers":{"x":{"vendor":"anthropic","api_key":"k"}},"redis_url":"localhost:6379"}`,
			wantErr: "model is required",
		},
		{
			name:    "missing api_key",
			raw:     `{"providers":{"x":{"vendor":"anthropic","model":"m"}},"redis_url":"localhost:6379"}`,
			wantErr: "api_key is required",
		},
		{
			name: "missing default with multiple providers",
			raw: `{
				"providers":{
					"a":{"vendor":"anthropic","api_key":"k","model":"m"},
					"b":{"vendor":"venice","api_key":"k","model":"m"}
				},
				"redis_url":"localhost:6379"
			}`,
			wantErr: "default_provider is required",
		},
		{
			name:    "unknown default_provider",
			raw:     `{"default_provider":"missing","providers":{"a":{"vendor":"anthropic","api_key":"k","model":"m"}},"redis_url":"localhost:6379"}`,
			wantErr: `default_provider "missing" does not match`,
		},
		{
			name:    "empty providers",
			raw:     `{"providers":{},"redis_url":"localhost:6379"}`,
			wantErr: "at least one provider is required",
		},
		{
			name:    "null provider entry",
			raw:     `{"providers":{"x":null},"redis_url":"localhost:6379"}`,
			wantErr: "entry is null",
		},
		{
			name:    "split missing backend_model",
			raw:     `{"providers":{"x":{"vendor":"venice","api_key":"vk","model":"rp","backend_vendor":"anthropic","backend_api_key":"ak"}},"redis_url":"localhost:6379"}`,
			wantErr: "backend_model is required",
		},
		{
			name:    "split missing backend_api_key",
			raw:     `{"providers":{"x":{"vendor":"venice","api_key":"vk","model":"rp","backend_vendor":"anthropic","backend_model":"haiku"}},"redis_url":"localhost:6379"}`,
			wantErr: "backend_api_key is required when backend_vendor differs",
		},
		{
			name:    "unknown backend_vendor",
			raw:     `{"providers":{"x":{"vendor":"venice","api_key":"vk","model":"rp","backend_vendor":"groq","backend_api_key":"ak","backend_model":"m"}},"redis_url":"localhost:6379"}`,
			wantErr: `unknown backend_vendor "groq"`,
		},
		{
			name:    "backend_api_key without backend_vendor",
			raw:     `{"providers":{"x":{"vendor":"anthropic","api_key":"k","model":"m","backend_api_key":"ak"}},"redis_url":"localhost:6379"}`,
			wantErr: "backend_api_key is unused without backend_vendor",
		},
		{
			name:    "backend_api_key when vendors match",
			raw:     `{"providers":{"x":{"vendor":"anthropic","api_key":"k","model":"m","backend_vendor":"anthropic","backend_api_key":"ak"}},"redis_url":"localhost:6379"}`,
			wantErr: "backend_api_key is unused when backend_vendor matches vendor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CONFIG", writeConfig(t, tt.raw))
			cfg, err := Load()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestLoad_MissingCONFIG(t *testing.T) {
	t.Setenv("CONFIG", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when CONFIG is unset")
	}
}
