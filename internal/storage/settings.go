package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/paulomcnally/p40la-ihost-automation/internal/models"
)

// SettingsStorage encapsula el acceso a la tabla settings.
type SettingsStorage struct {
	db *sql.DB
}

// NewSettingsStorage crea un nuevo SettingsStorage.
func NewSettingsStorage(db *sql.DB) *SettingsStorage {
	return &SettingsStorage{db: db}
}

// Get devuelve el valor de una clave o una cadena vacía si no existe.
func (s *SettingsStorage) Get(ctx context.Context, key string) (string, error) {
	var value string
	if err := s.db.QueryRowContext(ctx,
		"SELECT value FROM settings WHERE key = ?", key,
	).Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", fmt.Errorf("obtener setting %s: %w", key, err)
	}
	return value, nil
}

// Set inserta o actualiza el valor de una clave.
func (s *SettingsStorage) Set(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	if err != nil {
		return fmt.Errorf("guardar setting %s: %w", key, err)
	}
	return nil
}

// GetAll devuelve todos los settings (útil para debug/admin).
func (s *SettingsStorage) GetAll(ctx context.Context) ([]models.Setting, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT key, value FROM settings")
	if err != nil {
		return nil, fmt.Errorf("listar settings: %w", err)
	}
	defer rows.Close()

	var settings []models.Setting
	for rows.Next() {
		var st models.Setting
		if err := rows.Scan(&st.Key, &st.Value); err != nil {
			return nil, fmt.Errorf("escanear setting: %w", err)
		}
		settings = append(settings, st)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return settings, nil
}
