package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/paulomcnally/p40la-ihost-automation/internal/storage"
)

const (
	SettingLanguage = "language"

	SettingWebhookAPIKey   = "webhook_api_key"
	SettingWebhookEnabled  = "webhook_enabled"
	WebhookAPIKeyMinLength = 32
)

var allowedLanguages = map[string]bool{
	"es": true,
	"en": true,
}

// AppSettingsService contiene la lógica de negocio para configuraciones de la aplicación.
type AppSettingsService struct {
	storage *storage.SettingsStorage
}

// NewAppSettingsService crea un nuevo AppSettingsService.
func NewAppSettingsService(st *storage.SettingsStorage) *AppSettingsService {
	return &AppSettingsService{storage: st}
}

// GetLanguage devuelve el idioma configurado, por defecto 'es'.
func (s *AppSettingsService) GetLanguage(ctx context.Context) (string, error) {
	value, err := s.storage.Get(ctx, SettingLanguage)
	if err != nil {
		return "", fmt.Errorf("obtener idioma: %w", err)
	}
	if value == "" || !allowedLanguages[value] {
		return "es", nil
	}
	return value, nil
}

// SetLanguage cambia el idioma de la aplicación.
func (s *AppSettingsService) SetLanguage(ctx context.Context, lang string) error {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if !allowedLanguages[lang] {
		return fmt.Errorf("idioma no soportado")
	}
	return s.storage.Set(ctx, SettingLanguage, lang)
}

// GetAll devuelve todas las configuraciones.
func (s *AppSettingsService) GetAll(ctx context.Context) (map[string]string, error) {
	settings, err := s.storage.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(settings))
	for _, st := range settings {
		result[st.Key] = st.Value
	}
	lang, err := s.GetLanguage(ctx)
	if err != nil {
		return nil, err
	}
	result[SettingLanguage] = lang
	result[SettingWebhookEnabled] = fmt.Sprintf("%t", s.IsWebhookEnabled(ctx))
	return result, nil
}

// GetWebhookAPIKey devuelve la api_key global de webhooks (vacía si no está configurada).
func (s *AppSettingsService) GetWebhookAPIKey(ctx context.Context) (string, error) {
	value, err := s.storage.Get(ctx, SettingWebhookAPIKey)
	if err != nil {
		return "", fmt.Errorf("obtener api_key de webhook: %w", err)
	}
	return value, nil
}

// IsWebhookEnabled indica si la feature de webhooks está activa.
func (s *AppSettingsService) IsWebhookEnabled(ctx context.Context) bool {
	value, err := s.storage.Get(ctx, SettingWebhookEnabled)
	if err != nil {
		return false
	}
	return value == "true" || value == "1"
}

// SetWebhookConfig actualiza la api_key y el toggle maestro de webhooks.
func (s *AppSettingsService) SetWebhookConfig(ctx context.Context, apiKey string, enabled bool) error {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey != "" && len(apiKey) < WebhookAPIKeyMinLength {
		return fmt.Errorf("la api_key debe tener al menos %d caracteres", WebhookAPIKeyMinLength)
	}
	if err := s.storage.Set(ctx, SettingWebhookAPIKey, apiKey); err != nil {
		return err
	}
	if err := s.storage.Set(ctx, SettingWebhookEnabled, fmt.Sprintf("%t", enabled)); err != nil {
		return err
	}
	return nil
}
