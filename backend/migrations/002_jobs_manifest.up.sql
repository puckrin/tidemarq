CREATE TABLE IF NOT EXISTS jobs (
    id                 INTEGER  PRIMARY KEY AUTOINCREMENT,
    name               TEXT     NOT NULL,
    source_path        TEXT     NOT NULL,
    destination_path   TEXT     NOT NULL,
    mode               TEXT     NOT NULL CHECK(mode IN ('one-way-backup', 'one-way-mirror', 'two-way')),
    status             TEXT     NOT NULL DEFAULT 'idle' CHECK(status IN ('idle', 'running', 'paused', 'error', 'disabled')),
    bandwidth_limit_kb INTEGER  NOT NULL DEFAULT 0,
    last_run_at        DATETIME,
    last_error         TEXT,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- One row per file per job; updated after each successful transfer.
CREATE TABLE IF NOT EXISTS manifest_entries (
    id          INTEGER  PRIMARY KEY AUTOINCREMENT,
    job_id      INTEGER  NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    rel_path    TEXT     NOT NULL,
    sha256      TEXT     NOT NULL,
    size_bytes  INTEGER  NOT NULL,
    mod_time    DATETIME NOT NULL,
    permissions INTEGER  NOT NULL,
    synced_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(job_id, rel_path)
);
