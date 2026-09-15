DROP TABLE IF EXISTS webhook_logs;

ALTER TABLE accounts DROP COLUMN last_run_at;
ALTER TABLE accounts DROP COLUMN schedule_enabled;
ALTER TABLE accounts DROP COLUMN schedule_time;
ALTER TABLE accounts DROP COLUMN webhook_url;