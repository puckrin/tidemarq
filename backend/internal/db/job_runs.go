package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// JobRun is a single row in the job_runs table — one entry per executed run of a job.
type JobRun struct {
	ID               int64     `json:"id"`
	JobID            int64     `json:"job_id"`
	StartedAt        time.Time `json:"started_at"`
	EndedAt          time.Time `json:"ended_at"`
	Outcome          string    `json:"outcome"` // "completed" | "paused" | "error"
	FilesCopied      int       `json:"files_copied"`
	FilesSkipped     int       `json:"files_skipped"`
	FilesQuarantined int       `json:"files_quarantined"`
	FilesErrored     int       `json:"files_errored"`
	BytesReviewed    int64     `json:"bytes_reviewed"`
	BytesCopied      int64     `json:"bytes_copied"`
	ErrorMessage     *string   `json:"error_message,omitempty"`
}

// CreateJobRunParams holds the fields for inserting a new job run.
type CreateJobRunParams struct {
	JobID            int64
	StartedAt        time.Time
	EndedAt          time.Time
	Outcome          string
	FilesCopied      int
	FilesSkipped     int
	FilesQuarantined int
	FilesErrored     int
	BytesReviewed    int64
	BytesCopied      int64
	ErrorMessage     *string
}

// CreateJobRun inserts a new run record. Idempotency is the caller's responsibility:
// this is a write-once table — one row per execution of a job.
func (db *DB) CreateJobRun(ctx context.Context, p CreateJobRunParams) (*JobRun, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO job_runs (
			job_id, started_at, ended_at, outcome,
			files_copied, files_skipped, files_quarantined, files_errored,
			bytes_reviewed, bytes_copied, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.JobID, p.StartedAt, p.EndedAt, p.Outcome,
		p.FilesCopied, p.FilesSkipped, p.FilesQuarantined, p.FilesErrored,
		p.BytesReviewed, p.BytesCopied, p.ErrorMessage,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return db.GetJobRun(ctx, id)
}

// GetJobRun retrieves a single run record by ID.
func (db *DB) GetJobRun(ctx context.Context, id int64) (*JobRun, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, job_id, started_at, ended_at, outcome,
		        files_copied, files_skipped, files_quarantined, files_errored,
		        bytes_reviewed, bytes_copied, error_message
		 FROM job_runs WHERE id = ?`, id,
	)
	return scanJobRun(row)
}

// LatestJobRun returns the most recent successfully completed run for jobID, or
// (nil, nil) if no completed run exists. Used to seed ETA estimates on the next run.
// Paused and errored runs are excluded so a stalled or failed run does not contaminate
// the baseline duration.
func (db *DB) LatestJobRun(ctx context.Context, jobID int64) (*JobRun, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, job_id, started_at, ended_at, outcome,
		        files_copied, files_skipped, files_quarantined, files_errored,
		        bytes_reviewed, bytes_copied, error_message
		 FROM job_runs
		 WHERE job_id = ? AND outcome = 'completed'
		 ORDER BY started_at DESC
		 LIMIT 1`, jobID,
	)
	run, err := scanJobRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return run, err
}

// ListJobRuns returns the most recent `limit` runs for jobID, newest first.
// A limit of 0 defaults to 50.
func (db *DB) ListJobRuns(ctx context.Context, jobID int64, limit int) ([]*JobRun, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.QueryContext(ctx,
		`SELECT id, job_id, started_at, ended_at, outcome,
		        files_copied, files_skipped, files_quarantined, files_errored,
		        bytes_reviewed, bytes_copied, error_message
		 FROM job_runs
		 WHERE job_id = ?
		 ORDER BY started_at DESC
		 LIMIT ?`, jobID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*JobRun
	for rows.Next() {
		run, err := scanJobRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanJobRun(r rowScanner) (*JobRun, error) {
	jr := &JobRun{}
	err := r.Scan(
		&jr.ID, &jr.JobID, &jr.StartedAt, &jr.EndedAt, &jr.Outcome,
		&jr.FilesCopied, &jr.FilesSkipped, &jr.FilesQuarantined, &jr.FilesErrored,
		&jr.BytesReviewed, &jr.BytesCopied, &jr.ErrorMessage,
	)
	if err != nil {
		return nil, err
	}
	return jr, nil
}
