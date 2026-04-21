-- Add configurable retention for the audit log.
-- Default 90 days matches the spec's intent for a useful but bounded history.
ALTER TABLE settings ADD COLUMN audit_log_retention_days INTEGER NOT NULL DEFAULT 90;
UPDATE settings SET audit_log_retention_days = 90 WHERE id = 1;
