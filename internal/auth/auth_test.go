package auth

import (
	"testing"
	"uuid"

	"github.com/jwebster45206/story-engine/internal/config"
)

const (
	testAPIKey  = "22222222-2222-4222-8222-222222222222"
	testAPIKeyB = "44444444-4444-4444-8444-444444444444"
)

func TestCanAccess(t *testing.T) {
	hashA := config.HashKey(uuid.MustParse(testAPIKey))
	hashB := config.HashKey(uuid.MustParse(testAPIKeyB))
	api := Principal{KeyHash: hashA}

	if !api.CanAccess(hashA) {
		t.Fatal("owner should access own hash")
	}
	if api.CanAccess(hashB) {
		t.Fatal("key must not access another hash")
	}
	if api.CanAccess("") {
		t.Fatal("empty owner hash is not owned by any key")
	}
}

func TestLookup(t *testing.T) {
	cfg := &config.Config{
		APIKeyHashes: []string{config.HashKey(uuid.MustParse(testAPIKey))},
	}

	p, ok := Lookup(cfg, testAPIKey)
	if !ok {
		t.Fatal("expected configured key to match")
	}
	if p.KeyHash != config.HashKey(uuid.MustParse(testAPIKey)) {
		t.Fatalf("hash = %q", p.KeyHash)
	}

	if _, ok := Lookup(cfg, testAPIKeyB); ok {
		t.Fatal("unknown key must not match")
	}
	if _, ok := Lookup(cfg, "not-a-uuid"); ok {
		t.Fatal("invalid token must not match")
	}
}
