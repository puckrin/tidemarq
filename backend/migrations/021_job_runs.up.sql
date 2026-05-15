-- Persist the per-run summary for every job execution. Used to:
--   1. Show a "last run took N minutes" prior in ETA calculations on subsequent runs.
--   2. Power a future "run history" view in the UI without recomputing from the audit log.
-- The jobs.last_run_at / jobs.last_error columns remain authoritative for the most
-- recent status, but they only hold a single value; job_runs keeps the full history.
CREATE TABLE IF NOT EXISTS job_runs (
    id                INTEGER  PRIMARY KEY AUTOINCREMENT,
    job_id            INTEGER  NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    started_at        DATETIME NOT NULL,
    ended_at          DATETIME NOT NULL,
    outcome           TEXT     NOT NULL CHECK(outcome IN ('completed', 'paused', 'error')),
    files_copied      INTEGER  NOT NULL DEFAULT 0,
    files_skipped     INTEGER  NOT NULL DEFAULT 0,
    files_quarantined INTEGER  NOT NULL DEFAULT 0,
    files_errored     INTEGER  NOT NULL DEFAULT 0,
    bytes_reviewed    INTEGER  NOT NULL DEFAULT 0,
    bytes_copied      INTEGER  NOT NULL DEFAULT 0,
    error_message     TEXT
);

CREATE INDEX IF NOT EXISTS idx_job_runs_job_id_started_at ON job_runs(job_id, started_at DESC);
