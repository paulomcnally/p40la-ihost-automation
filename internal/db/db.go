package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

// OpenDB abre la base de datos SQLite y aplica las migraciones.
func OpenDB(dbPath, migrationsDir string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("abrir base de datos: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping base de datos: %w", err)
	}
	if err := Migrate(db, migrationsDir); err != nil {
		return nil, fmt.Errorf("migrar base de datos: %w", err)
	}
	return db, nil
}

// Migrate aplica todos los archivos .up.sql del directorio de migraciones en orden,
// registrando cada una en la tabla schema_migrations para evitar re-ejecuciones.
func Migrate(db *sql.DB, migrationsDir string) error {
	if err := ensureSchemaMigrationsTable(db); err != nil {
		return fmt.Errorf("crear tabla de migraciones: %w", err)
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("leer migraciones: %w", err)
	}

	var ups []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			ups = append(ups, e.Name())
		}
	}
	sort.Strings(ups)

	for _, name := range ups {
		applied, err := isMigrationApplied(db, name)
		if err != nil {
			return fmt.Errorf("verificar migración %s: %w", name, err)
		}
		if applied {
			continue
		}

		data, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			return fmt.Errorf("leer migración %s: %w", name, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("iniciar transacción para %s: %w", name, err)
		}

		if _, err := tx.Exec(string(data)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("ejecutar migración %s: %w", name, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("registrar migración %s: %w", name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("confirmar migración %s: %w", name, err)
		}
	}
	return nil
}

func ensureSchemaMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func isMigrationApplied(db *sql.DB, version string) (bool, error) {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}
