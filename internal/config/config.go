package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"log/slog"
)

// Config agrupa toda la configuración de la aplicación.
type Config struct {
	Port            string
	DataDir         string
	LogLevel        slog.Level
	BcryptCost      int
	SessionSecret   string
	SessionDuration time.Duration
	CookieName      string
	SecureCookie    bool
	Version         string
}

// Load lee la configuración desde variables de entorno con valores por defecto.
func Load() (*Config, error) {
	port := getEnv("PORT", "8089")
	dataDir := getEnv("DATA_DIR", "./data")
	logLevel, err := parseLogLevel(getEnv("LOG_LEVEL", "info"))
	if err != nil {
		return nil, fmt.Errorf("LOG_LEVEL inválido: %w", err)
	}

	bcryptCost, err := strconv.Atoi(getEnv("BCRYPT_COST", "10"))
	if err != nil || bcryptCost < 4 || bcryptCost > 31 {
		return nil, fmt.Errorf("BCRYPT_COST debe ser un entero entre 4 y 31")
	}

	sessionDuration, err := time.ParseDuration(getEnv("SESSION_DURATION", "24h"))
	if err != nil {
		return nil, fmt.Errorf("SESSION_DURATION inválida: %w", err)
	}

	secureCookie, err := strconv.ParseBool(getEnv("SECURE_COOKIE", "false"))
	if err != nil {
		return nil, fmt.Errorf("SECURE_COOKIE debe ser true o false")
	}

	return &Config{
		Port:            port,
		DataDir:         dataDir,
		LogLevel:        logLevel,
		BcryptCost:      bcryptCost,
		SessionSecret:   getEnv("SESSION_SECRET", ""),
		SessionDuration: sessionDuration,
		CookieName:      getEnv("COOKIE_NAME", "p40la_ihost_automation_session"),
		SecureCookie:    secureCookie,
		Version:         getEnv("VERSION", "dev"),
	}, nil
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func parseLogLevel(level string) (slog.Level, error) {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("nivel desconocido: %s", level)
	}
}
