package middleware

import (
	"crypto/ecdsa"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jwebster45206/story-engine/internal/auth"
	"github.com/jwebster45206/story-engine/internal/httperror"
)

// JWT requires a valid ES256 Bearer token on all paths except /health.
func JWT(pub *ecdsa.PublicKey, next http.Handler) http.Handler {
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

		p, err := auth.ParseBearer(pub, token)
		if err != nil {
			httperror.Write(w, slog.Default(), http.StatusUnauthorized, "unauthorized")
			return
		}

		next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), p)))
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
