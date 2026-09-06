package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/internal/auth"
	"github.com/jwebster45206/story-engine/internal/config"
)

func testJWTConfig(t *testing.T) *config.Config {
	t.Helper()
	pubPEM, err := os.ReadFile(filepath.Join("..", "auth", "testdata", "ec-p256.pub.pem"))
	if err != nil {
		t.Fatal(err)
	}
	pub, err := config.ParseES256PublicKey(string(pubPEM))
	if err != nil {
		t.Fatal(err)
	}
	return &config.Config{JWTPublicKey: pub}
}

func testToken(t *testing.T, id uuid.UUID) string {
	t.Helper()
	privPEM, err := os.ReadFile(filepath.Join("..", "auth", "testdata", "ec-p256.pem"))
	if err != nil {
		t.Fatal(err)
	}
	priv, err := auth.ParseES256PrivateKey(string(privPEM))
	if err != nil {
		t.Fatal(err)
	}
	tok, err := auth.NewToken(id)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestJWT_HealthExempt(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := JWT(testJWTConfig(t), ok)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestJWT_MissingHeader(t *testing.T) {
	h := JWT(testJWTConfig(t), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
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

func TestJWT_BadScheme(t *testing.T) {
	h := JWT(testJWTConfig(t), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
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

func TestJWT_BadToken(t *testing.T) {
	h := JWT(testJWTConfig(t), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not run")
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/scenarios", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
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

func TestJWT_Principal(t *testing.T) {
	id := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	h := JWT(testJWTConfig(t), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFrom(r.Context())
		if !ok {
			t.Fatal("missing principal")
		}
		if p.ID != id {
			t.Fatalf("id = %v", p.ID)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/scenarios", nil)
	req.Header.Set("Authorization", "Bearer "+testToken(t, id))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
}
