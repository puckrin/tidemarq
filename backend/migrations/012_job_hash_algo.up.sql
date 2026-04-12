-- Add hash_algo to jobs so each job can use a different file integrity algorithm.
-- Existing jobs default to 'sha256' to preserve compatibility with already-stored
-- manifest hashes.  New jobs created via the API default to 'blake3'.
ALTER TABLE jobs
    ADD COLUMN hash_algo TEXT NOT NULL DEFAULT 'sha256';
