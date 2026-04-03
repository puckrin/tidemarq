-- SQLite does not support DROP COLUMN before version 3.35.0.
-- Recreate the table without the trigger columns.
CREATE TABLE jobs_backup AS SELECT id, name, source_path, destination_path, mode, status, bandwidth_limit_kb, last_run_at, last_error, created_at, updated_at FROM jobs;
DROP TABLE jobs;
ALTER TABLE jobs_backup RENAME TO jobs;
