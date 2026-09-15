package services

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/paulomcnally/p40la-ihost-automation/internal/models"
	"github.com/paulomcnally/p40la-ihost-automation/internal/storage"
)

// Errores de dominio del módulo de webhooks.
var (
	ErrWebhookDisabled = errors.New("los webhooks están deshabilitados")
	ErrWebhookNoKey    = errors.New("no hay api_key de webhook configurada")
	ErrWebhookNoURL    = errors.New("la cuenta no tiene webhook_url configurada")
)

// WebhookService gestiona el envío de facturas al webhook y su configuración.
type WebhookService struct {
	settings *AppSettingsService
	bills    *BillsService
	storage  *storage.PluginsStorage
	client   *http.Client
}

// NewWebhookService crea un nuevo WebhookService.
func NewWebhookService(settings *AppSettingsService, bills *BillsService, st *storage.PluginsStorage) *WebhookService {
	return &WebhookService{
		settings: settings,
		bills:    bills,
		storage:  st,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

// GetAccountWebhook devuelve la configuración webhook de una cuenta.
func (s *WebhookService) GetAccountWebhook(ctx context.Context, accountID int64) (*models.AccountWebhook, error) {
	cfg, err := s.storage.GetAccountWebhook(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, ErrNotFound
	}
	return cfg, nil
}

// SetAccountWebhook valida y persiste la configuración webhook de una cuenta.
func (s *WebhookService) SetAccountWebhook(ctx context.Context, cfg models.AccountWebhook) (*models.AccountWebhook, error) {
	if _, err := s.bills.apps.AccountWithCredentials(ctx, cfg.AccountID); err != nil {
		return nil, err
	}
	if cfg.WebhookURL != nil && *cfg.WebhookURL != "" {
		if err := validateWebhookURL(*cfg.WebhookURL); err != nil {
			return nil, err
		}
	}
	if cfg.ScheduleTime != nil && *cfg.ScheduleTime != "" {
		if !validScheduleTime(*cfg.ScheduleTime) {
			return nil, fmt.Errorf("schedule_time debe tener formato HH:MM")
		}
	}
	if cfg.ScheduleFrequency == "" {
		cfg.ScheduleFrequency = "daily"
	}
	if err := validateScheduleFrequency(cfg); err != nil {
		return nil, err
	}
	if err := s.storage.SetAccountWebhook(ctx, cfg); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s.storage.GetAccountWebhook(ctx, cfg.AccountID)
}

func validateScheduleFrequency(cfg models.AccountWebhook) error {
	switch cfg.ScheduleFrequency {
	case "daily":
		return nil
	case "every_n_days":
		if cfg.ScheduleInterval == nil || *cfg.ScheduleInterval < 1 || *cfg.ScheduleInterval > 365 {
			return fmt.Errorf("schedule_interval debe estar entre 1 y 365 días")
		}
		return nil
	case "weekly":
		if cfg.ScheduleDays == nil || *cfg.ScheduleDays == "" {
			return fmt.Errorf("schedule_days es obligatorio para frecuencia semanal (0-6)")
		}
		for _, part := range strings.Split(*cfg.ScheduleDays, ",") {
			d, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil || d < 0 || d > 6 {
				return fmt.Errorf("schedule_days inválido para semanal: %q", *cfg.ScheduleDays)
			}
		}
		return nil
	case "monthly":
		if cfg.ScheduleDays == nil || *cfg.ScheduleDays == "" {
			return fmt.Errorf("schedule_days es obligatorio para frecuencia mensual (1-31)")
		}
		d, err := strconv.Atoi(*cfg.ScheduleDays)
		if err != nil || d < 1 || d > 31 {
			return fmt.Errorf("schedule_days inválido para mensual: %q", *cfg.ScheduleDays)
		}
		return nil
	default:
		return fmt.Errorf("frecuencia no soportada: %s", cfg.ScheduleFrequency)
	}
}

// ListWebhookLogs devuelve los eventos de entrega de una cuenta con paginación.
func (s *WebhookService) ListWebhookLogs(ctx context.Context, accountID int64, limit, offset int) ([]models.WebhookLog, error) {
	return s.storage.ListWebhookLogs(ctx, accountID, limit, offset)
}

// RunJob ejecuta la consulta de facturas de una cuenta y envía cada factura al webhook.
func (s *WebhookService) RunJob(ctx context.Context, accountID int64) (delivered, failed int, err error) {
	cfg, err := s.storage.GetAccountWebhook(ctx, accountID)
	if err != nil {
		return 0, 0, err
	}
	if cfg == nil || cfg.WebhookURL == nil || *cfg.WebhookURL == "" {
		return 0, 0, ErrWebhookNoURL
	}

	enabled := s.settings.IsWebhookEnabled(ctx)
	apiKey, err := s.settings.GetWebhookAPIKey(ctx)
	if err != nil {
		return 0, 0, err
	}
	if !enabled {
		s.logEvent(ctx, accountID, *cfg.WebhookURL, "error", 0, "{}", ErrWebhookDisabled.Error())
		return 0, 1, ErrWebhookDisabled
	}
	if apiKey == "" {
		s.logEvent(ctx, accountID, *cfg.WebhookURL, "error", 0, "{}", ErrWebhookNoKey.Error())
		return 0, 1, ErrWebhookNoKey
	}

	_, bills, err := s.bills.FetchBills(ctx, accountID)
	if err != nil {
		s.logEvent(ctx, accountID, *cfg.WebhookURL, "error", 0, "{}", "error consultando facturas: "+err.Error())
		return 0, 1, err
	}

	for _, b := range bills {
		payload, perr := buildWebhookPayload(b)
		if perr != nil {
			failed++
			s.logEvent(ctx, accountID, *cfg.WebhookURL, "error", 0, "{}", perr.Error())
			continue
		}
		payloadJSON, _ := json.Marshal(payload)
		status, httpStatus, response := s.send(ctx, *cfg.WebhookURL, apiKey, payload)
		s.logEvent(ctx, accountID, *cfg.WebhookURL, status, httpStatus, string(payloadJSON), response)
		if status == "ok" {
			delivered++
		} else {
			failed++
		}
	}

	if err := s.storage.SetLastRun(ctx, accountID); err != nil {
		slog.Error("marcar last_run", "account_id", accountID, "error", err)
	}
	return delivered, failed, nil
}

// logEvent registra una entrega en webhook_logs.
func (s *WebhookService) logEvent(ctx context.Context, accountID int64, webhookURL, status string, httpStatus int, payload, response string) {
	var statusPtr *int
	if httpStatus > 0 {
		statusPtr = &httpStatus
	}
	if _, err := s.storage.CreateWebhookLog(ctx, models.WebhookLog{
		AccountID:  accountID,
		WebhookURL: webhookURL,
		Status:     status,
		HTTPStatus: statusPtr,
		Payload:    payload,
		Response:   response,
	}); err != nil {
		slog.Error("registrar webhook log", "account_id", accountID, "error", err)
	}
}

// send hace el POST al webhook con el header X-Webhook-Key.
func (s *WebhookService) send(ctx context.Context, webhookURL, apiKey string, payload map[string]any) (string, int, string) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "error", 0, err.Error()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return "error", 0, err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Key", apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return "error", 0, "request: " + err.Error()
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "ok", resp.StatusCode, strings.TrimSpace(string(responseBody))
	}
	return "error", resp.StatusCode, fmt.Sprintf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
}

