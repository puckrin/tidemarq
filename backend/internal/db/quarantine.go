package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// QuarantineEntry represents a soft-deleted file held in quarantine.
type QuarantineEntry struct {
	ID             int64     `json:"id"`
	JobID          int64     `json:"job_id"`
	RelPath        string    `json:"rel_path"`
	QuarantinePath string    `json:"quarantine_path"`
	SHA256         string    `json:"sha256"`
	SizeBytes      int64     `json:"size_bytes"`
	DeletedAt      time.Time `json:"deleted_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

// CreateQuarantineEntry records a newly quarantined file.
func (db *DB) CreateQuarantineEntry(ctx context.Context, e *QuarantineEntry) (*QuarantineEntry, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO quarantine_entries
		     (job_id, rel_path, quarantine_path, sha256, size_bytes, deleted_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.JobID, e.RelPath, e.QuarantinePath, e.SHA256, e.SizeBytes, e.DeletedAt, e.ExpiresAt,
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
		`SELECT id, job_id, rel_path, quarantine_path, sha256, size_bytes, deleted_at, expires_at
		 FROM quarantine_entries WHERE id = ?`, id,
	)
	e := &QuarantineEntry{}
	err := row.Scan(&e.ID, &e.JobID, &e.RelPath, &e.QuarantinePath,
		&e.SHA256, &e.SizeBytes, &e.DeletedAt, &e.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return e, err
}

// ListQuarantineEntries returns all quarantine entries, optionally filtered by job.
// Pass jobID=0 to list all.
func (db *DB) ListQuarantineEntries(ctx context.Context, jobID int64) ([]*QuarantineEntry, error) {
	var rows *sql.Rows
	var err error
	if jobID != 0 {
		rows, err = db.QueryContext(ctx,
			`SELECT id, job_id, rel_path, quarantine_path, sha256, size_bytes, deleted_at, expires_at
			 FROM quarantine_entries WHERE job_id = ? ORDER BY deleted_at DESC`, jobID,
		)
	} else {
		rows, err = db.QueryContext(ctx,
			`SELECT id, job_id, rel_path, quarantine_path, sha256, size_bytes, deleted_at, expires_at
			 FROM quarantine_entries ORDER BY deleted_at DESC`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*QuarantineEntry
	for rows.Next() {
		e := &QuarantineEntry{}
		if err := rows.Scan(&e.ID, &e.JobID, &e.RelPath, &e.QuarantinePath,
			&e.SHA256, &e.SizeBytes, &e.DeletedAt, &e.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DeleteQuarantineEntry removes a quarantine record (used after restore or expiry).
func (db *DB) DeleteQuarantineEntry(ctx context.Context, id int64) error {
	_, err := db.ExecContext(ctx, `DELETE FROM quarantine_entries WHERE id = ?`, id)
	return err
}
