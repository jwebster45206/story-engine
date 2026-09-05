// Package httperror writes JSON API error responses.
package httperror

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Response is the JSON body for HTTP errors.
type Response struct {
	Error string `json:"error"`
}

// Write sends status and a JSON Response. 401 also sets WWW-Authenticate: Bearer.
func Write(w http.ResponseWriter, logger *slog.Logger, status int, msg string) {
	if logger == nil {
		logger = slog.Default()
	}
	w.Header().Set("Content-Type", "application/json")
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(Response{Error: msg}); err != nil {
		logger.Error("failed to encode error response", "error", err)
	}
}
