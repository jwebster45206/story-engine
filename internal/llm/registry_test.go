package llm

import (
	"io"
	"log/slog"
	"testing"

	"github.com/jwebster45206/story-engine/internal/config"
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
