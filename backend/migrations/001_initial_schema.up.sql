CREATE TABLE IF NOT EXISTS users (
    id           INTEGER  PRIMARY KEY AUTOINCREMENT,
    username     TEXT     NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    role         TEXT     NOT NULL CHECK(role IN ('admin', 'operator', 'viewer')),
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
