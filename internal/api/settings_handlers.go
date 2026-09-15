package api

import (
	"encoding/json"
	"net/http"

	"github.com/paulomcnally/p40la-ihost-automation/internal/services"
)

// SettingsHandlers agrupa los handlers de configuraciones.
type SettingsHandlers struct {
	service *services.AppSettingsService
}

// NewSettingsHandlers crea un nuevo SettingsHandlers.
func NewSettingsHandlers(service *services.AppSettingsService) *SettingsHandlers {
	return &SettingsHandlers{service: service}
}

// GetSettings responde con todas las configuraciones.
func (h *SettingsHandlers) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.GetAll(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, settings)
}

type setLanguageRequest struct {
	Language string `json:"language"`
}

// SetLanguage cambia el idioma de la aplicación.
func (h *SettingsHandlers) SetLanguage(w http.ResponseWriter, r *http.Request) {
	var req setLanguageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Cuerpo JSON inválido")
		return
	}
	if err := h.service.SetLanguage(r.Context(), req.Language); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"language": req.Language})
}
