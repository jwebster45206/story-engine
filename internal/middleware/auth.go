package middleware

import (
	"crypto/ecdsa"
	"log/slog"
	"net/http"
	"strings"

	"github.com/jwebster45206/story-engine/internal/auth"
	"github.com/jwebster45206/story-engine/internal/httperror"
)

// JWT requires a valid ES256 Bearer token.
func JWT(pub *ecdsa.PublicKey, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string
		if scheme, rest, ok := strings.Cut(r.Header.Get("Authorization"), " "); ok && strings.EqualFold(scheme, "Bearer") {
			token = strings.TrimSpace(rest)
		}
		p, err := auth.ParseBearer(pub, token)
		if err != nil {
			httperror.Write(w, slog.Default(), http.StatusUnauthorized, "unauthorized")
			return
		}

		next.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), p)))
	})
}
