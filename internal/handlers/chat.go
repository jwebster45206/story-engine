package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jwebster45206/roleplay-agent/pkg/chat"
)

func ChatHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Only allow POST method
	if r.Method != http.MethodPost {
		slog.Warn("Method not allowed for chat endpoint",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr)

		w.WriteHeader(http.StatusMethodNotAllowed)
		response := chat.ChatResponse{
			Error: "Method not allowed. Only POST is supported.",
		}

		if err := json.NewEncoder(w).Encode(response); err != nil {
			slog.Error("Error encoding chat error response",
				"error", err,
				"method", r.Method,
				"path", r.URL.Path)
		}
		return
	}

	slog.Info("Chat endpoint accessed",
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr)

	// Return not implemented error
	w.WriteHeader(http.StatusNotImplemented)
	response := chat.ChatResponse{
		Error: "Chat functionality is not yet implemented.",
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.Error("Error encoding chat response",
			"error", err,
			"method", r.Method,
			"path", r.URL.Path)
		return
	}
}
