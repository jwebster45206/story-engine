// Package auth identifies API callers from ES256 JWTs.
package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"uuid"

	"github.com/golang-jwt/jwt/v5"
)

const (
	tokenTTL       = time.Hour
	privateKeyFile = "auth-key.pem"
	publicKeyFile  = "auth-key.pub.pem"
)

type contextKey struct{}

// Principal is the authenticated caller attached to the request context.
type Principal struct {
	ID uuid.UUID
}

// WithPrincipal returns a child context carrying p.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, p)
}

// PrincipalFrom returns the Principal stored by JWT middleware.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(contextKey{}).(Principal)
	return p, ok
}

// ParseBearer verifies an ES256 JWT and returns the principal from sub.
// The nil UUID is rejected.
func ParseBearer(pub *ecdsa.PublicKey, tokenString string) (Principal, error) {
	if pub == nil {
		return Principal{}, fmt.Errorf("missing public key")
	}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodES256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	tok, err := parser.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(t *jwt.Token) (any, error) {
		return pub, nil
	})
	if err != nil {
		return Principal{}, err
	}
	claims, ok := tok.Claims.(*jwt.RegisteredClaims)
	if !ok || !tok.Valid {
		return Principal{}, fmt.Errorf("invalid token")
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return Principal{}, fmt.Errorf("sub must be a UUID")
	}
	if id == uuid.Nil() {
		return Principal{}, fmt.Errorf("nil UUID is not allowed")
	}
	return Principal{ID: id}, nil
}

// Token signs an ES256 JWT for principal with the default TTL. Local-issuer helper.
func Token(key *ecdsa.PrivateKey, principal uuid.UUID) (string, error) {
	return TokenTTL(key, principal, tokenTTL)
}

// TokenTTL signs an ES256 JWT for principal that expires after ttl.
func TokenTTL(key *ecdsa.PrivateKey, principal uuid.UUID, ttl time.Duration) (string, error) {
	if key == nil {
		return "", fmt.Errorf("missing private key")
	}
	if principal == uuid.Nil() {
		return "", fmt.Errorf("nil UUID is not allowed")
	}
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{
		Subject:   principal.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
	})
	return tok.SignedString(key)
}

// LoadPrivateKey reads auth-key.pem from dir.
func LoadPrivateKey(dir string) (*ecdsa.PrivateKey, error) {
	path := filepath.Join(dir, privateKeyFile)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	key, err := parsePrivateKey(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return key, nil
}

// LoadPublicKey reads auth-key.pub.pem from dir.
func LoadPublicKey(dir string) (*ecdsa.PublicKey, error) {
	path := filepath.Join(dir, publicKeyFile)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	key, err := parsePublicKey(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return key, nil
}

func parsePrivateKey(pemBytes []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("must be a PEM-encoded private key")
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	if key.Curve != elliptic.P256() {
		return nil, fmt.Errorf("must use P-256")
	}
	return key, nil
}

func parsePublicKey(pemBytes []byte) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("must be a PEM-encoded public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	ec, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("must be an ECDSA public key")
	}
	if ec.Curve != elliptic.P256() {
		return nil, fmt.Errorf("must use P-256")
	}
	return ec, nil
}

type bearerTransport struct {
	base  http.RoundTripper
	token string
}

// Bearer returns a RoundTripper that sets Authorization: Bearer on each request.
func Bearer(token string) http.RoundTripper {
	return &bearerTransport{base: http.DefaultTransport, token: token}
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}
