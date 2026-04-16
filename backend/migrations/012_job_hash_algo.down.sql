-- Recreate jobs table without hash_algo column.
CREATE TABLE jobs_new (
    id                 INTEGER  PRIMARY KEY AUTOINCREMENT,
    name               TEXT     NOT NULL UNIQUE,
    source_path        TEXT     NOT NULL DEFAULT '',
    destination_path   TEXT     NOT NULL DEFAULT '',
    source_mount_id    INTEGER  REFERENCES mounts(id) ON DELETE SET NULL,
    dest_mount_id      INTEGER  REFERENCES mounts(id) ON DELETE SET NULL,
    mode               TEXT     NOT NULL DEFAULT 'one-way-backup',
    status             TEXT     NOT NULL DEFAULT 'idle',
    bandwidth_limit_kb INTEGER  NOT NULL DEFAULT 0,
    conflict_strategy  TEXT     NOT NULL DEFAULT 'ask-user',
    cron_schedule      TEXT     NOT NULL DEFAULT '',
    watch_enabled      INTEGER  NOT NULL DEFAULT 0,
    full_checksum      INTEGER  NOT NULL DEFAULT 0,
    last_run_at        DATETIME,
    last_error         TEXT,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO jobs_new
    SELECT id, name, source_path, destination_path, source_mount_id, dest_mount_id,
           mode, status, bandwidth_limit_kb, conflict_strategy, cron_schedule,
           watch_enabled, full_checksum, last_run_at, last_error, created_at, updated_at
    FROM jobs;
DROP TABLE jobs;
ALTER TABLE jobs_new RENAME TO jobs;
