package services

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"

	"github.com/paulomcnally/p40la-ihost-automation/internal/config"
	"github.com/paulomcnally/p40la-ihost-automation/internal/models"
	"github.com/paulomcnally/p40la-ihost-automation/internal/storage"
)

const (
	secretSettingKey = "auth_cookie_secret"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// AuthService contiene la lógica de autenticación y sesiones.
type AuthService struct {
	users    *storage.UserStorage
	settings *storage.SettingsStorage
	cfg      *config.Config
}

// NewAuthService crea el servicio de autenticación.
func NewAuthService(users *storage.UserStorage, settings *storage.SettingsStorage, cfg *config.Config) *AuthService {
	return &AuthService{
		users:    users,
		settings: settings,
		cfg:      cfg,
	}
}

// IsSetupComplete indica si ya existe al menos un usuario.
func (s *AuthService) IsSetupComplete(ctx context.Context) (bool, error) {
	count, err := s.users.Count(ctx)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// CreateFirstUser crea el único usuario administrador y devuelve la sesión.
func (s *AuthService) CreateFirstUser(ctx context.Context, email, password, passwordConfirm string) (*models.User, *http.Cookie, error) {
	if err := validateEmail(email); err != nil {
		return nil, nil, err
	}
	if err := validatePasswordPair(password, passwordConfirm); err != nil {
		return nil, nil, err
	}

	exists, err := s.IsSetupComplete(ctx)
	if err != nil {
		return nil, nil, err
	}
	if exists {
		return nil, nil, fmt.Errorf("ya existe un usuario configurado")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.cfg.BcryptCost)
	if err != nil {
		return nil, nil, fmt.Errorf("hashear contraseña: %w", err)
	}

	user, err := s.users.Create(ctx, email, string(hash))
	if err != nil {
		return nil, nil, fmt.Errorf("crear usuario: %w", err)
	}

	cookie, err := s.createSessionCookie(ctx, user.Email, false)
	if err != nil {
		return nil, nil, err
	}

	return user, cookie, nil
}

// Login valida credenciales y devuelve la sesión.
func (s *AuthService) Login(ctx context.Context, email, password string, remember bool) (*models.User, *http.Cookie, error) {
	if err := validateEmail(email); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(password) == "" {
		return nil, nil, fmt.Errorf("contraseña requerida")
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, nil, fmt.Errorf("buscar usuario: %w", err)
	}
	if user == nil {
		// Mensaje genérico para no revelar existencia del email.
		return nil, nil, fmt.Errorf("credenciales inválidas")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, nil, fmt.Errorf("credenciales inválidas")
	}

	cookie, err := s.createSessionCookie(ctx, user.Email, remember)
	if err != nil {
		return nil, nil, err
	}

	return user, cookie, nil
}

// CookieName devuelve el nombre de la cookie de sesión configurado.
func (s *AuthService) CookieName() string {
	return s.cfg.CookieName
}

// Logout devuelve una cookie expirada.
func (s *AuthService) Logout() *http.Cookie {
	return &http.Cookie{
		Name:     s.cfg.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   s.cfg.SecureCookie,
	}
}

// ValidatePassword verifica el password de un usuario autenticado sin crear
// una sesión nueva. Se usa para aprobaciones sensibles (ej. revelar
// credenciales de una cuenta, SPEC-011).
func (s *AuthService) ValidatePassword(ctx context.Context, email, password string) bool {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil || user == nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) == nil
}

// ValidateSession verifica una cookie de sesión y devuelve el usuario asociado.
func (s *AuthService) ValidateSession(ctx context.Context, cookieValue string) (*models.User, error) {
	email, expires, err := s.parseSessionCookie(cookieValue)
	if err != nil {
		return nil, err
	}
	if time.Now().After(expires) {
		return nil, fmt.Errorf("sesión expirada")
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("usuario no encontrado")
	}
	return user, nil
}

func (s *AuthService) createSessionCookie(ctx context.Context, email string, remember bool) (*http.Cookie, error) {
	duration := s.cfg.SessionDuration
	if remember {
		duration = 30 * 24 * time.Hour
	}
	expires := time.Now().Add(duration)

	value, err := s.signSessionCookie(email, expires)
	if err != nil {
		return nil, err
	}

	return &http.Cookie{
		Name:     s.cfg.CookieName,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(duration.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   s.cfg.SecureCookie,
	}, nil
}

func (s *AuthService) signSessionCookie(email string, expires time.Time) (string, error) {
	secret, err := s.getOrCreateSecret(context.Background())
	if err != nil {
		return "", err
	}

	emailB64 := base64.RawURLEncoding.EncodeToString([]byte(email))
	timestamp := strconv.FormatInt(expires.Unix(), 10)
	payload := emailB64 + "|" + timestamp

	h := hmac.New(sha256.New, secret)
	h.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return payload + "|" + sig, nil
}

func (s *AuthService) parseSessionCookie(value string) (string, time.Time, error) {
	parts := strings.Split(value, "|")
	if len(parts) != 3 {
		return "", time.Time{}, fmt.Errorf("cookie inválida")
	}

	emailBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", time.Time{}, fmt.Errorf("cookie inválida")
	}
	email := string(emailBytes)

	ts, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("cookie inválida")
	}
	expires := time.Unix(ts, 0)

	expected, err := s.signSessionCookie(email, expires)
	if err != nil {
		return "", time.Time{}, err
	}
	if subtle.ConstantTimeCompare([]byte(expected), []byte(value)) != 1 {
		return "", time.Time{}, fmt.Errorf("cookie inválida")
	}

	return email, expires, nil
}

func (s *AuthService) getOrCreateSecret(ctx context.Context) ([]byte, error) {
	if s.cfg.SessionSecret != "" {
		return []byte(s.cfg.SessionSecret), nil
	}

	value, err := s.settings.Get(ctx, secretSettingKey)
	if err != nil {
		return nil, err
	}
	if value != "" {
		return []byte(value), nil
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("generar secreto: %w", err)
	}
	newSecret := base64.RawURLEncoding.EncodeToString(b)
	if err := s.settings.Set(ctx, secretSettingKey, newSecret); err != nil {
		return nil, err
	}
	return []byte(newSecret), nil
}

func validateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email requerido")
	}
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("email inválido")
	}
	return nil
}

func validatePasswordPair(password, confirm string) error {
	if err := validatePassword(password); err != nil {
		return err
	}
	if password != confirm {
		return fmt.Errorf("las contraseñas no coinciden")
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("la contraseña debe tener al menos 8 caracteres")
	}
	var hasLetter, hasNumber bool
	for _, r := range password {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsNumber(r) {
			hasNumber = true
		}
	}
	if !hasLetter || !hasNumber {
		return fmt.Errorf("la contraseña debe contener al menos una letra y un número")
	}
	return nil
}
