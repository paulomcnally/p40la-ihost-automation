package services

import (
	"context"
	"testing"
	"time"

	"github.com/paulomcnally/p40la-ihost-automation/internal/config"
	"github.com/paulomcnally/p40la-ihost-automation/internal/db"
	"github.com/paulomcnally/p40la-ihost-automation/internal/storage"
)

func newTestAuth(t *testing.T) *AuthService {
	t.Helper()
	database, err := db.OpenDB(":memory:", "../../migrations")
	if err != nil {
		t.Fatalf("abrir db de prueba: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	cfg := &config.Config{
		BcryptCost:      10,
		SessionDuration: 1 * time.Hour,
		CookieName:      "p40la_ihost_automation_session",
		SecureCookie:    false,
	}
	return NewAuthService(storage.NewUserStorage(database), storage.NewSettingsStorage(database), cfg)
}

func TestCreateFirstUser(t *testing.T) {
	auth := newTestAuth(t)
	ctx := context.Background()

	user, cookie, err := auth.CreateFirstUser(ctx, "admin@example.com", "Password123", "Password123")
	if err != nil {
		t.Fatalf("crear primer usuario: %v", err)
	}
	if user.Email != "admin@example.com" {
		t.Errorf("email esperado admin@example.com, got %s", user.Email)
	}
	if cookie == nil || cookie.Value == "" {
		t.Error("se esperaba cookie de sesión")
	}

	completed, _ := auth.IsSetupComplete(ctx)
	if !completed {
		t.Error("setup debería estar completo")
	}

	// No debe permitir segundo usuario.
	if _, _, err := auth.CreateFirstUser(ctx, "otro@example.com", "Password123", "Password123"); err == nil {
		t.Error("debería rechazar segundo usuario")
	}
}

func TestCreateFirstUserValidation(t *testing.T) {
	auth := newTestAuth(t)
	ctx := context.Background()

	cases := []struct {
		name     string
		email    string
		password string
		confirm  string
	}{
		{"email vacío", "", "Password123", "Password123"},
		{"email inválido", "no-es-email", "Password123", "Password123"},
		{"contraseña corta", "a@b.com", "Pass1", "Pass1"},
		{"sin número", "a@b.com", "Password", "Password"},
		{"sin letra", "a@b.com", "12345678", "12345678"},
		{"confirmación distinta", "a@b.com", "Password123", "Password124"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := auth.CreateFirstUser(ctx, tc.email, tc.password, tc.confirm); err == nil {
				t.Errorf("%s: se esperaba error", tc.name)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	auth := newTestAuth(t)
	ctx := context.Background()

	if _, _, err := auth.CreateFirstUser(ctx, "admin@example.com", "Password123", "Password123"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	user, cookie, err := auth.Login(ctx, "admin@example.com", "Password123", false)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if user.Email != "admin@example.com" {
		t.Errorf("email esperado admin@example.com, got %s", user.Email)
	}
	if cookie == nil {
		t.Error("se esperaba cookie")
	}

	if _, _, err := auth.Login(ctx, "admin@example.com", "mal", false); err == nil {
		t.Error("login con contraseña incorrecta debería fallar")
	}
	if _, _, err := auth.Login(ctx, "otro@example.com", "Password123", false); err == nil {
		t.Error("login con email inexistente debería fallar")
	}
}

func TestSession(t *testing.T) {
	auth := newTestAuth(t)
	ctx := context.Background()

	if _, _, err := auth.CreateFirstUser(ctx, "admin@example.com", "Password123", "Password123"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, cookie, err := auth.Login(ctx, "admin@example.com", "Password123", false)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	user, err := auth.ValidateSession(ctx, cookie.Value)
	if err != nil {
		t.Fatalf("validar sesión: %v", err)
	}
	if user.Email != "admin@example.com" {
		t.Errorf("email esperado admin@example.com, got %s", user.Email)
	}

	// Cookie inválida.
	if _, err := auth.ValidateSession(ctx, "invalid"); err == nil {
		t.Error("cookie inválida debería fallar")
	}

	// Logout limpia cookie.
	logoutCookie := auth.Logout()
	if logoutCookie.MaxAge != -1 {
		t.Errorf("logout cookie MaxAge esperado -1, got %d", logoutCookie.MaxAge)
	}
}

// TestValidatePassword verifica la validación de password para aprobaciones
// sensibles (reveal de credenciales, SPEC-011).
func TestValidatePassword(t *testing.T) {
	auth := newTestAuth(t)
	ctx := context.Background()

	if _, _, err := auth.CreateFirstUser(ctx, "admin@example.com", "Password123", "Password123"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if !auth.ValidatePassword(ctx, "admin@example.com", "Password123") {
		t.Error("ValidatePassword con password correcto debería devolver true")
	}
	if auth.ValidatePassword(ctx, "admin@example.com", "mal") {
		t.Error("ValidatePassword con password incorrecto debería devolver false")
	}
	if auth.ValidatePassword(ctx, "noexiste@example.com", "Password123") {
		t.Error("ValidatePassword con usuario inexistente debería devolver false")
	}
}

// TestCookieName verifica que la cookie usa el nombre configurado y no colisiona
// con la de otros proyectos (ej. p40la-ihost, que usa "session").
func TestCookieName(t *testing.T) {
	auth := newTestAuth(t)
	ctx := context.Background()

	if got := auth.CookieName(); got != "p40la_ihost_automation_session" {
		t.Errorf("CookieName esperado p40la_ihost_automation_session, got %s", got)
	}

	_, cookie, err := auth.CreateFirstUser(ctx, "admin@example.com", "Password123", "Password123")
	if err != nil {
		t.Fatalf("crear primer usuario: %v", err)
	}
	if cookie.Name != auth.CookieName() {
		t.Errorf("cookie de setup esperada %s, got %s", auth.CookieName(), cookie.Name)
	}

	_, loginCookie, err := auth.Login(ctx, "admin@example.com", "Password123", false)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if loginCookie.Name != auth.CookieName() {
		t.Errorf("cookie de login esperada %s, got %s", auth.CookieName(), loginCookie.Name)
	}

	if logoutCookie := auth.Logout(); logoutCookie.Name != auth.CookieName() {
		t.Errorf("cookie de logout esperada %s, got %s", auth.CookieName(), logoutCookie.Name)
	}
}

// TestCookieNameCustom verifica que un nombre de cookie custom se respeta.
func TestCookieNameCustom(t *testing.T) {
	database, err := db.OpenDB(":memory:", "../../migrations")
	if err != nil {
		t.Fatalf("abrir db de prueba: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	cfg := &config.Config{
		BcryptCost:      10,
		SessionDuration: 1 * time.Hour,
		CookieName:      "mi_sesion_custom",
		SecureCookie:    false,
	}
	auth := NewAuthService(storage.NewUserStorage(database), storage.NewSettingsStorage(database), cfg)

	_, cookie, err := auth.CreateFirstUser(context.Background(), "admin@example.com", "Password123", "Password123")
	if err != nil {
		t.Fatalf("crear primer usuario: %v", err)
	}
	if cookie.Name != "mi_sesion_custom" {
		t.Errorf("cookie esperada mi_sesion_custom, got %s", cookie.Name)
	}
}
