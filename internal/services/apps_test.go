package services

import (
	"context"
	"errors"
	"testing"

	"github.com/paulomcnally/p40la-ihost-automation/internal/db"
	"github.com/paulomcnally/p40la-ihost-automation/internal/storage"
)

func newTestAppsService(t *testing.T) *AppsService {
	t.Helper()
	database, err := db.OpenDB(t.TempDir()+"/test.db", "../../migrations")
	if err != nil {
		t.Fatalf("abrir base de datos de prueba: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	appsStorage := storage.NewAppsStorage(database)
	pluginsStorage := storage.NewPluginsStorage(database)
	return NewAppsService(appsStorage, pluginsStorage)
}

func TestCreateAccountPluginName(t *testing.T) {
	ctx := context.Background()
	svc := newTestAppsService(t)

	app, err := svc.CreateApp(ctx, "App de prueba")
	if err != nil {
		t.Fatalf("crear app: %v", err)
	}
	creds := `{"username":"demo","password":"secret"}`

	t.Run("sin plugin", func(t *testing.T) {
		account, err := svc.CreateAccount(ctx, app.ID, "SVC-1", nil, creds, nil)
		if err != nil {
			t.Fatalf("crear cuenta sin plugin: %v", err)
		}
		if account.PluginName != nil {
			t.Errorf("PluginName = %v, se esperaba nil", *account.PluginName)
		}
	})

	t.Run("con plugin válido del catálogo", func(t *testing.T) {
		if err := svc.plugins.UpsertPlugin(ctx, "demo.nicaragua", "1.0.0", "https://demo.ni", nil, "active"); err != nil {
			t.Fatalf("insertar plugin: %v", err)
		}
		account, err := svc.CreateAccount(ctx, app.ID, "SVC-2", nil, creds, ptrStr("demo.nicaragua"))
		if err != nil {
			t.Fatalf("crear cuenta con plugin: %v", err)
		}
		if account.PluginName == nil || *account.PluginName != "demo.nicaragua" {
			t.Errorf("PluginName = %v, se esperaba demo.nicaragua", account.PluginName)
		}
	})

	t.Run("con plugin inexistente", func(t *testing.T) {
		_, err := svc.CreateAccount(ctx, app.ID, "SVC-3", nil, creds, ptrStr("no.existe"))
		if !errors.Is(err, ErrPluginNotFound) {
			t.Errorf("se esperaba ErrPluginNotFound, got %v", err)
		}
	})

	t.Run("con plugin_name vacío se trata como sin plugin", func(t *testing.T) {
		account, err := svc.CreateAccount(ctx, app.ID, "SVC-4", nil, creds, ptrStr("  "))
		if err != nil {
			t.Fatalf("crear cuenta con plugin vacío: %v", err)
		}
		if account.PluginName != nil {
			t.Errorf("PluginName = %v, se esperaba nil", *account.PluginName)
		}
	})
}

func ptrStr(s string) *string { return &s }