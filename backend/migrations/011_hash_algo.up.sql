-- Add hash_algo columns so stored hashes are self-describing.
-- Existing rows default to 'sha256' which is the only algorithm used prior to
-- this migration.

ALTER TABLE manifest_entries
    ADD COLUMN hash_algo TEXT NOT NULL DEFAULT 'sha256';

ALTER TABLE file_versions
    ADD COLUMN hash_algo TEXT NOT NULL DEFAULT 'sha256';

ALTER TABLE quarantine_entries
    ADD COLUMN hash_algo TEXT NOT NULL DEFAULT 'sha256';

ALTER TABLE conflicts
    ADD COLUMN src_hash_algo  TEXT NOT NULL DEFAULT 'sha256';

ALTER TABLE conflicts
    ADD COLUMN dest_hash_algo TEXT NOT NULL DEFAULT 'sha256';
