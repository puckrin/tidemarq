-- Allow jobs to use network mounts as source or destination.
-- A NULL mount_id means the path is a local filesystem path.
ALTER TABLE jobs ADD COLUMN source_mount_id INTEGER REFERENCES mounts(id) ON DELETE SET NULL;
ALTER TABLE jobs ADD COLUMN dest_mount_id   INTEGER REFERENCES mounts(id) ON DELETE SET NULL;
