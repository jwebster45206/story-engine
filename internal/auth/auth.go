// Package auth identifies API callers from ES256 JWTs.
package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const tokenTTL = time.Hour

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
	if id == uuid.Nil {
		return Principal{}, fmt.Errorf("nil UUID is not allowed")
	}
	return Principal{ID: id}, nil
}

// NewToken signs an ES256 JWT for principal. Local-issuer helper.
func NewToken(principal uuid.UUID) (*jwt.Token, error) {
	if principal == uuid.Nil {
		return nil, fmt.Errorf("nil UUID is not allowed")
	}
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{
		Subject:   principal.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
	}), nil
}

// ParseES256PrivateKey parses a PEM-encoded EC private key (SEC1 or PKCS#8).
func ParseES256PrivateKey(pemStr string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("must be a PEM-encoded private key")
	}
	var key *ecdsa.PrivateKey
	if parsed, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		key = parsed
	} else {
		raw, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("private key: %w", err)
		}
		ec, ok := raw.(*ecdsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key must be ECDSA")
		}
		key = ec
	}
	if key.Curve != elliptic.P256() {
		return nil, fmt.Errorf("private key must use P-256")
	}
	return key, nil
}
