-- Replace the separate job_id and created_at indexes with a composite index
-- that satisfies the common "filter by job, order by time" query in a single
-- index scan with no post-sort step.
--
-- idx_audit_log_job_id is made redundant by the composite (job_id, created_at)
-- because SQLite can use a composite index for queries on the leftmost column.
DROP INDEX IF EXISTS idx_audit_log_job_id;

CREATE INDEX IF NOT EXISTS idx_audit_log_job_created
    ON audit_log (job_id, created_at DESC);
