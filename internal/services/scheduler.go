package services

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/paulomcnally/p40la-ihost-automation/internal/models"
	"github.com/paulomcnally/p40la-ihost-automation/internal/storage"
)

// Frecuencias de ejecución soportadas por el job de webhook.
const (
	ScheduleDaily      = "daily"
	ScheduleEveryNDays = "every_n_days"
	ScheduleWeekly     = "weekly"
	ScheduleMonthly    = "monthly"
)

// Scheduler ejecuta los jobs de webhook de las cuentas configuradas.
type Scheduler struct {
	webhook *WebhookService
	storage *storage.PluginsStorage
	tick    time.Duration
}

// NewScheduler crea un nuevo Scheduler.
func NewScheduler(webhook *WebhookService, st *storage.PluginsStorage) *Scheduler {
	return &Scheduler{
		webhook: webhook,
		storage: st,
		tick:    30 * time.Second,
	}
}

// Start lanza el loop del scheduler en una goroutine hasta que ctx se cancele.
func (s *Scheduler) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(s.tick)
		defer ticker.Stop()
		s.runDue(ctx)
		for {
			select {
			case <-ctx.Done():
				slog.Info("scheduler detenido")
				return
			case <-ticker.C:
				s.runDue(ctx)
			}
		}
	}()
	slog.Info("scheduler de webhooks iniciado", "tick", s.tick.String())
}

// runDue busca cuentas cuyo job debe ejecutarse en este minuto según hora+frecuencia.
func (s *Scheduler) runDue(ctx context.Context) {
	now := time.Now()

	cfgs, err := s.storage.ListEnabledWebhookAccounts(ctx)
	if err != nil {
		slog.Error("scheduler: listar cuentas habilitadas", "error", err)
		return
	}
	for _, cfg := range cfgs {
		if !isDue(cfg, now) {
			continue
		}
		slog.Info("scheduler: ejecutando job de webhook", "account_id", cfg.AccountID, "frecuencia", cfg.ScheduleFrequency)
		delivered, failed, err := s.webhook.RunJob(ctx, cfg.AccountID)
		if err != nil {
			slog.Warn("scheduler: job fallido", "account_id", cfg.AccountID, "delivered", delivered, "failed", failed, "error", err)
			continue
		}
		slog.Info("scheduler: job completado", "account_id", cfg.AccountID, "delivered", delivered, "failed", failed)
	}
}

// isDue decide si una cuenta debe ejecutarse en el instante `now`.
func isDue(cfg models.AccountWebhook, now time.Time) bool {
	if !cfg.ScheduleEnabled || cfg.ScheduleTime == nil || *cfg.ScheduleTime == "" {
		return false
	}
	if now.Format("15:04") != *cfg.ScheduleTime {
		return false
	}

	switch cfg.ScheduleFrequency {
	case ScheduleEveryNDays:
		if cfg.ScheduleInterval == nil || *cfg.ScheduleInterval < 1 {
			return false
		}
		if cfg.LastRunAt == nil {
			return true
		}
		return now.Sub(*cfg.LastRunAt) >= time.Duration(*cfg.ScheduleInterval)*24*time.Hour
	case ScheduleWeekly:
		if cfg.ScheduleDays == nil || !containsWeekday(*cfg.ScheduleDays, int(now.Weekday())) {
			return false
		}
		return cfg.LastRunAt == nil || !sameDay(cfg.LastRunAt, now)
	case ScheduleMonthly:
		if cfg.ScheduleDays == nil {
			return false
		}
		day, err := strconv.Atoi(*cfg.ScheduleDays)
		if err != nil || day != now.Day() {
			return false
		}
		return cfg.LastRunAt == nil || !sameDay(cfg.LastRunAt, now)
	default: // daily
		return cfg.LastRunAt == nil || !sameDay(cfg.LastRunAt, now)
	}
}

// containsWeekday verifica si el día de la semana (0=Domingo) está en la lista CSV.
func containsWeekday(csv string, weekday int) bool {
	for _, part := range strings.Split(csv, ",") {
		d, err := strconv.Atoi(strings.TrimSpace(part))
		if err == nil && d == weekday {
			return true
		}
	}
	return false
}

func sameDay(a *time.Time, b time.Time) bool {
	if a == nil {
		return false
	}
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
