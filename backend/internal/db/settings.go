package db

import (
	"context"
	"time"
)

// AppSettings holds the application-wide configuration stored in the database.
type AppSettings struct {
	VersionsToKeep          int       `json:"versions_to_keep"`
	QuarantineRetentionDays int       `json:"quarantine_retention_days"`
	UpdatedAt               time.Time `json:"updated_at"`
}

// GetSettings returns the current application settings.
// Returns safe defaults if the row is missing for any reason.
func (db *DB) GetSettings(ctx context.Context) (*AppSettings, error) {
	row := db.DB.QueryRowContext(ctx,
		`SELECT versions_to_keep, quarantine_retention_days, updated_at FROM settings WHERE id = 1`)
	var s AppSettings
	if err := row.Scan(&s.VersionsToKeep, &s.QuarantineRetentionDays, &s.UpdatedAt); err != nil {
		return &AppSettings{VersionsToKeep: 10, QuarantineRetentionDays: 30, UpdatedAt: time.Now()}, nil
	}
	return &s, nil
}

// UpdateSettings persists new values and returns the saved settings.
func (db *DB) UpdateSettings(ctx context.Context, versionsToKeep, quarantineRetentionDays int) (*AppSettings, error) {
	now := time.Now().UTC()
	_, err := db.DB.ExecContext(ctx,
		`UPDATE settings SET versions_to_keep = ?, quarantine_retention_days = ?, updated_at = ? WHERE id = 1`,
		versionsToKeep, quarantineRetentionDays, now)
	if err != nil {
		return nil, err
	}
	return &AppSettings{
		VersionsToKeep:          versionsToKeep,
		QuarantineRetentionDays: quarantineRetentionDays,
		UpdatedAt:               now,
	}, nil
}

// PruneFileVersions deletes versions beyond the newest keepCount for a given
// job+file, removing the stored files from disk. Returns the paths removed.
// If keepCount is 0, all versions are kept (unlimited).
func (db *DB) PruneFileVersions(ctx context.Context, jobID int64, relPath string, keepCount int) ([]string, error) {
	if keepCount <= 0 {
		return nil, nil
	}
	rows, err := db.DB.QueryContext(ctx,
		`SELECT id, stored_path FROM file_versions
		 WHERE job_id = ? AND rel_path = ?
		 ORDER BY created_at DESC
		 LIMIT -1 OFFSET ?`,
		jobID, relPath, keepCount)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type row struct {
		id   int64
		path string
	}
	var toDelete []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.path); err != nil {
			return nil, err
		}
		toDelete = append(toDelete, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var removed []string
	for _, r := range toDelete {
		if _, err := db.DB.ExecContext(ctx, `DELETE FROM file_versions WHERE id = ?`, r.id); err != nil {
			return removed, err
		}
		removed = append(removed, r.path)
	}
	return removed, nil
}
