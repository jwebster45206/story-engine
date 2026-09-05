package middleware

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jwebster45206/story-engine/internal/config"
	"github.com/jwebster45206/story-engine/internal/httperror"
)

type contextKey struct{}

// Principal is the authenticated caller attached to the request context.
type Principal struct {
	KeyHash string
}

func withPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, p)
}

// PrincipalFrom returns the Principal stored by APIKey middleware.
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

// APIKey requires a configured Bearer token on all paths except /health.
func APIKey(cfg *config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			httperror.Write(w, slog.Default(), http.StatusUnauthorized, "unauthorized")
			return
		}

		p, ok := lookupPrincipal(cfg, token)
		if !ok {
			httperror.Write(w, slog.Default(), http.StatusUnauthorized, "unauthorized")
			return
		}

		next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), p)))
	})
}

func bearerToken(header string) (string, bool) {
	scheme, rest, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token := strings.TrimSpace(rest)
	if token == "" {
		return "", false
	}
	return token, true
}

func lookupPrincipal(cfg *config.Config, presented string) (Principal, bool) {
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
