package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/paulomcnally/p40la-ihost-automation/internal/models"
	"github.com/paulomcnally/p40la-ihost-automation/internal/storage"
)

// Errores de dominio del módulo de apps.
var (
	ErrNotFound     = errors.New("registro no encontrado")
	ErrDuplicate    = errors.New("registro duplicado")
	ErrInvalidJSON  = errors.New("credenciales no son JSON válido")
	ErrInvalidName  = errors.New("el nombre no puede estar vacío")
	ErrInvalidID    = errors.New("identificador no válido")
	ErrInvalidCreds = errors.New("credenciales no válidas")
)

// AppsService contiene la lógica de negocio para apps y cuentas.
type AppsService struct {
	storage *storage.AppsStorage
	plugins *storage.PluginsStorage
}

// NewAppsService crea un nuevo AppsService.
func NewAppsService(st *storage.AppsStorage, plugins *storage.PluginsStorage) *AppsService {
	return &AppsService{storage: st, plugins: plugins}
}

func validateCredentials(raw string) error {
	var obj any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return ErrInvalidJSON
	}
	if _, ok := obj.(map[string]any); !ok {
		return ErrInvalidJSON
	}
	return nil
}

// ListApps devuelve todas las apps con su cantidad de cuentas.
func (s *AppsService) ListApps(ctx context.Context) ([]models.App, error) {
	return s.storage.ListApps(ctx)
}

// CreateApp crea una nueva app validando el nombre.
func (s *AppsService) CreateApp(ctx context.Context, name string) (*models.App, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalidName
	}
	app, err := s.storage.CreateApp(ctx, name)
	if err != nil {
		if storage.IsDuplicate(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return app, nil
}

// UpdateApp actualiza el nombre de una app.
func (s *AppsService) UpdateApp(ctx context.Context, id int64, name string) (*models.App, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalidName
	}
	if err := s.storage.UpdateApp(ctx, id, name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		if storage.IsDuplicate(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return s.storage.GetApp(ctx, id)
}

// DeleteApp borra una app y sus cuentas.
func (s *AppsService) DeleteApp(ctx context.Context, id int64) error {
	if err := s.storage.DeleteApp(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// ListAccounts devuelve las cuentas de una app (sin credenciales).
func (s *AppsService) ListAccounts(ctx context.Context, appID int64) ([]models.Account, error) {
	return s.storage.ListAccounts(ctx, appID)
}

// CreateAccount crea una cuenta validando identificador, credenciales JSON y
// (opcionalmente) el plugin del catálogo que queda asociado a la cuenta.
func (s *AppsService) CreateAccount(ctx context.Context, appID int64, identifier string, label *string, credentials string, pluginName *string) (*models.Account, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, ErrInvalidID
	}
	if err := validateCredentials(credentials); err != nil {
		return nil, err
	}
	if pluginName != nil {
		name := strings.TrimSpace(*pluginName)
		if name == "" {
			pluginName = nil
		} else {
			plugin, err := s.plugins.GetPlugin(ctx, name)
			if err != nil {
				return nil, err
			}
			if plugin == nil {
				return nil, ErrPluginNotFound
			}
			pluginName = &name
		}
	}
	account, err := s.storage.CreateAccount(ctx, appID, identifier, label, credentials, pluginName)
	if err != nil {
		if storage.IsDuplicate(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return account, nil
}

// UpdateAccount actualiza una cuenta.
func (s *AppsService) UpdateAccount(ctx context.Context, accountID int64, identifier string, label *string, credentials string) (*models.Account, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, ErrInvalidID
	}
	if err := validateCredentials(credentials); err != nil {
		return nil, err
	}
	if err := s.storage.UpdateAccount(ctx, accountID, identifier, label, credentials); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		if storage.IsDuplicate(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return s.storage.GetAccount(ctx, accountID)
}

// DeleteAccount borra una cuenta.
func (s *AppsService) DeleteAccount(ctx context.Context, id int64) error {
	if err := s.storage.DeleteAccount(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// AccountWithCredentials devuelve una cuenta incluyendo sus credenciales.
func (s *AppsService) AccountWithCredentials(ctx context.Context, id int64) (*models.Account, error) {
	account, err := s.storage.GetAccount(ctx, id)
	if err != nil {
		return nil, err
	}
	if account == nil {
		return nil, ErrNotFound
	}
	return account, nil
}

// CredentialsForAccount devuelve las credenciales (JSON) de una cuenta.
func (s *AppsService) CredentialsForAccount(ctx context.Context, id int64) (string, error) {
	account, err := s.storage.GetAccount(ctx, id)
	if err != nil {
		return "", err
	}
	if account == nil {
		return "", ErrNotFound
	}
	return account.Credentials, nil
}
