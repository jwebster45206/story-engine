package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/jwebster45206/story-engine/internal/middleware"
	"github.com/jwebster45206/story-engine/pkg/state"
	"github.com/jwebster45206/story-engine/pkg/storage"
)

func writeJSONError(w http.ResponseWriter, logger *slog.Logger, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(ErrorResponse{Error: msg}); err != nil {
		logger.Error("Failed to encode error response", "error", err)
	}
}

func encodeGameState(w http.ResponseWriter, gs *state.GameState) error {
	out := *gs
	out.OwnerKeyHash = ""
	return json.NewEncoder(w).Encode(&out)
}

// loadAuthorizedGame returns the gamestate if the caller may access it.
// Writes 401 when no principal is present, 404 when missing or not owned, 500 on storage errors.
func loadAuthorizedGame(w http.ResponseWriter, r *http.Request, store storage.Storage, id uuid.UUID, logger *slog.Logger) (*state.GameState, bool) {
	p, ok := middleware.PrincipalFrom(r.Context())
	if !ok {
		writeJSONError(w, logger, http.StatusUnauthorized, "unauthorized")
		return nil, false
	}
	gs, err := store.LoadGameState(r.Context(), id)
	if err != nil {
		logger.Error("Failed to load game state", "error", err, "id", id.String())
		writeJSONError(w, logger, http.StatusInternalServerError, "Failed to load game state")
		return nil, false
	}
	if gs == nil || !p.CanAccess(gs.OwnerKeyHash) {
		writeJSONError(w, logger, http.StatusNotFound, "Game state not found")
		return nil, false
	}
	return gs, true
}

func requirePrincipal(w http.ResponseWriter, r *http.Request, logger *slog.Logger) (middleware.Principal, bool) {
	p, ok := middleware.PrincipalFrom(r.Context())
	if !ok {
		writeJSONError(w, logger, http.StatusUnauthorized, "unauthorized")
		return middleware.Principal{}, false
	}
	return p, true
}
