package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/internal/config"
	"github.com/jwebster45206/story-engine/pkg/chat"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type wantService struct {
	kind     string
	narrator string
	backend  string
	info     ProviderInfo
}

func serviceKind(svc LLMService) (kind, narrator, backend string) {
	switch s := svc.(type) {
	case *splitService:
		n, _, _ := serviceKind(s.narrator)
		b, _, _ := serviceKind(s.backend)
		return "split", n, b
	case *AnthropicService:
		return "anthropic", "", ""
	case *VeniceService:
		return "venice", "", ""
	default:
		return fmt.Sprintf("%T", svc), "", ""
	}
}

func TestNewRegistry(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *config.Config
		wantErr     string
		wantDefault string
		want        map[string]wantService
	}{
		{
			name: "dispatches by vendor",
			cfg: &config.Config{
				DefaultProvider: "sonnet",
				Providers: map[string]*config.ProviderConfig{
					"sonnet": {Vendor: config.VendorAnthropic, APIKey: "k", Model: "m1"},
					"uncen":  {Vendor: config.VendorVenice, APIKey: "k", Model: "m2"},
				},
			},
			wantDefault: "sonnet",
			want: map[string]wantService{
				"sonnet": {
					kind: "anthropic",
					info: ProviderInfo{Name: "sonnet", DisplayName: "sonnet", Vendor: config.VendorAnthropic, Model: "m1"},
				},
				"uncen": {
					kind: "venice",
					info: ProviderInfo{Name: "uncen", DisplayName: "uncen", Vendor: config.VendorVenice, Model: "m2"},
				},
			},
		},
		{
			name: "split backend wraps narrator and backend vendors",
			cfg: &config.Config{
				DefaultProvider: "venice-rp",
				Providers: map[string]*config.ProviderConfig{
					"venice-rp": {
						Vendor:        config.VendorVenice,
						DisplayName:   "Venice Roleplay",
						APIKey:        "vk",
						Model:         "venice-uncensored-role-play",
						BackendVendor: config.VendorAnthropic,
						BackendAPIKey: "ak",
						BackendModel:  "claude-haiku-4-5",
					},
				},
			},
			wantDefault: "venice-rp",
			want: map[string]wantService{
				"venice-rp": {
					kind:     "split",
					narrator: "venice",
					backend:  "anthropic",
					info: ProviderInfo{
						Name:        "venice-rp",
						DisplayName: "Venice Roleplay",
						Vendor:      config.VendorVenice,
						Model:       "venice-uncensored-role-play",
					},
				},
			},
		},
		{
			name: "same-vendor backend_vendor is not a split",
			cfg: &config.Config{
				DefaultProvider: "sonnet",
				Providers: map[string]*config.ProviderConfig{
					"sonnet": {
						Vendor:        config.VendorAnthropic,
						APIKey:        "k",
						Model:         "m1",
						BackendVendor: config.VendorAnthropic,
						BackendModel:  "haiku",
					},
				},
			},
			wantDefault: "sonnet",
			want: map[string]wantService{
				"sonnet": {
					kind: "anthropic",
					info: ProviderInfo{Name: "sonnet", DisplayName: "sonnet", Vendor: config.VendorAnthropic, Model: "m1"},
				},
			},
		},
		{
			name:    "nil config",
			wantErr: "config is required",
		},
		{
			name: "unsupported vendor",
			cfg: &config.Config{
				DefaultProvider: "x",
				Providers: map[string]*config.ProviderConfig{
					"x": {Vendor: "groq", APIKey: "k", Model: "m"},
				},
			},
			wantErr: `unsupported vendor "groq"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, err := NewRegistry(tt.cfg, testLogger())
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
				t.Fatalf("NewRegistry: %v", err)
			}
			if reg.Default() != tt.wantDefault {
				t.Fatalf("Default = %q, want %q", reg.Default(), tt.wantDefault)
			}
			for name, want := range tt.want {
				svc, err := reg.Get(name)
				if err != nil {
					t.Fatalf("Get(%q): %v", name, err)
				}
				kind, narrator, backend := serviceKind(svc)
				if kind != want.kind || narrator != want.narrator || backend != want.backend {
					t.Fatalf("%s: got kind=%s narrator=%s backend=%s, want %+v", name, kind, narrator, backend, want)
				}
				info, ok := reg.Info(name)
				if !ok || info != want.info {
					t.Fatalf("%s info = %#v ok=%v, want %#v", name, info, ok, want.info)
				}
			}
		})
	}
}

func TestRegistry_Get(t *testing.T) {
	reg, err := NewRegistry(&config.Config{
		DefaultProvider: "sonnet",
		Providers: map[string]*config.ProviderConfig{
			"sonnet": {Vendor: config.VendorAnthropic, APIKey: "k", Model: "m1"},
			"uncen":  {Vendor: config.VendorVenice, APIKey: "k", Model: "m2"},
		},
	}, testLogger())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	tests := []struct {
		name     string
		get      string
		wantKind string
		wantErr  string
	}{
		{name: "named anthropic", get: "sonnet", wantKind: "anthropic"},
		{name: "named venice", get: "uncen", wantKind: "venice"},
		{name: "empty uses default", get: "", wantKind: "anthropic"},
		{name: "unknown", get: "nope", wantErr: `unknown provider "nope"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, err := reg.Get(tt.get)
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
				t.Fatal(err)
			}
			kind, _, _ := serviceKind(svc)
			if kind != tt.wantKind {
				t.Fatalf("kind = %s, want %s", kind, tt.wantKind)
			}
		})
	}
}

