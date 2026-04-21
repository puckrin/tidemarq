-- SQLite does not support DROP COLUMN before 3.35.0; recreate the table instead.
CREATE TABLE settings_new (
  id                        INTEGER PRIMARY KEY CHECK (id = 1),
  versions_to_keep          INTEGER NOT NULL DEFAULT 10,
  quarantine_retention_days INTEGER NOT NULL DEFAULT 30,
  updated_at                DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO settings_new SELECT id, versions_to_keep, quarantine_retention_days, updated_at FROM settings;
DROP TABLE settings;
ALTER TABLE settings_new RENAME TO settings;
