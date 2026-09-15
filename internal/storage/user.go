package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/paulomcnally/p40la-ihost-automation/internal/models"
)

// UserStorage encapsula el acceso a la tabla users.
type UserStorage struct {
	db *sql.DB
}

// NewUserStorage crea un nuevo UserStorage.
func NewUserStorage(db *sql.DB) *UserStorage {
	return &UserStorage{db: db}
}

// Create inserta un nuevo usuario y devuelve el registro creado.
func (s *UserStorage) Create(ctx context.Context, email, passwordHash string) (*models.User, error) {
	result, err := s.db.ExecContext(ctx,
		"INSERT INTO users (email, password_hash) VALUES (?, ?)",
		email, passwordHash,
	)
	if err != nil {
		return nil, fmt.Errorf("insertar usuario: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("obtener id de usuario: %w", err)
	}
	return s.GetByID(ctx, id)
}

// GetByID busca un usuario por su ID.
func (s *UserStorage) GetByID(ctx context.Context, id int64) (*models.User, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT id, email, password_hash, created_at, updated_at FROM users WHERE id = ?", id,
	)
	return scanUser(row)
}

// GetByEmail busca un usuario por su email.
func (s *UserStorage) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email = ?", email,
	)
	return scanUser(row)
}

// Count devuelve la cantidad de usuarios registrados.
func (s *UserStorage) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return 0, fmt.Errorf("contar usuarios: %w", err)
	}
	return count, nil
}

func scanUser(row *sql.Row) (*models.User, error) {
	var u models.User
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("escanear usuario: %w", err)
	}
	return &u, nil
}
