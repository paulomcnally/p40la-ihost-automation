ALTER TABLE accounts ADD COLUMN webhook_url TEXT;
ALTER TABLE accounts ADD COLUMN schedule_time TEXT;
ALTER TABLE accounts ADD COLUMN schedule_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE accounts ADD COLUMN last_run_at DATETIME;

CREATE TABLE IF NOT EXISTS webhook_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    webhook_url TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'ok',
    payload TEXT NOT NULL,
    response TEXT NOT NULL,
    ran_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_webhook_logs_account_id ON webhook_logs(account_id);