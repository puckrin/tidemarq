CREATE TABLE IF NOT EXISTS settings (
  id                        INTEGER PRIMARY KEY CHECK (id = 1),
  versions_to_keep          INTEGER NOT NULL DEFAULT 10,
  quarantine_retention_days INTEGER NOT NULL DEFAULT 30,
  updated_at                DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Seed the single settings row on first migration.
INSERT OR IGNORE INTO settings (id, versions_to_keep, quarantine_retention_days, updated_at)
VALUES (1, 10, 30, CURRENT_TIMESTAMP);
