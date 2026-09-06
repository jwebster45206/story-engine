package runner

import (
	"crypto/ecdsa"
	"net/http"

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
	tok, err := auth.NewToken(t.principal)
	if err != nil {
		return nil, err
	}
	raw, err := tok.SignedString(t.key)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+raw)
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

func (r *Runner) UseJWT(key *ecdsa.PrivateKey, principal uuid.UUID) {
	r.Client.Transport = &bearerTransport{
		base:      r.Client.Transport,
		key:       key,
		principal: principal,
	}
}
