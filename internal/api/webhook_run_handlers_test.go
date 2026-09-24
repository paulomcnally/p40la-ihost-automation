package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/paulomcnally/p40la-ihost-automation/internal/db"
	"github.com/paulomcnally/p40la-ihost-automation/internal/models"
	"github.com/paulomcnally/p40la-ihost-automation/internal/plugins"
	"github.com/paulomcnally/p40la-ihost-automation/internal/services"
	"github.com/paulomcnally/p40la-ihost-automation/internal/storage"
)

// newRunWebhookTestHandlers levanta el stack completo de webhooks contra una DB
// temporal (con migraciones) y devuelve los handlers + una cuenta con webhook
// configurado (sin plugin) + la api_key válida.
func newRunWebhookTestHandlers(t *testing.T) (*WebhookHandlers, int64, string) {
	t.Helper()

	database, err := db.OpenDB(filepath.Join(t.TempDir(), "app.db"), "../../migrations")
	if err != nil {
		t.Fatalf("abrir db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	settingsStorage := storage.NewSettingsStorage(database)
	appsStorage := storage.NewAppsStorage(database)
	pluginsStorage := storage.NewPluginsStorage(database)

	appSettings := services.NewAppSettingsService(settingsStorage)
	if err := appSettings.SetWebhookConfig(context.Background(), "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6", true); err != nil {
		t.Fatalf("configurar webhooks: %v", err)
	}
	apiKey, _ := appSettings.GetWebhookAPIKey(context.Background())

	appsService := services.NewAppsService(appsStorage, pluginsStorage)
	app, err := appsService.CreateApp(context.Background(), "Test")
	if err != nil {
		t.Fatalf("crear app: %v", err)
	}
	account, err := appsService.CreateAccount(context.Background(), app.ID, "TEST-0001", nil, `{}`, nil)
	if err != nil {
		t.Fatalf("crear cuenta: %v", err)
	}

	url := "http://localhost:9999/webhooks/test"
	if err := pluginsStorage.SetAccountWebhook(context.Background(), models.AccountWebhook{
		AccountID:         account.ID,
		WebhookURL:        &url,
		ScheduleEnabled:   true,
		ScheduleFrequency: "daily",
	}); err != nil {
		t.Fatalf("configurar webhook de cuenta: %v", err)
	}

	registry := plugins.NewRegistry()
	billsService := services.NewBillsService(appsService, registry, pluginsStorage)
	webhookService := services.NewWebhookService(appSettings, billsService, pluginsStorage)
	handlers := NewWebhookHandlers(appSettings, webhookService)

	return handlers, account.ID, apiKey
}

func TestRunWebhookUnauthorized(t *testing.T) {
	h, accountID, _ := newRunWebhookTestHandlers(t)

	// Sin header X-Webhook-Key
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+strconv.FormatInt(accountID,10)+"/webhook:run", nil)
	req.SetPathValue("accountId", strconv.FormatInt(accountID, 10))
	rec := httptest.NewRecorder()
	h.RunWebhook(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sin header: status %d, want 401; body %s", rec.Code, rec.Body.String())
	}

	// Con key incorrecta
	req = httptest.NewRequest(http.MethodPost, "/api/accounts/"+strconv.FormatInt(accountID,10)+"/webhook:run", nil)
	req.SetPathValue("accountId", strconv.FormatInt(accountID, 10))
	req.Header.Set("X-Webhook-Key", "clave-incorrecta")
	rec = httptest.NewRecorder()
	h.RunWebhook(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("key incorrecta: status %d, want 401; body %s", rec.Code, rec.Body.String())
	}
}

func TestRunWebhookWithValidKeyMapsDomainErrors(t *testing.T) {
	h, accountID, apiKey := newRunWebhookTestHandlers(t)

	// Cuenta sin plugin asociado: RunJob falla con ErrNoPluginAssigned → 400
	req := httptest.NewRequest(http.MethodPost, "/api/accounts/"+strconv.FormatInt(accountID,10)+"/webhook:run", nil)
	req.SetPathValue("accountId", strconv.FormatInt(accountID, 10))
	req.Header.Set("X-Webhook-Key", apiKey)
	rec := httptest.NewRecorder()
	h.RunWebhook(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("sin plugin: status %d, want 400; body %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parsear respuesta: %v", err)
	}
	if resp["error"] != "invalid_request" {
		t.Errorf("error = %q, want invalid_request", resp["error"])
	}

	// Cuenta inexistente: RunJob falla con ErrWebhookNoURL → 400
	req = httptest.NewRequest(http.MethodPost, "/api/accounts/99999/webhook:run", nil)
	req.SetPathValue("accountId", "99999")
	req.Header.Set("X-Webhook-Key", apiKey)
	rec = httptest.NewRecorder()
	h.RunWebhook(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("cuenta inexistente: status %d, want 400; body %s", rec.Code, rec.Body.String())
	}
}

func TestRunWebhookInvalidID(t *testing.T) {
	h, _, apiKey := newRunWebhookTestHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/api/accounts/abc/webhook:run", nil)
	req.SetPathValue("accountId", "abc")
	req.Header.Set("X-Webhook-Key", apiKey)
	rec := httptest.NewRecorder()
	h.RunWebhook(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("ID inválido: status %d, want 400", rec.Code)
	}
}