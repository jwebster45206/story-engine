package main

import (
	"crypto/ecdsa"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/internal/auth"
)

type bearerTransport struct {
	base      http.RoundTripper
	key       *ecdsa.PrivateKey
	principal uuid.UUID
}

func (t *bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	// TODO: replace local minting with a token from an auth service.
	tok, err := auth.Mint(t.key, t.principal, time.Hour)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}
