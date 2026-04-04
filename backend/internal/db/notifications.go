package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// NotificationTarget holds connection details for one delivery channel.
type NotificationTarget struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	ConfigEnc []byte    `json:"-"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NotificationRule maps an event to a target, optionally scoped to a job.
type NotificationRule struct {
	ID        int64     `json:"id"`
	TargetID  int64     `json:"target_id"`
	Event     string    `json:"event"`
	JobID     *int64    `json:"job_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateNotificationTarget inserts a target and returns the created record.
func (db *DB) CreateNotificationTarget(ctx context.Context, name, typ string, configEnc []byte) (*NotificationTarget, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO notification_targets (name, type, config_enc) VALUES (?, ?, ?)`,
		name, typ, configEnc,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return db.GetNotificationTarget(ctx, id)
}

// GetNotificationTarget retrieves a target by ID.
func (db *DB) GetNotificationTarget(ctx context.Context, id int64) (*NotificationTarget, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, name, type, config_enc, enabled, created_at, updated_at
		 FROM notification_targets WHERE id = ?`, id,
	)
	return scanNotificationTarget(row)
}

// ListNotificationTargets returns all targets ordered by name.
func (db *DB) ListNotificationTargets(ctx context.Context) ([]*NotificationTarget, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, name, type, config_enc, enabled, created_at, updated_at
		 FROM notification_targets ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []*NotificationTarget
	for rows.Next() {
		t := &NotificationTarget{}
		var enabled int
		if err := rows.Scan(&t.ID, &t.Name, &t.Type, &t.ConfigEnc, &enabled, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.Enabled = enabled != 0
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

// UpdateNotificationTarget applies non-nil fields to the target.
func (db *DB) UpdateNotificationTarget(ctx context.Context, id int64, name string, configEnc []byte, enabled bool) (*NotificationTarget, error) {
	_, err := db.ExecContext(ctx,
		`UPDATE notification_targets SET name = ?, config_enc = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		name, configEnc, enabled, id,
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return db.GetNotificationTarget(ctx, id)
}

// DeleteNotificationTarget removes a target by ID (cascades to rules).
func (db *DB) DeleteNotificationTarget(ctx context.Context, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM notification_targets WHERE id = ?`, id)
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

// CreateNotificationRule adds a rule and returns the created record.
func (db *DB) CreateNotificationRule(ctx context.Context, targetID int64, event string, jobID *int64) (*NotificationRule, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO notification_rules (target_id, event, job_id) VALUES (?, ?, ?)`,
		targetID, event, jobID,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return db.GetNotificationRule(ctx, id)
}

// GetNotificationRule retrieves a rule by ID.
func (db *DB) GetNotificationRule(ctx context.Context, id int64) (*NotificationRule, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, target_id, event, job_id, created_at FROM notification_rules WHERE id = ?`, id,
	)
	r := &NotificationRule{}
	err := row.Scan(&r.ID, &r.TargetID, &r.Event, &r.JobID, &r.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return r, err
}

// ListNotificationRules returns rules for a target (targetID=0 for all).
func (db *DB) ListNotificationRules(ctx context.Context, targetID int64) ([]*NotificationRule, error) {
	var rows *sql.Rows
	var err error
	if targetID == 0 {
		rows, err = db.QueryContext(ctx,
			`SELECT id, target_id, event, job_id, created_at FROM notification_rules ORDER BY id`,
		)
	} else {
		rows, err = db.QueryContext(ctx,
			`SELECT id, target_id, event, job_id, created_at FROM notification_rules WHERE target_id = ? ORDER BY id`,
			targetID,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []*NotificationRule
	for rows.Next() {
		r := &NotificationRule{}
		if err := rows.Scan(&r.ID, &r.TargetID, &r.Event, &r.JobID, &r.CreatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// DeleteNotificationRule removes a rule by ID.
func (db *DB) DeleteNotificationRule(ctx context.Context, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM notification_rules WHERE id = ?`, id)
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

// ListRulesForEvent returns all enabled targets whose rules match the given
// event and optionally a specific job ID.
func (db *DB) ListRulesForEvent(ctx context.Context, event string, jobID int64) ([]*NotificationTarget, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT DISTINCT t.id, t.name, t.type, t.config_enc, t.enabled, t.created_at, t.updated_at
		 FROM notification_targets t
		 JOIN notification_rules r ON r.target_id = t.id
		 WHERE t.enabled = 1
		   AND r.event = ?
		   AND (r.job_id IS NULL OR r.job_id = ?)`,
		event, jobID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var targets []*NotificationTarget
	for rows.Next() {
		t := &NotificationTarget{}
		var enabled int
		if err := rows.Scan(&t.ID, &t.Name, &t.Type, &t.ConfigEnc, &enabled, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		t.Enabled = enabled != 0
		targets = append(targets, t)
	}
	return targets, rows.Err()
}

func scanNotificationTarget(row *sql.Row) (*NotificationTarget, error) {
	t := &NotificationTarget{}
	var enabled int
	err := row.Scan(&t.ID, &t.Name, &t.Type, &t.ConfigEnc, &enabled, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.Enabled = enabled != 0
	return t, nil
}
