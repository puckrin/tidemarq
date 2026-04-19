CREATE INDEX IF NOT EXISTS idx_quarantine_job_status ON quarantine_entries (job_id, status);
