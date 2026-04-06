package db

import (
	"context"
	"time"
)

// AuditEntry represents a row in the audit_log table.
type AuditEntry struct {
	ID        int64     `json:"id"`
	JobID     *int64    `json:"job_id,omitempty"`
	JobName   string    `json:"job_name"`
	Actor     string    `json:"actor"`
	Event     string    `json:"event"`
	Message   string    `json:"message"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateAuditEntryParams holds the fields for a new audit log entry.
type CreateAuditEntryParams struct {
	JobID   *int64
	JobName string
	Actor   string
	Event   string
	Message string
	Detail  string
}

// CreateAuditEntry inserts a new entry into the audit log.
func (db *DB) CreateAuditEntry(ctx context.Context, p CreateAuditEntryParams) (*AuditEntry, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO audit_log (job_id, job_name, actor, event, message, detail) VALUES (?, ?, ?, ?, ?, ?)`,
		p.JobID, p.JobName, p.Actor, p.Event, p.Message, p.Detail,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return db.GetAuditEntry(ctx, id)
}

// GetAuditEntry retrieves a single entry by ID.
func (db *DB) GetAuditEntry(ctx context.Context, id int64) (*AuditEntry, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, job_id, job_name, actor, event, message, detail, created_at FROM audit_log WHERE id = ?`, id,
	)
	e := &AuditEntry{}
	err := row.Scan(&e.ID, &e.JobID, &e.JobName, &e.Actor, &e.Event, &e.Message, &e.Detail, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return e, nil
}

// AuditFilter holds optional query constraints for ListAuditEntries.
type AuditFilter struct {
	JobID     *int64
	Event     string
	Since     *time.Time
	Until     *time.Time
	Limit     int    // 0 = default 500
	Offset    int
}

// ListAuditEntries returns entries matching the filter, newest first.
func (db *DB) ListAuditEntries(ctx context.Context, f AuditFilter) ([]*AuditEntry, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 500
	}

	query := `SELECT id, job_id, job_name, actor, event, message, detail, created_at
	          FROM audit_log WHERE 1=1`
	args := []any{}

	if f.JobID != nil {
		query += ` AND job_id = ?`
		args = append(args, *f.JobID)
	}
	if f.Event != "" {
		query += ` AND event = ?`
		args = append(args, f.Event)
	}
	if f.Since != nil {
		query += ` AND created_at >= ?`
		args = append(args, f.Since.UTC())
	}
	if f.Until != nil {
		query += ` AND created_at <= ?`
		args = append(args, f.Until.UTC())
	}

	query += ` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, f.Offset)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*AuditEntry
	for rows.Next() {
		e := &AuditEntry{}
		if err := rows.Scan(&e.ID, &e.JobID, &e.JobName, &e.Actor, &e.Event, &e.Message, &e.Detail, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
