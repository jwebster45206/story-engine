package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/internal/config"
)

func testKeys(t *testing.T) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	t.Helper()
	dir := filepath.Join("testdata")
	privPEM, err := os.ReadFile(filepath.Join(dir, "ec-p256.pem"))
	if err != nil {
		t.Fatal(err)
	}
	pubPEM, err := os.ReadFile(filepath.Join(dir, "ec-p256.pub.pem"))
	if err != nil {
		t.Fatal(err)
	}
	priv, err := ParseES256PrivateKey(string(privPEM))
	if err != nil {
		t.Fatal(err)
	}
	pub, err := config.ParseES256PublicKey(string(pubPEM))
	if err != nil {
		t.Fatal(err)
	}
	return priv, pub
}

func signClaims(t *testing.T, method jwt.SigningMethod, key any, claims jwt.RegisteredClaims) string {
	t.Helper()
	raw, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestParseBearer(t *testing.T) {
	priv, pub := testKeys(t)
	id := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	now := time.Now()
	validClaims := jwt.RegisteredClaims{
		Subject:   id.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}

	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	header, err := json.Marshal(map[string]string{"alg": "none", "typ": "JWT"})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"sub": id.String(),
		"exp": now.Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	noneTok := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + "."

	tok, err := NewToken(id)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		token   string
		want    uuid.UUID
		wantErr bool
	}{
		{name: "valid", token: valid, want: id},
		{
			name: "expired",
			token: signClaims(t, jwt.SigningMethodES256, priv, jwt.RegisteredClaims{
				Subject:   id.String(),
				IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
				ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
			}),
			wantErr: true,
		},
		{
			name: "missing exp",
			token: signClaims(t, jwt.SigningMethodES256, priv, jwt.RegisteredClaims{
				Subject:  id.String(),
				IssuedAt: jwt.NewNumericDate(now),
			}),
			wantErr: true,
		},
		{
			name: "nil subject",
			token: signClaims(t, jwt.SigningMethodES256, priv, jwt.RegisteredClaims{
				Subject:   uuid.Nil.String(),
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			}),
			wantErr: true,
		},
		{
			name:    "hs256",
			token:   signClaims(t, jwt.SigningMethodHS256, []byte("not-the-ec-key"), validClaims),
			wantErr: true,
		},
		{name: "alg none", token: noneTok, wantErr: true},
		{
			name:    "wrong key",
			token:   signClaims(t, jwt.SigningMethodES256, other, validClaims),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParseBearer(pub, tt.token)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if p.ID != tt.want {
				t.Fatalf("id = %v, want %v", p.ID, tt.want)
			}
		})
	}
}

func TestNewToken_NilPrincipal(t *testing.T) {
	if _, err := NewToken(uuid.Nil); err == nil {
		t.Fatal("expected error")
	}
}
