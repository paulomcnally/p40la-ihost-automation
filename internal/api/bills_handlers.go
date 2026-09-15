package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/paulomcnally/p40la-ihost-automation/internal/models"
	"github.com/paulomcnally/p40la-ihost-automation/internal/plugins"
	"github.com/paulomcnally/p40la-ihost-automation/internal/services"
)

// BillsHandlers agrupa los handlers de facturas por cuenta.
type BillsHandlers struct {
	service *services.BillsService
}

// NewBillsHandlers crea un nuevo BillsHandlers.
func NewBillsHandlers(service *services.BillsService) *BillsHandlers {
	return &BillsHandlers{service: service}
}

func mapBillsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "Registro no encontrado")
	case errors.Is(err, services.ErrPluginNotFound):
		respondError(w, http.StatusNotFound, "not_found", "Plugin no encontrado")
	case errors.Is(err, services.ErrNoPluginAssigned):
		respondError(w, http.StatusBadRequest, "invalid_request", "La cuenta no tiene plugin asociado")
	case errors.Is(err, services.ErrInvalidCreds):
		respondError(w, http.StatusBadRequest, "invalid_request", "Credenciales no válidas")
	case errors.Is(err, plugins.ErrAuthFailed):
		respondError(w, http.StatusBadGateway, "auth_failed", err.Error())
	case errors.Is(err, plugins.ErrAuthExpired):
		respondError(w, http.StatusUnauthorized, "auth_expired", err.Error())
	case errors.Is(err, plugins.ErrServiceNotFound):
		respondError(w, http.StatusBadGateway, "service_not_found", err.Error())
	case errors.Is(err, plugins.ErrAPISchemaChanged):
		respondError(w, http.StatusBadGateway, "api_changed", err.Error())
	case errors.Is(err, plugins.ErrUpstream):
		respondError(w, http.StatusBadGateway, "upstream_error", err.Error())
	default:
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

// GetAccountPlugin responde el plugin asociado a una cuenta.
func (h *BillsHandlers) GetAccountPlugin(w http.ResponseWriter, r *http.Request) {
	accountID, ok := parseID(r, "accountId")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid_request", "ID inválido")
		return
	}
	plugin, err := h.service.AccountPlugin(r.Context(), accountID)
	if err != nil {
		mapBillsError(w, err)
		return
	}
	if plugin == nil {
		respondJSON(w, http.StatusOK, map[string]any{"plugin_name": nil})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"plugin_name": plugin.Name,
		"version":     plugin.Version,
	})
}

type setAccountPluginRequest struct {
	PluginName *string `json:"plugin_name"`
}

// SetAccountPlugin asocia o desasocia un plugin a una cuenta.
func (h *BillsHandlers) SetAccountPlugin(w http.ResponseWriter, r *http.Request) {
	accountID, ok := parseID(r, "accountId")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid_request", "ID inválido")
		return
	}
	var req setAccountPluginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Cuerpo JSON inválido")
		return
	}
	plugin, err := h.service.SetAccountPlugin(r.Context(), accountID, req.PluginName)
	if err != nil {
		mapBillsError(w, err)
		return
	}
	if plugin == nil {
		respondJSON(w, http.StatusOK, map[string]any{"plugin_name": nil})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"plugin_name": plugin.Name,
		"version":     plugin.Version,
	})
}

// FetchBills ejecuta la consulta de facturas de una cuenta y la persiste.
func (h *BillsHandlers) FetchBills(w http.ResponseWriter, r *http.Request) {
	accountID, ok := parseID(r, "accountId")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid_request", "ID inválido")
		return
	}
	rec, bills, err := h.service.FetchBills(r.Context(), accountID)
	if err != nil {
		mapBillsError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{
		"id":             rec.ID,
		"account_id":     rec.AccountID,
		"plugin_name":    rec.PluginName,
		"plugin_version": rec.PluginVersion,
		"status":         rec.Status,
		"bills":          bills,
	})
}

// ListBills responde el historial de consultas de facturas de una cuenta con paginación.
// Soporta ?limit, ?offset y ?at (timestamp ISO) para saltar a la página de una ejecución.
func (h *BillsHandlers) ListBills(w http.ResponseWriter, r *http.Request) {
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
	if atStr := r.URL.Query().Get("at"); atStr != "" {
		if at, err := time.Parse(time.RFC3339, atStr); err == nil {
			resolved, err := h.service.ResolveOffsetFor(r.Context(), accountID, at, limit)
			if err == nil {
				offset = resolved
			}
		}
	}
	bills, err := h.service.ListBills(r.Context(), accountID, limit, offset)
	if err != nil {
		mapBillsError(w, err)
		return
	}
	if bills == nil {
		bills = []models.BillRecord{}
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"items":  bills,
		"offset": offset,
	})
}
