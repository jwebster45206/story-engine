package auth

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/internal/httperror"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

// RequirePrincipal returns the caller Principal.
// Writes 401 when none is present.
func RequirePrincipal(w http.ResponseWriter, r *http.Request, logger *slog.Logger) (Principal, bool) {
	p, ok := PrincipalFrom(r.Context())
	if !ok {
		httperror.Write(w, logger, http.StatusUnauthorized, "unauthorized")
		return Principal{}, false
	}
	return p, true
}

// AuthorizeGame reports whether the caller may access the gamestate.
// Writes 401 when no principal is present, 404 when missing or not owned, 500 on storage errors.
func AuthorizeGame(w http.ResponseWriter, r *http.Request, store storage.Storage, id uuid.UUID, logger *slog.Logger) bool {
	p, ok := RequirePrincipal(w, r, logger)
	if !ok {
		return false
	}
	hash, found, err := store.GetOwnerKeyHash(r.Context(), id)
	if err != nil {
		logger.Error("Failed to load game state owner hash", "error", err, "id", id.String())
		httperror.Write(w, logger, http.StatusInternalServerError, "Failed to load game state")
		return false
	}
	if !found || !p.CanAccess(hash) {
		httperror.Write(w, logger, http.StatusNotFound, "Game state not found")
		return false
	}
	return true
}
