package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/internal/httperror"
	"github.com/jwebster45206/story-engine/internal/middleware"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

func encodeGameState(w http.ResponseWriter, gs *state.GameState) error {
	out := *gs
	out.OwnerKeyHash = ""
	return json.NewEncoder(w).Encode(&out)
}

// authorizeGame reports whether the caller may access the gamestate.
// Writes 401 when no principal is present, 404 when missing or not owned, 500 on storage errors.
func authorizeGame(w http.ResponseWriter, r *http.Request, store storage.Storage, id uuid.UUID, logger *slog.Logger) bool {
	p, ok := middleware.PrincipalFrom(r.Context())
	if !ok {
		httperror.Write(w, logger, http.StatusUnauthorized, "unauthorized")
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

// loadAuthorizedGame returns the gamestate if the caller may access it.
// Writes 401 when no principal is present, 404 when missing or not owned, 500 on storage errors.
func loadAuthorizedGame(w http.ResponseWriter, r *http.Request, store storage.Storage, id uuid.UUID, logger *slog.Logger) (*state.GameState, bool) {
	if !authorizeGame(w, r, store, id, logger) {
		return nil, false
	}
	gs, err := store.LoadGameState(r.Context(), id)
	if err != nil {
		logger.Error("Failed to load game state", "error", err, "id", id.String())
		httperror.Write(w, logger, http.StatusInternalServerError, "Failed to load game state")
		return nil, false
	}
	if gs == nil {
		httperror.Write(w, logger, http.StatusNotFound, "Game state not found")
		return nil, false
	}
	return gs, true
}

func requirePrincipal(w http.ResponseWriter, r *http.Request, logger *slog.Logger) (middleware.Principal, bool) {
	p, ok := middleware.PrincipalFrom(r.Context())
	if !ok {
		httperror.Write(w, logger, http.StatusUnauthorized, "unauthorized")
		return middleware.Principal{}, false
	}
	return p, true
}
