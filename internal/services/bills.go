package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/paulomcnally/p40la-ihost-automation/internal/models"
	"github.com/paulomcnally/p40la-ihost-automation/internal/plugins"
	"github.com/paulomcnally/p40la-ihost-automation/internal/storage"
)

// Errores de dominio del módulo de facturas.
var (
	ErrPluginNotFound   = errors.New("plugin no encontrado")
	ErrNoPluginAssigned = errors.New("la cuenta no tiene plugin asociado")
)

// BillPayload es la representación persistible/visible de una factura.
type BillPayload struct {
	Period        string         `json:"period"`
	Amount        string         `json:"amount"`
	DueDate       string         `json:"due_date"`
	Status        string         `json:"status"`
	InvoiceNumber string         `json:"invoice_number,omitempty"`
	Raw           map[string]any `json:"raw,omitempty"`
}

// BillsService orquesta las consultas de facturas vía plugins.
type BillsService struct {
	apps    *AppsService
	plugins *plugins.Registry
	storage *storage.PluginsStorage
}

// NewBillsService crea un nuevo BillsService.
func NewBillsService(apps *AppsService, registry *plugins.Registry, st *storage.PluginsStorage) *BillsService {
	return &BillsService{apps: apps, plugins: registry, storage: st}
}

// SyncCatalog sincroniza el registro de plugins compilados con el catálogo SQLite.
func (s *BillsService) SyncCatalog(ctx context.Context) error {
	for _, p := range s.plugins.List() {
		desc := p.Description()
		if err := s.storage.UpsertPlugin(ctx, p.Name(), p.Version(), p.Source(), &desc, "active"); err != nil {
			return err
		}
	}
	return nil
}

// ListPlugins devuelve el catálogo de plugins desde SQLite.
func (s *BillsService) ListPlugins(ctx context.Context) ([]models.Plugin, error) {
	return s.storage.ListPlugins(ctx)
}

// PluginSchema devuelve el schema de credenciales de un plugin registrado.
// Si el plugin no existe en el registry, devuelve ErrPluginNotFound.
func (s *BillsService) PluginSchema(ctx context.Context, name string) ([]plugins.CredentialField, error) {
	p, ok := s.plugins.Get(name)
	if !ok {
		return nil, ErrPluginNotFound
	}
	return p.CredentialSchema(), nil
}

// AccountPlugin devuelve el plugin asociado a una cuenta (nil si no tiene).
func (s *BillsService) AccountPlugin(ctx context.Context, accountID int64) (*models.Plugin, error) {
	account, err := s.apps.AccountWithCredentials(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account.PluginName == nil {
		return nil, nil
	}
	return s.storage.GetPlugin(ctx, *account.PluginName)
}

// SetAccountPlugin asocia o desasocia (nil) un plugin a una cuenta.
func (s *BillsService) SetAccountPlugin(ctx context.Context, accountID int64, pluginName *string) (*models.Plugin, error) {
	if _, err := s.apps.AccountWithCredentials(ctx, accountID); err != nil {
		return nil, err
	}
	if pluginName != nil {
		if _, ok := s.plugins.Get(*pluginName); !ok {
			return nil, ErrPluginNotFound
		}
	}
	if err := s.storage.SetAccountPlugin(ctx, accountID, pluginName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if pluginName == nil {
		return nil, nil
	}
	return s.storage.GetPlugin(ctx, *pluginName)
}

// FetchBills consulta las facturas de una cuenta, persiste el resultado y lo devuelve.
func (s *BillsService) FetchBills(ctx context.Context, accountID int64) (*models.BillRecord, []BillPayload, error) {
	account, err := s.apps.AccountWithCredentials(ctx, accountID)
	if err != nil {
		return nil, nil, err
	}
	if account.PluginName == nil {
		return nil, nil, ErrNoPluginAssigned
	}

	p, ok := s.plugins.Get(*account.PluginName)
	if !ok {
		return nil, nil, ErrPluginNotFound
	}

	var creds map[string]string
	if err := json.Unmarshal([]byte(account.Credentials), &creds); err != nil {
		return nil, nil, ErrInvalidCreds
	}

	record := models.BillRecord{
		AccountID:     accountID,
		PluginName:    p.Name(),
		PluginVersion: p.Version(),
		Source:        p.Source(),
	}

	bills, err := p.FetchBills(ctx, creds, account.Identifier)
	if err != nil {
		record.Status = "error"
		msg := err.Error()
		record.Error = &msg
		record.Raw = "{}"
		rec, perr := s.storage.CreateBill(ctx, record)
		if perr != nil {
			return nil, nil, perr
		}
		return rec, nil, err
	}

	payloads := make([]BillPayload, 0, len(bills))
	for _, b := range bills {
		payloads = append(payloads, BillPayload{
			Period:        b.Period,
			Amount:        b.Amount,
			DueDate:       b.DueDate,
			Status:        b.Status,
			InvoiceNumber: extractInvoiceNumber(b.Raw),
			Raw:           b.Raw,
		})
	}
	raw, err := json.Marshal(payloads)
	if err != nil {
		return nil, nil, err
	}
	record.Status = "ok"
	record.Raw = string(raw)

	rec, err := s.storage.CreateBill(ctx, record)
	if err != nil {
		return nil, nil, err
	}
	return rec, payloads, nil
}

// ListBills devuelve las consultas de facturas de una cuenta con paginación.
func (s *BillsService) ListBills(ctx context.Context, accountID int64, limit, offset int) ([]models.BillRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.storage.ListBills(ctx, accountID, limit, offset)
}

// ResolveOffsetFor devuelve el offset de la página que contiene la primera consulta
// igual o anterior a `at` (para saltar a la página de una ejecución concreta).
func (s *BillsService) ResolveOffsetFor(ctx context.Context, accountID int64, at time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 20
	}
	count, err := s.storage.CountBillsAfter(ctx, accountID, at)
	if err != nil {
		return 0, err
	}
	return (count / limit) * limit, nil
}
