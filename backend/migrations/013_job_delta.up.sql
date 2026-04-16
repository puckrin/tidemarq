-- Add delta transfer fields to jobs.
-- use_delta: enables rolling-checksum delta transfer for local paths.
-- delta_block_size: signature block size in bytes (0 = engine default 2048).
-- delta_min_bytes: minimum file size for delta to be attempted (0 = engine default 65536).

ALTER TABLE jobs ADD COLUMN use_delta INTEGER NOT NULL DEFAULT 0;
ALTER TABLE jobs ADD COLUMN delta_block_size INTEGER NOT NULL DEFAULT 0;
ALTER TABLE jobs ADD COLUMN delta_min_bytes INTEGER NOT NULL DEFAULT 0;
