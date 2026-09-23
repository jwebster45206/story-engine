package llm

import (
	"context"
	"encoding/json"
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

func TestNewRegistry_Dispatch(t *testing.T) {
	cfg := &config.Config{
		DefaultProvider: "sonnet",
		Providers: map[string]*config.ProviderConfig{
			"sonnet": {Vendor: config.VendorAnthropic, APIKey: "k", Model: "m1"},
			"uncen":  {Vendor: config.VendorVenice, APIKey: "k", Model: "m2"},
		},
	}
	reg, err := NewRegistry(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	if reg.Default() != "sonnet" {
		t.Fatalf("Default = %q", reg.Default())
	}

	svc, err := reg.Get("sonnet")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := svc.(*AnthropicService); !ok {
		t.Fatalf("expected *AnthropicService, got %T", svc)
	}

	svc, err = reg.Get("uncen")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := svc.(*VeniceService); !ok {
		t.Fatalf("expected *VeniceService, got %T", svc)
	}

	svc, err = reg.Get("")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := svc.(*AnthropicService); !ok {
		t.Fatalf("empty name should resolve default anthropic, got %T", svc)
	}

	if _, err := reg.Get("nope"); err == nil {
		t.Fatal("expected error for unknown provider")
	}

	info, ok := reg.Info("sonnet")
	if !ok || info.Model != "m1" || info.Vendor != config.VendorAnthropic {
		t.Fatalf("unexpected info: %#v", info)
	}
}

func TestNewRegistry_SplitBackend(t *testing.T) {
	cfg := &config.Config{
		DefaultProvider: "venice-rp",
		Providers: map[string]*config.ProviderConfig{
			"venice-rp": {
				Vendor:        config.VendorVenice,
				APIKey:        "vk",
				Model:         "venice-uncensored-role-play",
				BackendVendor: config.VendorAnthropic,
				BackendAPIKey: "ak",
				BackendModel:  "claude-haiku-4-5",
			},
		},
	}
	reg, err := NewRegistry(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	svc, err := reg.Get("venice-rp")
	if err != nil {
		t.Fatal(err)
	}
	split, ok := svc.(*splitService)
	if !ok {
		t.Fatalf("expected *splitService, got %T", svc)
	}
	if _, ok := split.narrator.(*VeniceService); !ok {
		t.Fatalf("narrator = %T", split.narrator)
	}
	if _, ok := split.backend.(*AnthropicService); !ok {
		t.Fatalf("backend = %T", split.backend)
	}
	info, ok := reg.Info("venice-rp")
	if !ok || info.Vendor != config.VendorVenice || info.Model != "venice-uncensored-role-play" {
		t.Fatalf("catalog should stay narrator-facing: %#v", info)
	}
}

func TestNewRegistry_SameVendorBackendDoesNotSplit(t *testing.T) {
	cfg := &config.Config{
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
	}
	reg, err := NewRegistry(cfg, testLogger())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	svc, err := reg.Get("sonnet")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := svc.(*AnthropicService); !ok {
		t.Fatalf("expected *AnthropicService, got %T", svc)
	}
}

func TestNewRegistry_SplitBackendHTTP(t *testing.T) {
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

	cfg := &config.Config{
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
	}
	reg, err := NewRegistry(cfg, testLogger())
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
