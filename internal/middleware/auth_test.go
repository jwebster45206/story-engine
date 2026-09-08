package middleware

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"uuid"

	"github.com/jwebster45206/story-engine/internal/auth"
)

func testKeyPair(t *testing.T) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return priv, &priv.PublicKey
}

func TestJWT_HealthExempt(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	_, pub := testKeyPair(t)
	h := JWT(pub, ok)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestJWT_MetricsExempt(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	_, pub := testKeyPair(t)
	h := JWT(pub, ok)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}

func TestJWT_Unauthorized(t *testing.T) {
	_, pub := testKeyPair(t)
	h := JWT(pub, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("next should not run")
	}))
	for _, header := range []string{"", "Basic abcdefghijklmnop", "Bearer not-a-jwt"} {
		req := httptest.NewRequest(http.MethodGet, "/v1/scenarios", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("header %q: status = %d, want 401", header, rr.Code)
		}
	}
}

func TestJWT_Principal(t *testing.T) {
	id := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	priv, pub := testKeyPair(t)
	raw, err := auth.Token(priv, id)
	if err != nil {
		t.Fatal(err)
	}
	h := JWT(pub, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	req.Header.Set("Authorization", "Bearer "+raw)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
}
