package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/paulomcnally/p40la-ihost-automation/internal/models"
)

// PluginsStorage encapsula el acceso a las tablas plugins y bills.
type PluginsStorage struct {
	db *sql.DB
}

// NewPluginsStorage crea un nuevo PluginsStorage.
func NewPluginsStorage(db *sql.DB) *PluginsStorage {
	return &PluginsStorage{db: db}
}

// UpsertPlugin inserta o actualiza un plugin del catálogo.
func (s *PluginsStorage) UpsertPlugin(ctx context.Context, name, version, source string, description *string, status string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO plugins (name, version, source, description, status)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			version = excluded.version,
			source = excluded.source,
			description = excluded.description,
			status = excluded.status,
			updated_at = CURRENT_TIMESTAMP
	`, name, version, source, description, status)
	if err != nil {
		return fmt.Errorf("upsert plugin %s: %w", name, err)
	}
	return nil
}

// ListPlugins devuelve todos los plugins del catálogo.
func (s *PluginsStorage) ListPlugins(ctx context.Context) ([]models.Plugin, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, version, source, description, status, created_at, updated_at
		FROM plugins ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, fmt.Errorf("listar plugins: %w", err)
	}
	defer rows.Close()

	var plugins []models.Plugin
	for rows.Next() {
		var p models.Plugin
		var description sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &p.Version, &p.Source, &description, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("escanear plugin: %w", err)
		}
		if description.Valid {
			p.Description = &description.String
		}
		plugins = append(plugins, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return plugins, nil
}

// GetPlugin devuelve un plugin del catálogo por nombre.
func (s *PluginsStorage) GetPlugin(ctx context.Context, name string) (*models.Plugin, error) {
	var p models.Plugin
	var description sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, version, source, description, status, created_at, updated_at
		FROM plugins WHERE name = ?`, name).
		Scan(&p.ID, &p.Name, &p.Version, &p.Source, &description, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("obtener plugin %s: %w", name, err)
	}
	if description.Valid {
		p.Description = &description.String
	}
	return &p, nil
}

