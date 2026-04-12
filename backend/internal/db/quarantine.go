package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// QuarantineEntry represents a soft-deleted file held in quarantine.
type QuarantineEntry struct {
	ID             int64      `json:"id"`
	JobID          int64      `json:"job_id"`
	RelPath        string     `json:"rel_path"`
	QuarantinePath string     `json:"quarantine_path"`
	ContentHash    string     `json:"content_hash"`
	HashAlgo       string     `json:"hash_algo"`
	SizeBytes      int64      `json:"size_bytes"`
	DeletedAt      time.Time  `json:"deleted_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	// Status is "active" while the entry is in quarantine; "restored" after the
	// file was put back to the destination; "deleted" after permanent removal.
	Status    string     `json:"status"`
	RemovedAt *time.Time `json:"removed_at,omitempty"`
}

// CreateQuarantineEntry records a newly quarantined file.
func (db *DB) CreateQuarantineEntry(ctx context.Context, e *QuarantineEntry) (*QuarantineEntry, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO quarantine_entries
		     (job_id, rel_path, quarantine_path, sha256, hash_algo, size_bytes, deleted_at, expires_at, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'active')`,
		e.JobID, e.RelPath, e.QuarantinePath, e.ContentHash, e.HashAlgo, e.SizeBytes, e.DeletedAt, e.ExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return db.GetQuarantineEntry(ctx, id)
}

// GetQuarantineEntry retrieves a quarantine entry by ID.
func (db *DB) GetQuarantineEntry(ctx context.Context, id int64) (*QuarantineEntry, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, job_id, rel_path, quarantine_path, sha256, hash_algo, size_bytes, deleted_at, expires_at,
		        status, removed_at
		 FROM quarantine_entries WHERE id = ?`, id,
	)
	e := &QuarantineEntry{}
	err := row.Scan(&e.ID, &e.JobID, &e.RelPath, &e.QuarantinePath,
		&e.ContentHash, &e.HashAlgo, &e.SizeBytes, &e.DeletedAt, &e.ExpiresAt,
		&e.Status, &e.RemovedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return e, err
}

// ListQuarantineEntries returns active quarantine entries, optionally filtered by job.
// Pass jobID=0 to list all active entries.
func (db *DB) ListQuarantineEntries(ctx context.Context, jobID int64) ([]*QuarantineEntry, error) {
	var rows *sql.Rows
	var err error
	if jobID != 0 {
		rows, err = db.QueryContext(ctx,
			`SELECT id, job_id, rel_path, quarantine_path, sha256, hash_algo, size_bytes, deleted_at, expires_at,
			        status, removed_at
			 FROM quarantine_entries WHERE status = 'active' AND job_id = ? ORDER BY deleted_at DESC`, jobID,
		)
	} else {
		rows, err = db.QueryContext(ctx,
			`SELECT id, job_id, rel_path, quarantine_path, sha256, hash_algo, size_bytes, deleted_at, expires_at,
			        status, removed_at
			 FROM quarantine_entries WHERE status = 'active' ORDER BY deleted_at DESC`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanQuarantineRows(rows)
}

// ListRemovedQuarantineEntries returns entries that have been restored or permanently
// deleted, optionally filtered by job. Pass jobID=0 to list across all jobs.
func (db *DB) ListRemovedQuarantineEntries(ctx context.Context, jobID int64) ([]*QuarantineEntry, error) {
	var rows *sql.Rows
	var err error
	if jobID != 0 {
		rows, err = db.QueryContext(ctx,
			`SELECT id, job_id, rel_path, quarantine_path, sha256, hash_algo, size_bytes, deleted_at, expires_at,
			        status, removed_at
			 FROM quarantine_entries WHERE status != 'active' AND job_id = ? ORDER BY removed_at DESC`, jobID,
		)
	} else {
		rows, err = db.QueryContext(ctx,
			`SELECT id, job_id, rel_path, quarantine_path, sha256, hash_algo, size_bytes, deleted_at, expires_at,
			        status, removed_at
			 FROM quarantine_entries WHERE status != 'active' ORDER BY removed_at DESC`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanQuarantineRows(rows)
}

// MarkQuarantineRemoved transitions a quarantine entry out of the active state.
// status must be one of "restored" or "deleted".
func (db *DB) MarkQuarantineRemoved(ctx context.Context, id int64, status string) error {
	now := time.Now().UTC()
	_, err := db.ExecContext(ctx,
		`UPDATE quarantine_entries SET status = ?, removed_at = ? WHERE id = ?`,
		status, now, id,
	)
	return err
}

// DeleteQuarantineEntry permanently removes a quarantine record from the database.
// This is used by ClearRemovedQuarantineEntries and automated expiry.
func (db *DB) DeleteQuarantineEntry(ctx context.Context, id int64) error {
	_, err := db.ExecContext(ctx, `DELETE FROM quarantine_entries WHERE id = ?`, id)
	return err
}

// ClearRemovedQuarantineEntries permanently deletes all non-active quarantine records,
// optionally scoped to a single job. Pass jobID=0 to clear all jobs.
func (db *DB) ClearRemovedQuarantineEntries(ctx context.Context, jobID int64) error {
	if jobID != 0 {
		_, err := db.ExecContext(ctx,
			`DELETE FROM quarantine_entries WHERE status != 'active' AND job_id = ?`, jobID,
		)
		return err
	}
	_, err := db.ExecContext(ctx, `DELETE FROM quarantine_entries WHERE status != 'active'`)
	return err
}

func scanQuarantineRows(rows *sql.Rows) ([]*QuarantineEntry, error) {
	var out []*QuarantineEntry
	for rows.Next() {
		e := &QuarantineEntry{}
		if err := rows.Scan(&e.ID, &e.JobID, &e.RelPath, &e.QuarantinePath,
			&e.ContentHash, &e.HashAlgo, &e.SizeBytes, &e.DeletedAt, &e.ExpiresAt,
			&e.Status, &e.RemovedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
