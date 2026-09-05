// Package auth identifies API callers and authorizes gamestate access.
package auth

import (
	"context"
	"crypto/subtle"

	"github.com/jwebster45206/story-engine/internal/config"
)

type contextKey struct{}

// Principal is the authenticated caller attached to the request context.
type Principal struct {
	KeyHash string
}

// WithPrincipal returns a child context carrying p.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, p)
}

// PrincipalFrom returns the Principal stored by auth middleware.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(contextKey{}).(Principal)
	return p, ok
}

// CanAccess reports whether p may operate on a gamestate owned by ownerHash.
// Empty owner hashes are not owned by any key.
func (p Principal) CanAccess(ownerHash string) bool {
	if ownerHash == "" || p.KeyHash == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(p.KeyHash), []byte(ownerHash)) == 1
}

// Lookup returns a Principal if presented is a configured API key.
func Lookup(cfg *config.Config, presented string) (Principal, bool) {
	canon, err := config.CanonicalKey(presented)
	if err != nil {
		return Principal{}, false
	}
	h := config.HashKey(canon)
	hb := []byte(h)
	for _, kh := range cfg.APIKeyHashes {
		if subtle.ConstantTimeCompare(hb, []byte(kh)) == 1 {
			return Principal{KeyHash: h}, true
		}
	}
	return Principal{}, false
}
