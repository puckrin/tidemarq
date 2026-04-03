package db

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"time"
)

// ManifestEntry represents a row in the manifest_entries table.
type ManifestEntry struct {
	ID          int64
	JobID       int64
	RelPath     string
	SHA256      string
	SizeBytes   int64
	ModTime     time.Time
	Permissions fs.FileMode
	SyncedAt    time.Time
}

// GetManifestEntry retrieves the manifest entry for relPath within jobID.
// Returns ErrNotFound if no entry exists.
func (db *DB) GetManifestEntry(ctx context.Context, jobID int64, relPath string) (*ManifestEntry, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, job_id, rel_path, sha256, size_bytes, mod_time, permissions, synced_at
		 FROM manifest_entries WHERE job_id = ? AND rel_path = ?`,
		jobID, relPath,
	)
	e := &ManifestEntry{}
	var perm int64
	err := row.Scan(&e.ID, &e.JobID, &e.RelPath, &e.SHA256, &e.SizeBytes, &e.ModTime, &perm, &e.SyncedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	e.Permissions = fs.FileMode(perm)
	return e, nil
}

// UpsertManifestEntry creates or updates a manifest entry.
func (db *DB) UpsertManifestEntry(ctx context.Context, e *ManifestEntry) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO manifest_entries (job_id, rel_path, sha256, size_bytes, mod_time, permissions, synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(job_id, rel_path) DO UPDATE SET
		     sha256       = excluded.sha256,
		     size_bytes   = excluded.size_bytes,
		     mod_time     = excluded.mod_time,
		     permissions  = excluded.permissions,
		     synced_at    = excluded.synced_at`,
		e.JobID, e.RelPath, e.SHA256, e.SizeBytes, e.ModTime, int64(e.Permissions), e.SyncedAt,
	)
	return err
}

// DeleteManifestEntry removes the manifest entry for the given job and path.
func (db *DB) DeleteManifestEntry(ctx context.Context, jobID int64, relPath string) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM manifest_entries WHERE job_id = ? AND rel_path = ?`, jobID, relPath,
	)
	return err
}

// ListManifestEntries returns all manifest entries for jobID ordered by path.
func (db *DB) ListManifestEntries(ctx context.Context, jobID int64) ([]*ManifestEntry, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, job_id, rel_path, sha256, size_bytes, mod_time, permissions, synced_at
		 FROM manifest_entries WHERE job_id = ? ORDER BY rel_path`,
		jobID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*ManifestEntry
	for rows.Next() {
		e := &ManifestEntry{}
		var perm int64
		if err := rows.Scan(&e.ID, &e.JobID, &e.RelPath, &e.SHA256, &e.SizeBytes, &e.ModTime, &perm, &e.SyncedAt); err != nil {
			return nil, err
		}
		e.Permissions = fs.FileMode(perm)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
