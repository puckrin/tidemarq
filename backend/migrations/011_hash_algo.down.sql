-- SQLite does not support DROP COLUMN in older versions; recreate affected tables.

-- manifest_entries
CREATE TABLE manifest_entries_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id      INTEGER NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    rel_path    TEXT    NOT NULL,
    sha256      TEXT    NOT NULL,
    size_bytes  INTEGER NOT NULL DEFAULT 0,
    mod_time    DATETIME NOT NULL,
    permissions INTEGER NOT NULL DEFAULT 0,
    synced_at   DATETIME NOT NULL,
    UNIQUE(job_id, rel_path)
);
INSERT INTO manifest_entries_new
    SELECT id, job_id, rel_path, sha256, size_bytes, mod_time, permissions, synced_at
    FROM manifest_entries;
DROP TABLE manifest_entries;
ALTER TABLE manifest_entries_new RENAME TO manifest_entries;

-- file_versions
CREATE TABLE file_versions_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id      INTEGER NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    rel_path    TEXT    NOT NULL,
    version_num INTEGER NOT NULL DEFAULT 1,
    stored_path TEXT    NOT NULL,
    sha256      TEXT    NOT NULL,
    size_bytes  INTEGER NOT NULL DEFAULT 0,
    mod_time    DATETIME NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO file_versions_new
    SELECT id, job_id, rel_path, version_num, stored_path, sha256, size_bytes, mod_time, created_at
    FROM file_versions;
DROP TABLE file_versions;
ALTER TABLE file_versions_new RENAME TO file_versions;

-- quarantine_entries
CREATE TABLE quarantine_entries_new (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id           INTEGER  NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    rel_path         TEXT     NOT NULL,
    quarantine_path  TEXT     NOT NULL,
    sha256           TEXT     NOT NULL,
    size_bytes       INTEGER  NOT NULL DEFAULT 0,
    deleted_at       DATETIME NOT NULL,
    expires_at       DATETIME NOT NULL,
    status           TEXT     NOT NULL DEFAULT 'active',
    removed_at       DATETIME
);
INSERT INTO quarantine_entries_new
    SELECT id, job_id, rel_path, quarantine_path, sha256, size_bytes,
           deleted_at, expires_at, status, removed_at
    FROM quarantine_entries;
DROP TABLE quarantine_entries;
ALTER TABLE quarantine_entries_new RENAME TO quarantine_entries;

-- conflicts
CREATE TABLE conflicts_new (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id        INTEGER  NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    rel_path      TEXT     NOT NULL,
    src_sha256    TEXT     NOT NULL,
    dest_sha256   TEXT     NOT NULL,
    src_mod_time  DATETIME NOT NULL,
    dest_mod_time DATETIME NOT NULL,
    src_size      INTEGER  NOT NULL DEFAULT 0,
    dest_size     INTEGER  NOT NULL DEFAULT 0,
    strategy      TEXT     NOT NULL,
    status        TEXT     NOT NULL DEFAULT 'pending',
    conflict_path TEXT,
    resolution    TEXT,
    resolved_at   DATETIME,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO conflicts_new
    SELECT id, job_id, rel_path, src_sha256, dest_sha256,
           src_mod_time, dest_mod_time, src_size, dest_size,
           strategy, status, conflict_path, resolution, resolved_at, created_at
    FROM conflicts;
DROP TABLE conflicts;
ALTER TABLE conflicts_new RENAME TO conflicts;
