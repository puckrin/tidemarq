-- SQLite does not support DROP COLUMN on older versions; recreate the table.
CREATE TABLE jobs_backup AS SELECT
    id, name, source_path, destination_path, mode, status,
    bandwidth_limit_kb, conflict_strategy, cron_schedule, watch_enabled,
    last_run_at, last_error, created_at, updated_at
FROM jobs;

DROP TABLE jobs;

ALTER TABLE jobs_backup RENAME TO jobs;
