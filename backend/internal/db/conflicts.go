package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Conflict represents a detected sync conflict awaiting resolution.
type Conflict struct {
	ID          int64      `json:"id"`
	JobID       int64      `json:"job_id"`
	RelPath     string     `json:"rel_path"`
	SrcSHA256   string     `json:"src_sha256"`
	DestSHA256  string     `json:"dest_sha256"`
	SrcModTime  time.Time  `json:"src_mod_time"`
	DestModTime time.Time  `json:"dest_mod_time"`
	SrcSize     int64      `json:"src_size"`
	DestSize    int64      `json:"dest_size"`
	Strategy    string     `json:"strategy"`
	Status      string     `json:"status"`
	Resolution  *string    `json:"resolution,omitempty"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// CreateConflictParams holds fields required to record a new conflict.
type CreateConflictParams struct {
	JobID       int64
	RelPath     string
	SrcSHA256   string
	DestSHA256  string
	SrcModTime  time.Time
	DestModTime time.Time
	SrcSize     int64
	DestSize    int64
	Strategy    string
}

// CreateConflict records a new conflict and returns the created row.
func (db *DB) CreateConflict(ctx context.Context, p CreateConflictParams) (*Conflict, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO conflicts
		     (job_id, rel_path, src_sha256, dest_sha256, src_mod_time, dest_mod_time,
		      src_size, dest_size, strategy, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending')`,
		p.JobID, p.RelPath, p.SrcSHA256, p.DestSHA256,
		p.SrcModTime, p.DestModTime, p.SrcSize, p.DestSize, p.Strategy,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return db.GetConflict(ctx, id)
}

// GetConflict retrieves a conflict by ID.
func (db *DB) GetConflict(ctx context.Context, id int64) (*Conflict, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, job_id, rel_path, src_sha256, dest_sha256, src_mod_time, dest_mod_time,
		        src_size, dest_size, strategy, status, resolution, resolved_at, created_at
		 FROM conflicts WHERE id = ?`, id,
	)
	return scanConflict(row)
}

// ListConflicts returns all conflicts, optionally filtered by job ID.
// Pass jobID=0 to list all.
func (db *DB) ListConflicts(ctx context.Context, jobID int64) ([]*Conflict, error) {
	var rows *sql.Rows
	var err error
	if jobID != 0 {
		rows, err = db.QueryContext(ctx,
			`SELECT id, job_id, rel_path, src_sha256, dest_sha256, src_mod_time, dest_mod_time,
			        src_size, dest_size, strategy, status, resolution, resolved_at, created_at
			 FROM conflicts WHERE job_id = ? ORDER BY created_at DESC`, jobID,
		)
	} else {
		rows, err = db.QueryContext(ctx,
			`SELECT id, job_id, rel_path, src_sha256, dest_sha256, src_mod_time, dest_mod_time,
			        src_size, dest_size, strategy, status, resolution, resolved_at, created_at
			 FROM conflicts ORDER BY created_at DESC`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Conflict
	for rows.Next() {
		c, err := scanConflictRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ResolveConflict marks a conflict as resolved with the given resolution string.
func (db *DB) ResolveConflict(ctx context.Context, id int64, resolution, status string) error {
	now := time.Now().UTC()
	_, err := db.ExecContext(ctx,
		`UPDATE conflicts SET status = ?, resolution = ?, resolved_at = ? WHERE id = ?`,
		status, resolution, now, id,
	)
	return err
}

func scanConflict(row *sql.Row) (*Conflict, error) {
	c := &Conflict{}
	err := row.Scan(
		&c.ID, &c.JobID, &c.RelPath,
		&c.SrcSHA256, &c.DestSHA256,
		&c.SrcModTime, &c.DestModTime,
		&c.SrcSize, &c.DestSize,
		&c.Strategy, &c.Status,
		&c.Resolution, &c.ResolvedAt, &c.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

func scanConflictRows(rows *sql.Rows) (*Conflict, error) {
	c := &Conflict{}
	err := rows.Scan(
		&c.ID, &c.JobID, &c.RelPath,
		&c.SrcSHA256, &c.DestSHA256,
		&c.SrcModTime, &c.DestModTime,
		&c.SrcSize, &c.DestSize,
		&c.Strategy, &c.Status,
		&c.Resolution, &c.ResolvedAt, &c.CreatedAt,
	)
	return c, err
}
