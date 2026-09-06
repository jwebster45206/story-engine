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

func TestParseBearer_OK(t *testing.T) {
	priv, pub := testKeys(t)
	id := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	tok, err := Mint(priv, id, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ParseBearer(pub, tok)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != id {
		t.Fatalf("id = %v, want %v", p.ID, id)
	}
}

func TestParseBearer_Expired(t *testing.T) {
	priv, pub := testKeys(t)
	id := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	now := time.Now().Add(-2 * time.Hour)
	raw, err := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{
		Subject:   id.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}).SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseBearer(pub, raw); err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestParseBearer_MissingExp(t *testing.T) {
	priv, pub := testKeys(t)
	id := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	raw, err := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{
		Subject:  id.String(),
		IssuedAt: jwt.NewNumericDate(time.Now()),
	}).SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseBearer(pub, raw); err == nil {
		t.Fatal("expected error when exp is missing")
	}
}

func TestParseBearer_NilSubject(t *testing.T) {
	priv, pub := testKeys(t)
	raw, err := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{
		Subject:   uuid.Nil.String(),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseBearer(pub, raw); err == nil {
		t.Fatal("expected error for nil UUID sub")
	}
}

func TestParseBearer_RejectHS256(t *testing.T) {
	_, pub := testKeys(t)
	id := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   id.String(),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}).SignedString([]byte("not-the-ec-key"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseBearer(pub, raw); err == nil {
		t.Fatal("expected error for HS256")
	}
}

func TestParseBearer_RejectNone(t *testing.T) {
	_, pub := testKeys(t)
	id := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	header, err := json.Marshal(map[string]string{"alg": "none", "typ": "JWT"})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"sub": id.String(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	tok := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + "."
	if _, err := ParseBearer(pub, tok); err == nil {
		t.Fatal("expected error for alg none")
	}
}

func TestParseBearer_WrongKey(t *testing.T) {
	_, pub := testKeys(t)
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := uuid.MustParse("22222222-2222-4222-8222-222222222222")
	tok, err := Mint(other, id, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseBearer(pub, tok); err == nil {
		t.Fatal("expected error for wrong key")
	}
}

func TestMint_RejectsNilPrincipal(t *testing.T) {
	priv, _ := testKeys(t)
	if _, err := Mint(priv, uuid.Nil, time.Hour); err == nil {
		t.Fatal("expected error")
	}
}
