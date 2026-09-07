package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"
	"uuid"

	"github.com/golang-jwt/jwt/v5"
)

func testKeys(t *testing.T) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return priv, &priv.PublicKey
}

func privatePEM(t *testing.T, key *ecdsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
}

func publicPEM(t *testing.T, key *ecdsa.PublicKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
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

	valid, err := Token(priv, id)
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
				Subject:   uuid.Nil().String(),
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

func TestToken_NilPrincipal(t *testing.T) {
	priv, _ := testKeys(t)
	if _, err := Token(priv, uuid.Nil()); err == nil {
		t.Fatal("expected error")
	}
}

func TestTokenTTL(t *testing.T) {
	priv, pub := testKeys(t)
	id := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	ttl := 8 * time.Hour
	raw, err := TokenTTL(priv, id, ttl)
	if err != nil {
		t.Fatal(err)
	}
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodES256.Alg()}))
	tok, err := parser.ParseWithClaims(raw, &jwt.RegisteredClaims{}, func(*jwt.Token) (any, error) {
		return pub, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	claims, ok := tok.Claims.(*jwt.RegisteredClaims)
	if !ok || !tok.Valid {
		t.Fatal("invalid token")
	}
	got := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time)
	if got < ttl-time.Second || got > ttl+time.Second {
		t.Fatalf("ttl = %v, want %v", got, ttl)
	}
}

func TestLoadKeys(t *testing.T) {
	priv, pub := testKeys(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, privateKeyFile), privatePEM(t, priv), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, publicKeyFile), publicPEM(t, pub), 0o600); err != nil {
		t.Fatal(err)
	}
	gotPriv, err := LoadPrivateKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	gotPub, err := LoadPublicKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !gotPriv.Equal(priv) {
		t.Fatal("loaded private key does not match")
	}
	if !gotPub.Equal(pub) {
		t.Fatal("loaded public key does not match")
	}
}

func TestLoadPublicKey_Missing(t *testing.T) {
	if _, err := LoadPublicKey(t.TempDir()); err == nil {
		t.Fatal("expected error")
	}
}
