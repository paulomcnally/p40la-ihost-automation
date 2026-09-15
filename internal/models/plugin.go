package models

import "time"

// Plugin representa una entrada del catálogo de plugins en SQLite.
type Plugin struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Source      string    `json:"source"`
	Description *string   `json:"description,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// BillRecord representa una consulta de facturas persistida.
type BillRecord struct {
	ID            int64     `json:"id"`
	AccountID     int64     `json:"account_id"`
	PluginName    string    `json:"plugin_name"`
	PluginVersion string    `json:"plugin_version"`
	Source        string    `json:"source"`
	Status        string    `json:"status"`
	Error         *string   `json:"error,omitempty"`
	Raw           string    `json:"raw"`
	FetchedAt     time.Time `json:"fetched_at"`
	CreatedAt     time.Time `json:"created_at"`
}
