package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/paulomcnally/p40la-ihost-automation/internal/services"
)

// Handler agrupa los handlers HTTP de la aplicación.
type Handler struct {
	auth     *services.AuthService
	settings *SettingsHandlers
}

// NewHandler crea un nuevo Handler.
func NewHandler(auth *services.AuthService, settings *SettingsHandlers) *Handler {
	return &Handler{
		auth:     auth,
		settings: settings,
	}
}

type setupStatusResponse struct {
	SetupCompleted bool `json:"setup_completed"`
}

// SetupStatus responde si el sistema ya fue configurado.
func (h *Handler) SetupStatus(w http.ResponseWriter, r *http.Request) {
	completed, err := h.auth.IsSetupComplete(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal_error", "Error al verificar estado")
		return
	}
	respondJSON(w, http.StatusOK, setupStatusResponse{SetupCompleted: completed})
}

type setupRequest struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"password_confirm"`
}

type authResponse struct {
	UserID int64  `json:"user_id,omitempty"`
	Email  string `json:"email"`
}

// Setup crea el primer usuario y establece la sesión.
func (h *Handler) Setup(w http.ResponseWriter, r *http.Request) {
	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Cuerpo JSON inválido")
		return
	}

	user, cookie, err := h.auth.CreateFirstUser(r.Context(), req.Email, req.Password, req.PasswordConfirm)
	if err != nil {
		if err.Error() == "ya existe un usuario configurado" {
			respondError(w, http.StatusConflict, "already_setup", "El sistema ya fue configurado")
			return
		}
		respondError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	http.SetCookie(w, cookie)
	respondJSON(w, http.StatusCreated, authResponse{UserID: user.ID, Email: user.Email})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

// Login autentica a un usuario existente.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid_request", "Cuerpo JSON inválido")
		return
	}

	user, cookie, err := h.auth.Login(r.Context(), req.Email, req.Password, req.Remember)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid_credentials", "Email o contraseña incorrectos")
		return
	}

	http.SetCookie(w, cookie)
	respondJSON(w, http.StatusOK, authResponse{Email: user.Email})
}

// Logout cierra la sesión actual.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, h.auth.Logout())
	respondJSON(w, http.StatusOK, map[string]string{"message": "Sesión cerrada"})
}

// Me devuelve información del usuario autenticado.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromContext(r.Context())
	if !ok {
		respondError(w, http.StatusUnauthorized, "unauthorized", "Sesión no válida")
		return
	}
	respondJSON(w, http.StatusOK, authResponse{Email: user.Email})
}

// Health responde el estado del servicio.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func respondError(w http.ResponseWriter, status int, code, message string) {
	respondJSON(w, status, errorResponse{Error: code, Message: message})
}
