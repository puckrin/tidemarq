package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Job represents a row in the jobs table.
type Job struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	SourcePath       string     `json:"source_path"`
	DestinationPath  string     `json:"destination_path"`
	Mode             string     `json:"mode"`
	Status           string     `json:"status"`
	BandwidthLimitKB int64      `json:"bandwidth_limit_kb"`
	CronSchedule     string     `json:"cron_schedule"`
	WatchEnabled     bool       `json:"watch_enabled"`
	LastRunAt        *time.Time `json:"last_run_at,omitempty"`
	LastError        *string    `json:"last_error,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// CreateJobParams holds the fields required to create a job.
type CreateJobParams struct {
	Name             string
	SourcePath       string
	DestinationPath  string
	Mode             string
	BandwidthLimitKB int64
	CronSchedule     string
	WatchEnabled     bool
}

// UpdateJobParams holds the fields that may be updated on a job.
type UpdateJobParams struct {
	Name             string
	SourcePath       string
	DestinationPath  string
	Mode             string
	BandwidthLimitKB int64
	CronSchedule     string
	WatchEnabled     bool
}

// CreateJob inserts a new job and returns the created record.
func (db *DB) CreateJob(ctx context.Context, p CreateJobParams) (*Job, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO jobs (name, source_path, destination_path, mode, bandwidth_limit_kb, cron_schedule, watch_enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.SourcePath, p.DestinationPath, p.Mode, p.BandwidthLimitKB, p.CronSchedule, p.WatchEnabled,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return db.GetJobByID(ctx, id)
}

// GetJobByID retrieves a job by primary key.
func (db *DB) GetJobByID(ctx context.Context, id int64) (*Job, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, name, source_path, destination_path, mode, status, bandwidth_limit_kb,
		        cron_schedule, watch_enabled, last_run_at, last_error, created_at, updated_at
		 FROM jobs WHERE id = ?`, id,
	)
	return scanJob(row)
}

// ListJobs returns all jobs ordered by name.
func (db *DB) ListJobs(ctx context.Context) ([]*Job, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, name, source_path, destination_path, mode, status, bandwidth_limit_kb,
		        cron_schedule, watch_enabled, last_run_at, last_error, created_at, updated_at
		 FROM jobs ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		j, err := scanJobRows(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// UpdateJob applies p to the job with the given ID.
func (db *DB) UpdateJob(ctx context.Context, id int64, p UpdateJobParams) (*Job, error) {
	_, err := db.ExecContext(ctx,
		`UPDATE jobs SET name = ?, source_path = ?, destination_path = ?, mode = ?,
		                 bandwidth_limit_kb = ?, cron_schedule = ?, watch_enabled = ?,
		                 updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		p.Name, p.SourcePath, p.DestinationPath, p.Mode,
		p.BandwidthLimitKB, p.CronSchedule, p.WatchEnabled, id,
	)
	if err != nil {
		return nil, err
	}
	return db.GetJobByID(ctx, id)
}

// DeleteJob removes a job and its manifest entries (cascade).
func (db *DB) DeleteJob(ctx context.Context, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM jobs WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateJobStatus updates the status and last_error of a job.
// If setLastRun is true, last_run_at is also set to the current time.
func (db *DB) UpdateJobStatus(ctx context.Context, id int64, status string, lastError *string, setLastRun bool) error {
	if setLastRun {
		now := time.Now().UTC()
		_, err := db.ExecContext(ctx,
			`UPDATE jobs SET status = ?, last_error = ?, last_run_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			status, lastError, now, id,
		)
		return err
	}
	_, err := db.ExecContext(ctx,
		`UPDATE jobs SET status = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, lastError, id,
	)
	return err
}

func scanJob(row *sql.Row) (*Job, error) {
	j := &Job{}
	var watchEnabled int
	err := row.Scan(
		&j.ID, &j.Name, &j.SourcePath, &j.DestinationPath, &j.Mode, &j.Status,
		&j.BandwidthLimitKB, &j.CronSchedule, &watchEnabled,
		&j.LastRunAt, &j.LastError, &j.CreatedAt, &j.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	j.WatchEnabled = watchEnabled != 0
	return j, nil
}

func scanJobRows(rows *sql.Rows) (*Job, error) {
	j := &Job{}
	var watchEnabled int
	err := rows.Scan(
		&j.ID, &j.Name, &j.SourcePath, &j.DestinationPath, &j.Mode, &j.Status,
		&j.BandwidthLimitKB, &j.CronSchedule, &watchEnabled,
		&j.LastRunAt, &j.LastError, &j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	j.WatchEnabled = watchEnabled != 0
	return j, nil
}
