package handlers

import (
	"net/http"
	"uuid"

	"github.com/jwebster45206/story-engine/internal/config"
	"github.com/jwebster45206/story-engine/internal/middleware"
)

const (
	testAPIKeyA  = "22222222-2222-4222-8222-222222222222"
	testAPIKeyB  = "44444444-4444-4444-8444-444444444444"
	testAdminKey = "11111111-1111-4111-8111-111111111111"
)

func testKeyHash(key string) string {
	return config.HashKey(uuid.MustParse(key))
}

func testAuthConfig() *config.Config {
	return &config.Config{
		APIKeyHashes:   []string{testKeyHash(testAPIKeyA), testKeyHash(testAPIKeyB)},
		AdminKeyHashes: []string{testKeyHash(testAdminKey)},
	}
}

func serveKey(h http.Handler, w http.ResponseWriter, r *http.Request, key string) {
	r.Header.Set("Authorization", "Bearer "+key)
	middleware.APIKey(testAuthConfig(), h).ServeHTTP(w, r)
}

func serveAdmin(h http.Handler, w http.ResponseWriter, r *http.Request) {
	serveKey(h, w, r, testAdminKey)
}
