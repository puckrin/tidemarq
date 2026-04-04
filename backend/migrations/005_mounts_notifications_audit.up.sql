-- Network mounts (SFTP and SMB/CIFS).
CREATE TABLE IF NOT EXISTS mounts (
    id              INTEGER  PRIMARY KEY AUTOINCREMENT,
    name            TEXT     NOT NULL UNIQUE,
    type            TEXT     NOT NULL CHECK(type IN ('sftp','smb')),
    host            TEXT     NOT NULL,
    port            INTEGER  NOT NULL DEFAULT 0,
    username        TEXT     NOT NULL DEFAULT '',
    password_enc    BLOB,
    ssh_key_enc     BLOB,
    smb_share       TEXT     NOT NULL DEFAULT '',
    smb_domain      TEXT     NOT NULL DEFAULT '',
    sftp_host_key   TEXT     NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Notification targets (SMTP, webhook, Gotify).
-- Configuration is stored AES-256-GCM encrypted as JSON.
CREATE TABLE IF NOT EXISTS notification_targets (
    id              INTEGER  PRIMARY KEY AUTOINCREMENT,
    name            TEXT     NOT NULL UNIQUE,
    type            TEXT     NOT NULL CHECK(type IN ('smtp','webhook','gotify')),
    config_enc      BLOB     NOT NULL,
    enabled         INTEGER  NOT NULL DEFAULT 1,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Notification rules: which events fire to which targets.
-- job_id NULL means the rule applies to all jobs.
CREATE TABLE IF NOT EXISTS notification_rules (
    id              INTEGER  PRIMARY KEY AUTOINCREMENT,
    target_id       INTEGER  NOT NULL REFERENCES notification_targets(id) ON DELETE CASCADE,
    event           TEXT     NOT NULL CHECK(event IN ('job_failed','job_completed','conflict_detected','job_started')),
    job_id          INTEGER  REFERENCES jobs(id) ON DELETE CASCADE,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Persistent audit log for all job lifecycle events.
CREATE TABLE IF NOT EXISTS audit_log (
    id              INTEGER  PRIMARY KEY AUTOINCREMENT,
    job_id          INTEGER  REFERENCES jobs(id) ON DELETE SET NULL,
    job_name        TEXT     NOT NULL DEFAULT '',
    actor           TEXT     NOT NULL DEFAULT '',
    event           TEXT     NOT NULL,
    message         TEXT     NOT NULL,
    detail          TEXT     NOT NULL DEFAULT '',
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_job_id     ON audit_log (job_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_event      ON audit_log (event);