// SetAccountPlugin asocia (o desasocia con nil) un plugin a una cuenta.
func (s *PluginsStorage) SetAccountPlugin(ctx context.Context, accountID int64, pluginName *string) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE accounts SET plugin_name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		pluginName, accountID)
	if err != nil {
		return fmt.Errorf("asociar plugin a cuenta %d: %w", accountID, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CreateBill inserta una consulta de facturas.
func (s *PluginsStorage) CreateBill(ctx context.Context, rec models.BillRecord) (*models.BillRecord, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO bills (account_id, plugin_name, plugin_version, source, status, error, raw)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rec.AccountID, rec.PluginName, rec.PluginVersion, rec.Source, rec.Status, rec.Error, rec.Raw)
	if err != nil {
		return nil, fmt.Errorf("crear bill: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("obtener id de bill: %w", err)
	}
	return s.GetBill(ctx, id)
}

// GetBill devuelve un bill por id.
func (s *PluginsStorage) GetBill(ctx context.Context, id int64) (*models.BillRecord, error) {
	var rec models.BillRecord
	var errText sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id, account_id, plugin_name, plugin_version, source, status, error, raw, fetched_at, created_at
		FROM bills WHERE id = ?`, id).
		Scan(&rec.ID, &rec.AccountID, &rec.PluginName, &rec.PluginVersion, &rec.Source, &rec.Status,
			&errText, &rec.Raw, &rec.FetchedAt, &rec.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("obtener bill %d: %w", id, err)
	}
	if errText.Valid {
		rec.Error = &errText.String
	}
	return &rec, nil
}

// ListBills devuelve las consultas de una cuenta con paginación (más recientes primero).
func (s *PluginsStorage) ListBills(ctx context.Context, accountID int64, limit, offset int) ([]models.BillRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, plugin_name, plugin_version, source, status, error, raw, fetched_at, created_at
		FROM bills WHERE account_id = ? ORDER BY fetched_at DESC, id DESC LIMIT ? OFFSET ?`, accountID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listar bills: %w", err)
	}
	defer rows.Close()

	var bills []models.BillRecord
	for rows.Next() {
		var rec models.BillRecord
		var errText sql.NullString
		if err := rows.Scan(&rec.ID, &rec.AccountID, &rec.PluginName, &rec.PluginVersion, &rec.Source,
			&rec.Status, &errText, &rec.Raw, &rec.FetchedAt, &rec.CreatedAt); err != nil {
			return nil, fmt.Errorf("escanear bill: %w", err)
		}
		if errText.Valid {
			rec.Error = &errText.String
		}
		bills = append(bills, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return bills, nil
}

// CountBillsAfter devuelve cuántas consultas hay más recientes que un timestamp.
// Útil para resolver el offset de la página que contiene una ejecución dada.
func (s *PluginsStorage) CountBillsAfter(ctx context.Context, accountID int64, at time.Time) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM bills WHERE account_id = ? AND fetched_at > ?", accountID, at).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("contar bills recientes: %w", err)
	}
	return count, nil
}

// GetAccountWebhook devuelve la configuración webhook de una cuenta (nil si no existe).
func (s *PluginsStorage) GetAccountWebhook(ctx context.Context, accountID int64) (*models.AccountWebhook, error) {
	var url, schedule, frequency, days sql.NullString
	var enabled int
	var interval sql.NullInt64
	var lastRun sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT webhook_url, schedule_time, schedule_enabled, schedule_frequency, schedule_interval, schedule_days, last_run_at
		FROM accounts WHERE id = ?`, accountID).
		Scan(&url, &schedule, &enabled, &frequency, &interval, &days, &lastRun)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("obtener webhook de cuenta %d: %w", accountID, err)
	}
	cfg := &models.AccountWebhook{
		AccountID:         accountID,
		ScheduleEnabled:   enabled == 1,
		ScheduleFrequency: frequency.String,
	}
	if cfg.ScheduleFrequency == "" {
		cfg.ScheduleFrequency = "daily"
	}
	if url.Valid {
		cfg.WebhookURL = &url.String
	}
	if schedule.Valid {
		cfg.ScheduleTime = &schedule.String
	}
	if interval.Valid {
		v := int(interval.Int64)
		cfg.ScheduleInterval = &v
	}
	if days.Valid {
		cfg.ScheduleDays = &days.String
	}
	if lastRun.Valid {
		cfg.LastRunAt = &lastRun.Time
	}
	return cfg, nil
}

// SetAccountWebhook actualiza la configuración webhook de una cuenta.
func (s *PluginsStorage) SetAccountWebhook(ctx context.Context, cfg models.AccountWebhook) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE accounts SET
			webhook_url = ?, schedule_time = ?, schedule_enabled = ?,
			schedule_frequency = ?, schedule_interval = ?, schedule_days = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		cfg.WebhookURL, cfg.ScheduleTime, cfg.ScheduleEnabled,
		cfg.ScheduleFrequency, cfg.ScheduleInterval, cfg.ScheduleDays, cfg.AccountID)
	if err != nil {
		return fmt.Errorf("configurar webhook de cuenta %d: %w", cfg.AccountID, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ListEnabledWebhookAccounts devuelve las cuentas con job habilitado,
// plugin asociado y webhook_url configurada. La decisión de "debido"
// (hora + frecuencia) se hace en Go.
func (s *PluginsStorage) ListEnabledWebhookAccounts(ctx context.Context) ([]models.AccountWebhook, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, webhook_url, schedule_time, schedule_enabled, schedule_frequency, schedule_interval, schedule_days, last_run_at
		FROM accounts
		WHERE schedule_enabled = 1
		  AND plugin_name IS NOT NULL AND plugin_name != ''
		  AND webhook_url IS NOT NULL AND webhook_url != ''
		ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("listar cuentas con webhook habilitado: %w", err)
	}
	defer rows.Close()

	var cfgs []models.AccountWebhook
	for rows.Next() {
		var cfg models.AccountWebhook
		var url, schedule, frequency, days sql.NullString
		var enabled int
		var interval sql.NullInt64
		var lastRun sql.NullTime
		if err := rows.Scan(&cfg.AccountID, &url, &schedule, &enabled, &frequency, &interval, &days, &lastRun); err != nil {
			return nil, fmt.Errorf("escanear cuenta webhook: %w", err)
		}
		cfg.ScheduleEnabled = enabled == 1
		cfg.ScheduleFrequency = frequency.String
		if cfg.ScheduleFrequency == "" {
			cfg.ScheduleFrequency = "daily"
		}
		if url.Valid {
			cfg.WebhookURL = &url.String
		}
		if schedule.Valid {
			cfg.ScheduleTime = &schedule.String
		}
		if interval.Valid {
			v := int(interval.Int64)
			cfg.ScheduleInterval = &v
		}
		if days.Valid {
			cfg.ScheduleDays = &days.String
		}
		if lastRun.Valid {
			cfg.LastRunAt = &lastRun.Time
		}
		cfgs = append(cfgs, cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cfgs, nil
}

// SetLastRun marca la hora de última ejecución del job de una cuenta.
func (s *PluginsStorage) SetLastRun(ctx context.Context, accountID int64) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE accounts SET last_run_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?", accountID)
	if err != nil {
		return fmt.Errorf("marcar last_run de cuenta %d: %w", accountID, err)
	}
	return nil
}

// CreateWebhookLog inserta un evento de entrega de webhook.
func (s *PluginsStorage) CreateWebhookLog(ctx context.Context, log models.WebhookLog) (*models.WebhookLog, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO webhook_logs (account_id, webhook_url, status, http_status, payload, response)
		VALUES (?, ?, ?, ?, ?, ?)`,
		log.AccountID, log.WebhookURL, log.Status, log.HTTPStatus, log.Payload, log.Response)
	if err != nil {
		return nil, fmt.Errorf("crear webhook log: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("obtener id de webhook log: %w", err)
	}
	log.ID = id
	log.RanAt = time.Now().UTC()
	return &log, nil
}

// ListWebhookLogs devuelve los eventos de una cuenta con paginación.
func (s *PluginsStorage) ListWebhookLogs(ctx context.Context, accountID int64, limit, offset int) ([]models.WebhookLog, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, webhook_url, status, http_status, payload, response, ran_at
		FROM webhook_logs WHERE account_id = ? ORDER BY ran_at DESC, id DESC LIMIT ? OFFSET ?`, accountID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("listar webhook logs: %w", err)
	}
	defer rows.Close()

	var logs []models.WebhookLog
	for rows.Next() {
		var l models.WebhookLog
		var httpStatus sql.NullInt64
		if err := rows.Scan(&l.ID, &l.AccountID, &l.WebhookURL, &l.Status, &httpStatus, &l.Payload, &l.Response, &l.RanAt); err != nil {
			return nil, fmt.Errorf("escanear webhook log: %w", err)
		}
		if httpStatus.Valid {
			v := int(httpStatus.Int64)
			l.HTTPStatus = &v
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return logs, nil
}
