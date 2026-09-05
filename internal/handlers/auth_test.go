package handlers

import (
	"net/http"

	"github.com/jwebster45206/story-engine/internal/middleware"
)

const (
	testOwnerHashA = "aaa111ownerhashaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testOwnerHashB = "bbb222ownerhashbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func withAdmin(r *http.Request) *http.Request {
	return r.WithContext(middleware.WithPrincipal(r.Context(), middleware.Principal{
		Admin:   true,
		KeyHash: "test-admin-hash",
	}))
}

func withAPIKey(r *http.Request, keyHash string) *http.Request {
	return r.WithContext(middleware.WithPrincipal(r.Context(), middleware.Principal{
		Admin:   false,
		KeyHash: keyHash,
	}))
}

func serveAdmin(h http.Handler, w http.ResponseWriter, r *http.Request) {
	h.ServeHTTP(w, withAdmin(r))
}
