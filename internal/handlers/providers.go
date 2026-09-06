package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/jwebster45206/story-engine/internal/httperror"
)

type ProvidersHandler struct {
	catalog ProviderCatalog
	logger  *slog.Logger
}

func NewProvidersHandler(logger *slog.Logger, catalog ProviderCatalog) *ProvidersHandler {
	return &ProvidersHandler{
		logger:  logger,
		catalog: catalog,
	}
}

type providersResponse struct {
	Default   string `json:"default"`
	Providers []any  `json:"providers"`
}

func (h *ProvidersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		httperror.Write(w, h.logger, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	names := h.catalog.Names()
	providers := make([]any, 0, len(names))
	for _, name := range names {
		info, ok := h.catalog.Info(name)
		if !ok {
			continue
		}
		providers = append(providers, info)
	}

	resp := providersResponse{
		Default:   h.catalog.Default(),
		Providers: providers,
	}
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("Failed to encode providers response", "error", err)
	}
}
