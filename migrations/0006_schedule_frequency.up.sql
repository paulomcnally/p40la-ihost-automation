ALTER TABLE accounts ADD COLUMN schedule_frequency TEXT NOT NULL DEFAULT 'daily';
ALTER TABLE accounts ADD COLUMN schedule_interval INTEGER;
ALTER TABLE accounts ADD COLUMN schedule_days TEXT;