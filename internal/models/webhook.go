package models

import "time"

// AccountWebhook representa la configuración de webhook de una cuenta.
type AccountWebhook struct {
	AccountID         int64      `json:"account_id"`
	WebhookURL        *string    `json:"webhook_url"`
	ScheduleTime      *string    `json:"schedule_time"`
	ScheduleEnabled   bool       `json:"schedule_enabled"`
	ScheduleFrequency string     `json:"schedule_frequency"`
	ScheduleInterval  *int       `json:"schedule_interval,omitempty"`
	ScheduleDays      *string    `json:"schedule_days,omitempty"`
	LastRunAt         *time.Time `json:"last_run_at,omitempty"`
}

// WebhookLog representa un evento de entrega de facturas a un webhook.
type WebhookLog struct {
	ID         int64     `json:"id"`
	AccountID  int64     `json:"account_id"`
	WebhookURL string    `json:"webhook_url"`
	Status     string    `json:"status"`
	HTTPStatus *int      `json:"http_status,omitempty"`
	Payload    string    `json:"payload"`
	Response   string    `json:"response"`
	RanAt      time.Time `json:"ran_at"`
}
