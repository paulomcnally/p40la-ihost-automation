package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/paulomcnally/p40la-ihost-automation/internal/models"
	"github.com/paulomcnally/p40la-ihost-automation/internal/services"
)

// AppsHandlers agrupa los handlers del módulo de apps.
type AppsHandlers struct {
	service *services.AppsService
	auth    *services.AuthService
}

// NewAppsHandlers crea un nuevo AppsHandlers.
func NewAppsHandlers(service *services.AppsService, auth *services.AuthService) *AppsHandlers {
	return &AppsHandlers{service: service, auth: auth}
}

func parseID(r *http.Request, key string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(key), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func mapAppsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrNotFound):
		respondError(w, http.StatusNotFound, "not_found", "Registro no encontrado")
	case errors.Is(err, services.ErrDuplicate):
		respondError(w, http.StatusConflict, "conflict", "Ya existe un registro con ese nombre o identificador")
	case errors.Is(err, services.ErrInvalidJSON):
		respondError(w, http.StatusBadRequest, "invalid_request", "Las credenciales deben ser un objeto JSON válido")
	case errors.Is(err, services.ErrInvalidName):
		respondError(w, http.StatusBadRequest, "invalid_request", "El nombre no puede estar vacío")
	case errors.Is(err, services.ErrInvalidID):
		respondError(w, http.StatusBadRequest, "invalid_request", "El identificador no puede estar vacío")
	case errors.Is(err, services.ErrInvalidCreds):
		respondError(w, http.StatusBadRequest, "invalid_request", "Credenciales no válidas")
	case errors.Is(err, services.ErrPluginNotFound):
		respondError(w, http.StatusNotFound, "plugin_not_found", "Plugin no encontrado")
	default:
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

// ListApps responde con todas las apps.
func (h *AppsHandlers) ListApps(w http.ResponseWriter, r *http.Request) {
	apps, err := h.service.ListApps(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if apps == nil {
		apps = []models.App{}
	}
	respondJSON(w, http.StatusOK, apps)
}

type appRequest struct {
	Name string `json:"name"`
}

// CreateApp crea una nueva app.
func (h *AppsHandlers) CreateApp(w http.ResponseWriter, r *http.Request) {
	var req appRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Cuerpo JSON inválido")
		return
	}
	app, err := h.service.CreateApp(r.Context(), req.Name)
	if err != nil {
		mapAppsError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, app)
}

// UpdateApp actualiza el nombre de una app.
func (h *AppsHandlers) UpdateApp(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r, "id")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid_request", "ID inválido")
		return
	}
	var req appRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Cuerpo JSON inválido")
		return
	}
	app, err := h.service.UpdateApp(r.Context(), id, req.Name)
	if err != nil {
		mapAppsError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, app)
}

// DeleteApp borra una app y sus cuentas.
func (h *AppsHandlers) DeleteApp(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r, "id")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid_request", "ID inválido")
		return
	}
	if err := h.service.DeleteApp(r.Context(), id); err != nil {
		mapAppsError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListAccounts responde con las cuentas de una app.
func (h *AppsHandlers) ListAccounts(w http.ResponseWriter, r *http.Request) {
	appID, ok := parseID(r, "id")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid_request", "ID inválido")
		return
	}
	accounts, err := h.service.ListAccounts(r.Context(), appID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if accounts == nil {
		accounts = []models.Account{}
	}
	respondJSON(w, http.StatusOK, accounts)
}

type accountRequest struct {
	Identifier  string          `json:"identifier"`
	Label       *string         `json:"label"`
	PluginName  *string         `json:"plugin_name,omitempty"`
	Credentials json.RawMessage `json:"credentials"`
}

// CreateAccount crea una cuenta dentro de una app.
func (h *AppsHandlers) CreateAccount(w http.ResponseWriter, r *http.Request) {
	appID, ok := parseID(r, "id")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid_request", "ID inválido")
		return
	}
	var req accountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Cuerpo JSON inválido")
		return
	}
	if len(req.Credentials) == 0 {
		respondError(w, http.StatusBadRequest, "invalid_request", "Credenciales no válidas")
		return
	}
	account, err := h.service.CreateAccount(r.Context(), appID, req.Identifier, req.Label, string(req.Credentials), req.PluginName)
	if err != nil {
		mapAppsError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, account)
}

// UpdateAccount actualiza una cuenta.
func (h *AppsHandlers) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := parseID(r, "accountId")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid_request", "ID inválido")
		return
	}
	var req accountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Cuerpo JSON inválido")
		return
	}
	if len(req.Credentials) == 0 {
		respondError(w, http.StatusBadRequest, "invalid_request", "Credenciales no válidas")
		return
	}
	account, err := h.service.UpdateAccount(r.Context(), accountID, req.Identifier, req.Label, string(req.Credentials))
	if err != nil {
		mapAppsError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, account)
}

// DeleteAccount borra una cuenta.
func (h *AppsHandlers) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := parseID(r, "accountId")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid_request", "ID inválido")
		return
	}
	if err := h.service.DeleteAccount(r.Context(), accountID); err != nil {
		mapAppsError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type accountDetailResponse struct {
	ID             int64   `json:"id"`
	AppID          int64   `json:"app_id"`
	Identifier     string  `json:"identifier"`
	Label          *string `json:"label,omitempty"`
	PluginName     *string `json:"plugin_name,omitempty"`
	HasCredentials bool    `json:"has_credentials"`
}

// GetAccount responde una cuenta sin exponer las credenciales en claro
// (solo indica si existen; el reveal requiere password, SPEC-011).
func (h *AppsHandlers) GetAccount(w http.ResponseWriter, r *http.Request) {
	accountID, ok := parseID(r, "accountId")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid_request", "ID inválido")
		return
	}
	account, err := h.service.AccountWithCredentials(r.Context(), accountID)
	if err != nil {
		mapAppsError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, accountDetailResponse{
		ID:             account.ID,
		AppID:          account.AppID,
		Identifier:     account.Identifier,
		Label:          account.Label,
		PluginName:     account.PluginName,
		HasCredentials: account.Credentials != "" && account.Credentials != "{}",
	})
}

type revealCredentialsRequest struct {
	Password string `json:"password"`
}

// RevealCredentials valida el password del usuario autenticado y, solo si es
// correcto, devuelve las credenciales de la cuenta en claro.
func (h *AppsHandlers) RevealCredentials(w http.ResponseWriter, r *http.Request) {
	accountID, ok := parseID(r, "accountId")
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid_request", "ID inválido")
		return
	}

	user, ok := userFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "Sesión no válida")
		return
	}

	var req revealCredentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Password == "" {
		respondError(w, http.StatusBadRequest, "invalid_request", "Password requerido")
		return
	}

	if !h.auth.ValidatePassword(r.Context(), user.Email, req.Password) {
		respondError(w, http.StatusUnauthorized, "invalid_credentials", "Password incorrecto")
		return
	}

	credentials, err := h.service.CredentialsForAccount(r.Context(), accountID)
	if err != nil {
		mapAppsError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"credentials": credentials})
}
