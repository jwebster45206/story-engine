package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jwebster45206/story-engine/internal/config"
)

const (
	testAPIKey    = "22222222-2222-4222-8222-222222222222"
	testAdminKey  = "11111111-1111-4111-8111-111111111111"
	testSharedKey = "33333333-3333-4333-8333-333333333333"
)

func testAuthConfig() *config.Config {
	return &config.Config{
		APIKeyHashes:   []string{config.HashKey(testAPIKey)},
		AdminKeyHashes: []string{config.HashKey(testAdminKey)},
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
		if p.Admin {
			t.Fatal("api_key must not be admin")
		}
		if p.KeyHash != config.HashKey(testAPIKey) {
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

func TestAPIKey_AdminPrincipal(t *testing.T) {
	h := APIKey(testAuthConfig(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := PrincipalFrom(r.Context())
		if !ok || !p.Admin {
			t.Fatalf("principal admin=%v ok=%v", p.Admin, ok)
		}
		if p.KeyHash != config.HashKey(testAdminKey) {
			t.Fatalf("hash = %q", p.KeyHash)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/scenarios", nil)
	req.Header.Set("Authorization", "bearer "+testAdminKey)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
}

func TestAPIKey_CanonicalForm(t *testing.T) {
	h := APIKey(testAuthConfig(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := PrincipalFrom(r.Context())
		if !ok || p.Admin {
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

func TestAPIKey_KeyInBothListsIsAdmin(t *testing.T) {
	shared := testSharedKey
	cfg := &config.Config{
		APIKeyHashes:   []string{config.HashKey(shared)},
		AdminKeyHashes: []string{config.HashKey(shared)},
	}
	h := APIKey(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := PrincipalFrom(r.Context())
		if !ok || !p.Admin {
			t.Fatalf("want admin principal, got admin=%v ok=%v", p.Admin, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/providers", nil)
	req.Header.Set("Authorization", "Bearer "+shared)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
}

func TestPrincipalCanAccess(t *testing.T) {
	hashA := config.HashKey("aaaaaaaaaaaaaaaa")
	hashB := config.HashKey("bbbbbbbbbbbbbbbb")
	api := Principal{Admin: false, KeyHash: hashA}
	admin := Principal{Admin: true, KeyHash: hashB}

	if !api.CanAccess(hashA) {
		t.Fatal("owner should access own hash")
	}
	if api.CanAccess(hashB) {
		t.Fatal("api_key must not access another hash")
	}
	if api.CanAccess("") {
		t.Fatal("empty owner hash is not owned by api_key")
	}
	if !admin.CanAccess(hashA) || !admin.CanAccess("") {
		t.Fatal("admin should access any hash including empty")
	}
}
