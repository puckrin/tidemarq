package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Conflict represents a detected sync conflict awaiting resolution.
type Conflict struct {
	ID              int64      `json:"id"`
	JobID           int64      `json:"job_id"`
	RelPath         string     `json:"rel_path"`
	SrcContentHash  string     `json:"src_content_hash"`
	DestContentHash string     `json:"dest_content_hash"`
	SrcHashAlgo     string     `json:"src_hash_algo"`
	DestHashAlgo    string     `json:"dest_hash_algo"`
	SrcModTime      time.Time  `json:"src_mod_time"`
	DestModTime     time.Time  `json:"dest_mod_time"`
	SrcSize         int64      `json:"src_size"`
	DestSize        int64      `json:"dest_size"`
	Strategy        string     `json:"strategy"`
	Status          string     `json:"status"`
	ConflictPath    *string    `json:"conflict_path,omitempty"`
	Resolution      *string    `json:"resolution,omitempty"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// CreateConflictParams holds fields required to record a new conflict.
type CreateConflictParams struct {
	JobID           int64
	RelPath         string
	SrcContentHash  string
	DestContentHash string
	SrcHashAlgo     string
	DestHashAlgo    string
	SrcModTime      time.Time
	DestModTime     time.Time
	SrcSize         int64
	DestSize        int64
	Strategy        string
	ConflictPath    string // path of the .conflict.<ts> file, empty if not created
}

// CreateConflict records a new conflict and returns the created row.
// If a pending conflict already exists for the same job and relative path it is
// returned unchanged — this keeps repeated sync runs idempotent while a conflict
// is awaiting resolution.
func (db *DB) CreateConflict(ctx context.Context, p CreateConflictParams) (*Conflict, error) {
	existing := db.QueryRowContext(ctx,
		`SELECT id, job_id, rel_path, src_sha256, dest_sha256, src_hash_algo, dest_hash_algo,
		        src_mod_time, dest_mod_time, src_size, dest_size,
		        strategy, status, conflict_path, resolution, resolved_at, created_at
		 FROM conflicts WHERE job_id = ? AND rel_path = ? AND status = 'pending'`,
		p.JobID, p.RelPath,
	)
	if c, err := scanConflict(existing); err == nil {
		return c, nil
	}

	var conflictPath *string
	if p.ConflictPath != "" {
		conflictPath = &p.ConflictPath
	}
	res, err := db.ExecContext(ctx,
		`INSERT INTO conflicts
		     (job_id, rel_path, src_sha256, dest_sha256, src_hash_algo, dest_hash_algo,
		      src_mod_time, dest_mod_time, src_size, dest_size, strategy, status, conflict_path)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?)`,
		p.JobID, p.RelPath, p.SrcContentHash, p.DestContentHash, p.SrcHashAlgo, p.DestHashAlgo,
		p.SrcModTime, p.DestModTime, p.SrcSize, p.DestSize, p.Strategy, conflictPath,
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
		`SELECT id, job_id, rel_path, src_sha256, dest_sha256, src_hash_algo, dest_hash_algo,
		        src_mod_time, dest_mod_time, src_size, dest_size,
		        strategy, status, conflict_path, resolution, resolved_at, created_at
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
			`SELECT id, job_id, rel_path, src_sha256, dest_sha256, src_hash_algo, dest_hash_algo,
			        src_mod_time, dest_mod_time, src_size, dest_size,
			        strategy, status, conflict_path, resolution, resolved_at, created_at
			 FROM conflicts WHERE job_id = ? ORDER BY created_at DESC`, jobID,
		)
	} else {
		rows, err = db.QueryContext(ctx,
			`SELECT id, job_id, rel_path, src_sha256, dest_sha256, src_hash_algo, dest_hash_algo,
			        src_mod_time, dest_mod_time, src_size, dest_size,
			        strategy, status, conflict_path, resolution, resolved_at, created_at
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

// DeleteResolvedConflicts removes all non-pending conflict records, optionally
// scoped to a single job.  Pass jobID=0 to clear across all jobs.
func (db *DB) DeleteResolvedConflicts(ctx context.Context, jobID int64) error {
	if jobID != 0 {
		_, err := db.ExecContext(ctx,
			`DELETE FROM conflicts WHERE status != 'pending' AND job_id = ?`, jobID,
		)
		return err
	}
	_, err := db.ExecContext(ctx, `DELETE FROM conflicts WHERE status != 'pending'`)
	return err
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
		&c.SrcContentHash, &c.DestContentHash,
		&c.SrcHashAlgo, &c.DestHashAlgo,
		&c.SrcModTime, &c.DestModTime,
		&c.SrcSize, &c.DestSize,
		&c.Strategy, &c.Status,
		&c.ConflictPath, &c.Resolution, &c.ResolvedAt, &c.CreatedAt,
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
		&c.SrcContentHash, &c.DestContentHash,
		&c.SrcHashAlgo, &c.DestHashAlgo,
		&c.SrcModTime, &c.DestModTime,
		&c.SrcSize, &c.DestSize,
		&c.Strategy, &c.Status,
		&c.ConflictPath, &c.Resolution, &c.ResolvedAt, &c.CreatedAt,
	)
	return c, err
}
