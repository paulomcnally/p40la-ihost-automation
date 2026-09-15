package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/paulomcnally/p40la-ihost-automation/internal/models"
	"github.com/paulomcnally/p40la-ihost-automation/internal/services"
)

// WebhookHandlers agrupa los handlers de configuración y entrega de webhooks.
type WebhookHandlers struct {
	settings *services.AppSettingsService
	service  *services.WebhookService
}

// NewWebhookHandlers crea un nuevo WebhookHandlers.
func NewWebhookHandlers(settings *services.AppSettingsService, service *services.WebhookService) *WebhookHandlers {
	return &WebhookHandlers{settings: settings, service: service}
}

func mapWebhookError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "Registro no encontrado")
	case errors.Is(err, services.ErrWebhookDisabled):
		respondError(w, http.StatusBadRequest, "invalid_request", "Los webhooks están deshabilitados")
	case errors.Is(err, services.ErrWebhookNoKey):
		respondError(w, http.StatusBadRequest, "invalid_request", "No hay api_key de webhook configurada")
	case errors.Is(err, services.ErrWebhookNoURL):
		respondError(w, http.StatusBadRequest, "invalid_request", "La cuenta no tiene webhook_url configurada")
	case errors.Is(err, services.ErrNoPluginAssigned):
		respondError(w, http.StatusBadRequest, "invalid_request", "La cuenta no tiene plugin asociado")
	default:
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

// GetSettingsWebhook responde la configuración global de webhooks.
func (h *WebhookHandlers) GetSettingsWebhook(w http.ResponseWriter, r *http.Request) {
	apiKey, err := h.settings.GetWebhookAPIKey(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"api_key": apiKey,
		"enabled": h.settings.IsWebhookEnabled(r.Context()),
	})
}

type setSettingsWebhookRequest struct {
	APIKey  *string `json:"api_key"`
	Enabled *bool   `json:"enabled"`
}

// SetSettingsWebhook actualiza la api_key y/o el toggle maestro de webhooks.
func (h *WebhookHandlers) SetSettingsWebhook(w http.ResponseWriter, r *http.Request) {
	var req setSettingsWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Cuerpo JSON inválido")
		return
	}
	apiKey := ""
	if req.APIKey != nil {
		apiKey = *req.APIKey
	}
	enabled := h.settings.IsWebhookEnabled(r.Context())
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	if err := h.settings.SetWebhookConfig(r.Context(), apiKey, enabled); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"api_key": apiKey,
		"enabled": enabled,
	})
}

// GetAccountWebhook responde la configuración webhook de una cuenta.
func (h *WebhookHandlers) GetAccountWebhook(w http.ResponseWriter, r *http.Request) {
	accountID, ok := parseID(r, "accountId")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid_request", "ID inválido")
		return
	}
	cfg, err := h.service.GetAccountWebhook(r.Context(), accountID)
	if err != nil {
		mapWebhookError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, cfg)
}

type setAccountWebhookRequest struct {
	WebhookURL        *string `json:"webhook_url"`
	ScheduleTime      *string `json:"schedule_time"`
	ScheduleEnabled   *bool   `json:"schedule_enabled"`
	ScheduleFrequency *string `json:"schedule_frequency"`
	ScheduleInterval  *int    `json:"schedule_interval"`
	ScheduleDays      *string `json:"schedule_days"`
}

// SetAccountWebhook actualiza la configuración webhook de una cuenta.
func (h *WebhookHandlers) SetAccountWebhook(w http.ResponseWriter, r *http.Request) {
	accountID, ok := parseID(r, "accountId")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid_request", "ID inválido")
		return
	}
	var req setAccountWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Cuerpo JSON inválido")
		return
	}

	current, err := h.service.GetAccountWebhook(r.Context(), accountID)
	if err != nil {
		mapWebhookError(w, err)
		return
	}

	cfg := models.AccountWebhook{AccountID: accountID}
	cfg.WebhookURL = current.WebhookURL
	cfg.ScheduleTime = current.ScheduleTime
	cfg.ScheduleEnabled = current.ScheduleEnabled
	cfg.ScheduleFrequency = current.ScheduleFrequency
	cfg.ScheduleInterval = current.ScheduleInterval
	cfg.ScheduleDays = current.ScheduleDays
	if req.WebhookURL != nil {
		cfg.WebhookURL = req.WebhookURL
	}
	if req.ScheduleTime != nil {
		cfg.ScheduleTime = req.ScheduleTime
	}
	if req.ScheduleEnabled != nil {
		cfg.ScheduleEnabled = *req.ScheduleEnabled
	}
	if req.ScheduleFrequency != nil {
		cfg.ScheduleFrequency = *req.ScheduleFrequency
	}
	if req.ScheduleInterval != nil {
		cfg.ScheduleInterval = req.ScheduleInterval
	}
	if req.ScheduleDays != nil {
		cfg.ScheduleDays = req.ScheduleDays
	}

	updated, err := h.service.SetAccountWebhook(r.Context(), cfg)
	if err != nil {
		mapWebhookError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, updated)
}

// TestWebhook ejecuta el job de inmediato para una cuenta.
func (h *WebhookHandlers) TestWebhook(w http.ResponseWriter, r *http.Request) {
	accountID, ok := parseID(r, "accountId")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid_request", "ID inválido")
		return
	}
	delivered, failed, err := h.service.RunJob(r.Context(), accountID)
	if err != nil {
		mapWebhookError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]int{"delivered": delivered, "failed": failed})
}

// ListWebhookLogs responde los últimos eventos de entrega de una cuenta.
func (h *WebhookHandlers) ListWebhookLogs(w http.ResponseWriter, r *http.Request) {
	accountID, ok := parseID(r, "accountId")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid_request", "ID inválido")
		return
	}
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			offset = n
		}
	}
	logs, err := h.service.ListWebhookLogs(r.Context(), accountID, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if logs == nil {
		logs = []models.WebhookLog{}
	}
	respondJSON(w, http.StatusOK, logs)
}
