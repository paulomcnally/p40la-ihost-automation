package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/paulomcnally/p40la-ihost-automation/internal/models"
)

// AppsStorage encapsula el acceso a las tablas apps y accounts.
type AppsStorage struct {
	db *sql.DB
}

// NewAppsStorage crea un nuevo AppsStorage.
func NewAppsStorage(db *sql.DB) *AppsStorage {
	return &AppsStorage{db: db}
}

// CreateApp inserta una nueva app y devuelve la entidad creada.
func (s *AppsStorage) CreateApp(ctx context.Context, name string) (*models.App, error) {
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO apps (name) VALUES (?)", name,
	)
	if err != nil {
		return nil, fmt.Errorf("crear app: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("obtener id de app: %w", err)
	}
	app, err := s.GetApp(ctx, id)
	if err != nil {
		return nil, err
	}
	return app, nil
}

// GetApp devuelve una app por id.
func (s *AppsStorage) GetApp(ctx context.Context, id int64) (*models.App, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT a.id, a.name,
		        (SELECT COUNT(*) FROM accounts WHERE app_id = a.id) AS account_count,
		        a.created_at, a.updated_at
		 FROM apps a WHERE a.id = ?`, id,
	)
	var app models.App
	if err := row.Scan(&app.ID, &app.Name, &app.AccountCount, &app.CreatedAt, &app.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("obtener app %d: %w", id, err)
	}
	return &app, nil
}

// ListApps devuelve todas las apps con su cantidad de cuentas.
func (s *AppsStorage) ListApps(ctx context.Context) ([]models.App, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT a.id, a.name,
		        (SELECT COUNT(*) FROM accounts WHERE app_id = a.id) AS account_count,
		        a.created_at, a.updated_at
		 FROM apps a ORDER BY a.name COLLATE NOCASE`,
	)
	if err != nil {
		return nil, fmt.Errorf("listar apps: %w", err)
	}
	defer rows.Close()

	var apps []models.App
	for rows.Next() {
		var app models.App
		if err := rows.Scan(&app.ID, &app.Name, &app.AccountCount, &app.CreatedAt, &app.UpdatedAt); err != nil {
			return nil, fmt.Errorf("escanear app: %w", err)
		}
		apps = append(apps, app)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return apps, nil
}

// UpdateApp actualiza el nombre de una app.
func (s *AppsStorage) UpdateApp(ctx context.Context, id int64, name string) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE apps SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", name, id,
	)
	if err != nil {
		return fmt.Errorf("actualizar app %d: %w", id, err)
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

// DeleteApp borra una app y sus cuentas (ON DELETE CASCADE).
func (s *AppsStorage) DeleteApp(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM apps WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("borrar app %d: %w", id, err)
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

// CreateAccount inserta una cuenta dentro de una app.
func (s *AppsStorage) CreateAccount(ctx context.Context, appID int64, identifier string, label *string, credentials string, pluginName *string) (*models.Account, error) {
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO accounts (app_id, identifier, label, credentials, plugin_name) VALUES (?, ?, ?, ?, ?)",
		appID, identifier, label, credentials, pluginName,
	)
	if err != nil {
		return nil, fmt.Errorf("crear cuenta: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("obtener id de cuenta: %w", err)
	}
	account, err := s.GetAccount(ctx, id)
	if err != nil {
		return nil, err
	}
	return account, nil
}

// GetAccount devuelve una cuenta por id.
func (s *AppsStorage) GetAccount(ctx context.Context, id int64) (*models.Account, error) {
	var account models.Account
	var label sql.NullString
	var pluginName sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, app_id, identifier, label, plugin_name, credentials, created_at, updated_at
		 FROM accounts WHERE id = ?`, id,
	).Scan(&account.ID, &account.AppID, &account.Identifier, &label, &pluginName, &account.Credentials, &account.CreatedAt, &account.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("obtener cuenta %d: %w", id, err)
	}
	if label.Valid {
		account.Label = &label.String
	}
	if pluginName.Valid {
		account.PluginName = &pluginName.String
	}
	return &account, nil
}

// ListAccounts devuelve las cuentas de una app sin credenciales.
func (s *AppsStorage) ListAccounts(ctx context.Context, appID int64) ([]models.Account, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, app_id, identifier, label, plugin_name, created_at, updated_at
		 FROM accounts WHERE app_id = ? ORDER BY identifier COLLATE NOCASE`, appID,
	)
	if err != nil {
		return nil, fmt.Errorf("listar cuentas: %w", err)
	}
	defer rows.Close()

	var accounts []models.Account
	for rows.Next() {
		var account models.Account
		var label sql.NullString
		var pluginName sql.NullString
		if err := rows.Scan(&account.ID, &account.AppID, &account.Identifier, &label, &pluginName, &account.CreatedAt, &account.UpdatedAt); err != nil {
			return nil, fmt.Errorf("escanear cuenta: %w", err)
		}
		if label.Valid {
			account.Label = &label.String
		}
		if pluginName.Valid {
			account.PluginName = &pluginName.String
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accounts, nil
}

// UpdateAccount actualiza identifier, label y credentials de una cuenta.
func (s *AppsStorage) UpdateAccount(ctx context.Context, id int64, identifier string, label *string, credentials string) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE accounts SET identifier = ?, label = ?, credentials = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		identifier, label, credentials, id,
	)
	if err != nil {
		return fmt.Errorf("actualizar cuenta %d: %w", id, err)
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

// DeleteAccount borra una cuenta.
func (s *AppsStorage) DeleteAccount(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM accounts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("borrar cuenta %d: %w", id, err)
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

// IsDuplicate detecta errores de violación de unicidad de SQLite.
func IsDuplicate(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
