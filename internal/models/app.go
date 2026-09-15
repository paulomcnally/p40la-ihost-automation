package models

import "time"

// App representa una aplicación/proveedor (ej. Claro Nicaragua) con sus cuentas.
type App struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	AccountCount int64     `json:"account_count,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Account representa una cuenta/servicio dentro de una app.
type Account struct {
	ID          int64     `json:"id"`
	AppID       int64     `json:"app_id"`
	Identifier  string    `json:"identifier"`
	Label       *string   `json:"label,omitempty"`
	PluginName  *string   `json:"plugin_name,omitempty"`
	Credentials string    `json:"-"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