func TestRegistry_SplitBackendHTTP(t *testing.T) {
	var veniceHits, anthropicHits int
	veniceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		veniceHits++
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for _, line := range []string{
			`data: {"id":"1","object":"chat.completion.chunk","created":1,"model":"rp","choices":[{"index":0,"delta":{"content":"Ahoy"}}]}`,
			`data: [DONE]`,
		} {
			_, _ = w.Write([]byte(line + "\n"))
			w.(http.Flusher).Flush()
		}
	}))
	defer veniceServer.Close()

	anthropicServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		anthropicHits++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["tools"]; !ok {
			t.Error("expected anthropic tools on backend call")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"tool_use","id":"t1","name":"ruling","input":{"allowed":true,"reasoning":"ok","reaction":null,"scope":"dialogue","subject":null,"object":null}}],"model":"claude-haiku-4-5","stop_reason":"tool_use","usage":{"input_tokens":1,"output_tokens":1}}`))
	}))
	defer anthropicServer.Close()

	reg, err := NewRegistry(&config.Config{
		DefaultProvider: "venice-rp",
		Providers: map[string]*config.ProviderConfig{
			"venice-rp": {
				Vendor:        config.VendorVenice,
				APIKey:        "vk",
				Model:         "rp",
				BackendVendor: config.VendorAnthropic,
				BackendAPIKey: "ak",
				BackendModel:  "claude-haiku-4-5",
			},
		},
	}, testLogger())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	svc, err := reg.Get("venice-rp")
	if err != nil {
		t.Fatal(err)
	}
	split := svc.(*splitService)
	split.narrator.(*VeniceService).baseURL = veniceServer.URL
	split.backend.(*AnthropicService).baseURL = anthropicServer.URL

	ch, err := svc.ChatStream(context.Background(), []chat.ChatMessage{{Role: chat.ChatRoleUser, Content: "hi"}}, DefaultTemperature)
	if err != nil {
		t.Fatal(err)
	}
	var content strings.Builder
	for chunk := range ch {
		if chunk.Error != nil {
			t.Fatal(chunk.Error)
		}
		content.WriteString(chunk.Content)
	}
	if content.String() != "Ahoy" {
		t.Fatalf("stream = %q", content.String())
	}

	ruling, usage, err := svc.GetRuling(context.Background(), []chat.ChatMessage{{Role: chat.ChatRoleUser, Content: "hi"}})
	if err != nil {
		t.Fatal(err)
	}
	if ruling == nil || !ruling.Allowed {
		t.Fatalf("ruling = %#v", ruling)
	}
	if usage.Vendor != vendorAnthropic || usage.Model != "claude-haiku-4-5" {
		t.Fatalf("usage = %+v", usage)
	}
	if veniceHits != 1 || anthropicHits != 1 {
		t.Fatalf("hits venice=%d anthropic=%d", veniceHits, anthropicHits)
	}
}
