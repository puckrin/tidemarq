-- Add conflict resolution strategy to jobs.
ALTER TABLE jobs ADD COLUMN conflict_strategy TEXT NOT NULL DEFAULT 'ask-user'
    CHECK(conflict_strategy IN ('ask-user','newest-wins','largest-wins','source-wins','destination-wins'));

-- Track detected conflicts awaiting resolution.
CREATE TABLE IF NOT EXISTS conflicts (
    id              INTEGER  PRIMARY KEY AUTOINCREMENT,
    job_id          INTEGER  NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    rel_path        TEXT     NOT NULL,
    src_sha256      TEXT     NOT NULL,
    dest_sha256     TEXT     NOT NULL,
    src_mod_time    DATETIME NOT NULL,
    dest_mod_time   DATETIME NOT NULL,
    src_size        INTEGER  NOT NULL,
    dest_size       INTEGER  NOT NULL,
    strategy        TEXT     NOT NULL,
    status          TEXT     NOT NULL DEFAULT 'pending'
                    CHECK(status IN ('pending','resolved','auto-resolved')),
    resolution      TEXT,
    resolved_at     DATETIME,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Version history: snapshot of a file before it is overwritten.
CREATE TABLE IF NOT EXISTS file_versions (
    id              INTEGER  PRIMARY KEY AUTOINCREMENT,
    job_id          INTEGER  NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    rel_path        TEXT     NOT NULL,
    version_num     INTEGER  NOT NULL,
    stored_path     TEXT     NOT NULL,
    sha256          TEXT     NOT NULL,
    size_bytes      INTEGER  NOT NULL,
    mod_time        DATETIME NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Soft-deleted files awaiting expiry.
CREATE TABLE IF NOT EXISTS quarantine_entries (
    id              INTEGER  PRIMARY KEY AUTOINCREMENT,
    job_id          INTEGER  NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    rel_path        TEXT     NOT NULL,
    quarantine_path TEXT     NOT NULL,
    sha256          TEXT     NOT NULL,
    size_bytes      INTEGER  NOT NULL,
    deleted_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at      DATETIME NOT NULL
);
