package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/paulomcnally/p40la-ihost-automation/internal/api"
	"github.com/paulomcnally/p40la-ihost-automation/internal/config"
	"github.com/paulomcnally/p40la-ihost-automation/internal/db"
	"github.com/paulomcnally/p40la-ihost-automation/internal/plugins"
	"github.com/paulomcnally/p40la-ihost-automation/internal/services"
	"github.com/paulomcnally/p40la-ihost-automation/internal/storage"
	pluginall "github.com/paulomcnally/p40la-ihost-automation-plugins/all"
)

func main() {
	var (
		healthcheck = flag.Bool("healthcheck", false, "Verificar estado de salud del servidor")
		versionFlag = flag.Bool("version", false, "Mostrar versión")
	)
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	if *versionFlag {
		fmt.Println(cfg.Version)
		os.Exit(0)
	}

	initLogger(cfg.LogLevel)

	if *healthcheck {
		if err := runHealthcheck(cfg.Port); err != nil {
			slog.Error("healthcheck falló", "error", err)
			os.Exit(1)
		}
		slog.Info("healthcheck ok")
		os.Exit(0)
	}

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		slog.Error("crear directorio de datos", "error", err)
		os.Exit(1)
	}

	database, err := db.OpenDB(
		filepath.Join(cfg.DataDir, "app.db"),
		"./migrations",
	)
	if err != nil {
		slog.Error("abrir base de datos", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	userStorage := storage.NewUserStorage(database)
	settingsStorage := storage.NewSettingsStorage(database)
	appsStorage := storage.NewAppsStorage(database)
	pluginsStorage := storage.NewPluginsStorage(database)

	authService := services.NewAuthService(userStorage, settingsStorage, cfg)
	appSettingsService := services.NewAppSettingsService(settingsStorage)
	appsService := services.NewAppsService(appsStorage, pluginsStorage)

	registry := plugins.NewRegistry()
	pluginall.RegisterAll(registry)

	billsService := services.NewBillsService(appsService, registry, pluginsStorage)
	if err := billsService.SyncCatalog(context.Background()); err != nil {
		slog.Error("sincronizar catálogo de plugins", "error", err)
		os.Exit(1)
	}

	webhookService := services.NewWebhookService(appSettingsService, billsService, pluginsStorage)
	scheduler := services.NewScheduler(webhookService, pluginsStorage)
	scheduler.Start(context.Background())

	settingsHandlers := api.NewSettingsHandlers(appSettingsService)
	appsHandlers := api.NewAppsHandlers(appsService, authService)
	pluginsHandlers := api.NewPluginsHandlers(billsService)
	billsHandlers := api.NewBillsHandlers(billsService)
	webhookHandlers := api.NewWebhookHandlers(appSettingsService, webhookService)

	handler := api.NewHandler(authService, settingsHandlers)

	router := api.BuildRouter(handler, authService, appsHandlers, pluginsHandlers, billsHandlers, webhookHandlers, "./public")

	addr := ":" + cfg.Port
	slog.Info("iniciando servidor", "addr", addr, "version", cfg.Version)
	if err := http.ListenAndServe(addr, router); err != nil {
		slog.Error("servidor detenido", "error", err)
		os.Exit(1)
	}
}

func initLogger(level slog.Level) {
	opts := &slog.HandlerOptions{Level: level}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, opts)))
}

func runHealthcheck(port string) error {
	resp, err := http.Get("http://localhost:" + port + "/health")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
