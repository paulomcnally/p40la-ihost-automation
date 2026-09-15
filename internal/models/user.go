package models

import "time"

// User representa la cuenta de administrador única del sistema.
type User struct {
	ID           int64     `json:"user_id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
