package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/paulomcnally/p40la-ihost-automation/internal/services"
)

// BuildRouter configura y devuelve el enrutador principal.
func BuildRouter(handler *Handler, auth *services.AuthService, apps *AppsHandlers, plugins *PluginsHandlers, bills *BillsHandlers, webhooks *WebhookHandlers, staticDir string) http.Handler {
	mux := http.NewServeMux()

	authMiddleware := AuthMiddleware(auth)

	// APIs públicas
	mux.HandleFunc("GET /health", handler.Health)
	mux.HandleFunc("GET /api/setup-status", handler.SetupStatus)
	mux.HandleFunc("POST /api/setup", handler.Setup)
	mux.HandleFunc("POST /api/login", handler.Login)
	mux.HandleFunc("POST /api/logout", handler.Logout)
	mux.Handle("GET /api/me", authMiddleware(http.HandlerFunc(handler.Me)))

	// APIs de configuraciones
	mux.Handle("GET /api/settings", authMiddleware(http.HandlerFunc(handler.settings.GetSettings)))
	mux.Handle("POST /api/settings/language", authMiddleware(http.HandlerFunc(handler.settings.SetLanguage)))

	// APIs de apps y cuentas
	mux.Handle("GET /api/apps", authMiddleware(http.HandlerFunc(apps.ListApps)))
	mux.Handle("POST /api/apps", authMiddleware(http.HandlerFunc(apps.CreateApp)))
	mux.Handle("PUT /api/apps/{id}", authMiddleware(http.HandlerFunc(apps.UpdateApp)))
	mux.Handle("DELETE /api/apps/{id}", authMiddleware(http.HandlerFunc(apps.DeleteApp)))
	mux.Handle("GET /api/apps/{id}/accounts", authMiddleware(http.HandlerFunc(apps.ListAccounts)))
	mux.Handle("POST /api/apps/{id}/accounts", authMiddleware(http.HandlerFunc(apps.CreateAccount)))
	mux.Handle("GET /api/apps/{id}/accounts/{accountId}", authMiddleware(http.HandlerFunc(apps.GetAccount)))
	mux.Handle("PUT /api/apps/{id}/accounts/{accountId}", authMiddleware(http.HandlerFunc(apps.UpdateAccount)))
	mux.Handle("DELETE /api/apps/{id}/accounts/{accountId}", authMiddleware(http.HandlerFunc(apps.DeleteAccount)))
	mux.Handle("POST /api/accounts/{accountId}/credentials:reveal", authMiddleware(http.HandlerFunc(apps.RevealCredentials)))

	// APIs de plugins y facturas
	mux.Handle("GET /api/plugins", authMiddleware(http.HandlerFunc(plugins.ListPlugins)))
	mux.Handle("GET /api/plugins/{name}/schema", authMiddleware(http.HandlerFunc(plugins.GetPluginSchema)))
	mux.Handle("GET /api/accounts/{accountId}/plugin", authMiddleware(http.HandlerFunc(bills.GetAccountPlugin)))
	mux.Handle("PUT /api/accounts/{accountId}/plugin", authMiddleware(http.HandlerFunc(bills.SetAccountPlugin)))
	mux.Handle("POST /api/accounts/{accountId}/bills:fetch", authMiddleware(http.HandlerFunc(bills.FetchBills)))
	mux.Handle("GET /api/accounts/{accountId}/bills", authMiddleware(http.HandlerFunc(bills.ListBills)))

	// APIs de webhooks y jobs
	mux.Handle("GET /api/settings/webhook", authMiddleware(http.HandlerFunc(webhooks.GetSettingsWebhook)))
	mux.Handle("PUT /api/settings/webhook", authMiddleware(http.HandlerFunc(webhooks.SetSettingsWebhook)))
	mux.Handle("GET /api/accounts/{accountId}/webhook", authMiddleware(http.HandlerFunc(webhooks.GetAccountWebhook)))
	mux.Handle("PUT /api/accounts/{accountId}/webhook", authMiddleware(http.HandlerFunc(webhooks.SetAccountWebhook)))
	mux.Handle("POST /api/accounts/{accountId}/webhook:test", authMiddleware(http.HandlerFunc(webhooks.TestWebhook)))
	mux.Handle("GET /api/accounts/{accountId}/webhook/logs", authMiddleware(http.HandlerFunc(webhooks.ListWebhookLogs)))

	// Assets estáticos (JS/CSS bundles del build de Vite)
	assetsFS := http.FileServer(http.Dir(filepath.Join(staticDir, "assets")))
	mux.Handle("GET /assets/{file...}", http.StripPrefix("/assets/", assetsFS))

	// i18n JSON files
	i18nFS := http.FileServer(http.Dir(filepath.Join(staticDir, "i18n")))
	mux.Handle("GET /i18n/{file...}", http.StripPrefix("/i18n/", i18nFS))

	// SPA fallback — sirve index.html para cualquier ruta no-API no-asset
	mux.Handle("GET /{path...}", spaHandler(staticDir))

	return mux
}

func spaHandler(staticDir string) http.HandlerFunc {
	indexFile := filepath.Join(staticDir, "index.html")

	return func(w http.ResponseWriter, r *http.Request) {
		staticPath := filepath.Join(staticDir, r.URL.Path)
		if info, err := os.Stat(staticPath); err == nil && !info.IsDir() {
			http.ServeFile(w, r, staticPath)
			return
		}
		http.ServeFile(w, r, indexFile)
	}
}
