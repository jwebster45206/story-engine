package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/internal/services"
)

func TestProvidersHandler_List(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	handler := NewProvidersHandler(logger, testCatalog())

	req := httptest.NewRequest(http.MethodGet, "/v1/providers", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}

	body := rr.Body.String()
	if strings.Contains(strings.ToLower(body), "api_key") || strings.Contains(body, "sk-") {
		t.Fatalf("response must not leak keys: %s", body)
	}

	var resp struct {
		Default   string                   `json:"default"`
		Providers []services.ProviderInfo  `json:"providers"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Default != "foo" {
		t.Fatalf("default = %q", resp.Default)
	}
	if len(resp.Providers) != 2 {
		t.Fatalf("providers len = %d", len(resp.Providers))
	}
}
