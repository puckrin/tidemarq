DROP INDEX IF EXISTS idx_audit_log_job_created;

CREATE INDEX IF NOT EXISTS idx_audit_log_job_id ON audit_log (job_id);