// buildWebhookPayload adapta una factura al contrato de docs/webhooks-api.md.
func buildWebhookPayload(b BillPayload) (map[string]any, error) {
	year, month, err := parseBillPeriod(b.Period)
	if err != nil {
		return nil, fmt.Errorf("periodo inválido %q: %w", b.Period, err)
	}
	amount, err := strconv.ParseFloat(strings.TrimSpace(b.Amount), 64)
	if err != nil {
		amount = 0
	}

	payload := map[string]any{
		"year":   year,
		"month":  month,
		"amount": amount,
		"status": b.Status,
	}
	if invoice := extractInvoiceNumber(b.Raw); invoice != "" {
		payload["invoice_number"] = invoice
	}
	return payload, nil
}

// extractInvoiceNumber obtiene el número de factura limpio del raw de un bill.
//
// Los plugins guardan el número en claves distintas: Claro/DISNORTE usan
// "numFactura" (string); Tigo usa "invoiceId", que la API devuelve envuelto en
// {label, show, value, formattedValue} (por eso el fmt.Sprint directo producía
// "map[...]" en el webhook). La función tolera ambos formatos y la clave directa
// "invoice_number". Prefiere value cuando es escalar; si value es compuesto,
// cae a formattedValue. Devuelve "" si no hay número extraíble.
func extractInvoiceNumber(raw map[string]any) string {
	for _, key := range []string{"invoice_number", "numFactura", "invoiceId"} {
		if v, ok := raw[key]; ok && v != nil {
			if s := scalarValue(v); s != "" {
				return s
			}
		}
	}
	return ""
}

// scalarValue extrae un valor escalar de un campo de la API, que puede venir
// como string/número directo o como objeto {label, show, value, formattedValue}.
func scalarValue(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case map[string]any:
		if val, ok := t["value"]; ok && val != nil {
			if s := scalarValue(val); s != "" {
				return s
			}
		}
		if fv, ok := t["formattedValue"]; ok && fv != nil {
			if s := scalarValue(fv); s != "" {
				return s
			}
		}
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

// parseBillPeriod interpreta el periodo de una factura y devuelve año y mes.
// Acepta "DD-MM-YYYY" (formato usado por Claro) y "YYYY-MM" (formato del
// plugin Tigo); en el segundo caso se usa el día 1 del mes.
func parseBillPeriod(period string) (int, int, error) {
	period = strings.TrimSpace(period)
	if t, err := time.Parse("02-01-2006", period); err == nil {
		return t.Year(), int(t.Month()), nil
	}
	t, err := time.Parse("2006-01", period)
	if err != nil {
		return 0, 0, err
	}
	return t.Year(), int(t.Month()), nil
}

func validateWebhookURL(url string) error {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("webhook_url debe ser una URL http(s) válida")
	}
	return nil
}

func validScheduleTime(value string) bool {
	t, err := time.Parse("15:04", strings.TrimSpace(value))
	return err == nil && t.Hour() >= 0 && t.Hour() < 24
}
