package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"uuid"

	"github.com/jwebster45206/story-engine/internal/config"
)

const (
	testAPIKey = "22222222-2222-4222-8222-222222222222"
)

func testAuthConfig() *config.Config {
	return &config.Config{
		APIKeyHashes: []string{config.HashKey(uuid.MustParse(testAPIKey))},
	}
}

func TestAPIKey_HealthExempt(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := APIKey(testAuthConfig(), ok)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestAPIKey_MissingHeader(t *testing.T) {
	h := APIKey(testAuthConfig(), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not run")
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/scenarios", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	if got := rr.Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
}

func TestAPIKey_BadScheme(t *testing.T) {
	h := APIKey(testAuthConfig(), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not run")
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/scenarios", nil)
	req.Header.Set("Authorization", "Basic abcdefghijklmnop")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestAPIKey_WrongKey(t *testing.T) {
	h := APIKey(testAuthConfig(), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not run")
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/scenarios", nil)
	req.Header.Set("Authorization", "Bearer totally-wrong-key")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "unauthorized" {
		t.Fatalf("error = %q", body["error"])
	}
}

func TestAPIKey_APIKeyPrincipal(t *testing.T) {
	h := APIKey(testAuthConfig(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := PrincipalFrom(r.Context())
		if !ok {
			t.Fatal("missing principal")
		}
		if p.KeyHash != config.HashKey(uuid.MustParse(testAPIKey)) {
			t.Fatalf("hash = %q", p.KeyHash)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/scenarios", nil)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
}

func TestAPIKey_CanonicalForm(t *testing.T) {
	h := APIKey(testAuthConfig(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := PrincipalFrom(r.Context()); !ok {
			t.Fatal("expected api_key principal")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/scenarios", nil)
	req.Header.Set("Authorization", "Bearer "+strings.ToUpper(testAPIKey))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
}

func TestPrincipalCanAccess(t *testing.T) {
	hashA := config.HashKey(uuid.MustParse(testAPIKey))
	hashB := config.HashKey(uuid.MustParse("44444444-4444-4444-8444-444444444444"))
	api := Principal{KeyHash: hashA}

	if !api.CanAccess(hashA) {
		t.Fatal("owner should access own hash")
	}
	if api.CanAccess(hashB) {
		t.Fatal("api_key must not access another hash")
	}
	if api.CanAccess("") {
		t.Fatal("empty owner hash is not owned by any key")
	}
}
