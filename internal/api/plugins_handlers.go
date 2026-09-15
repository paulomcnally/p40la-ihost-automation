package api

import (
	"errors"
	"net/http"

	"github.com/paulomcnally/p40la-ihost-automation/internal/models"
	"github.com/paulomcnally/p40la-ihost-automation/internal/services"
)

// PluginsHandlers agrupa los handlers del catálogo de plugins.
type PluginsHandlers struct {
	service *services.BillsService
}

// NewPluginsHandlers crea un nuevo PluginsHandlers.
func NewPluginsHandlers(service *services.BillsService) *PluginsHandlers {
	return &PluginsHandlers{service: service}
}

// ListPlugins responde con el catálogo de plugins (con versión y fuente).
func (h *PluginsHandlers) ListPlugins(w http.ResponseWriter, r *http.Request) {
	plugins, err := h.service.ListPlugins(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if plugins == nil {
		plugins = []models.Plugin{}
	}
	respondJSON(w, http.StatusOK, plugins)
}

// GetPluginSchema responde con el schema de credenciales de un plugin.
func (h *PluginsHandlers) GetPluginSchema(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "Nombre de plugin requerido")
		return
	}
	schema, err := h.service.PluginSchema(r.Context(), name)
	if err != nil {
		if errors.Is(err, services.ErrPluginNotFound) {
			respondError(w, http.StatusNotFound, "not_found", "Plugin no encontrado")
			return
		}
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, schema)
}
