-- Track whether a quarantine entry is active, has been restored, or was deleted.
ALTER TABLE quarantine_entries ADD COLUMN status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE quarantine_entries ADD COLUMN removed_at TIMESTAMP;
