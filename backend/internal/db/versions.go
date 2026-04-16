package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// FileVersion represents a stored version of a file before it was overwritten.
type FileVersion struct {
	ID          int64     `json:"id"`
	JobID       int64     `json:"job_id"`
	RelPath     string    `json:"rel_path"`
	VersionNum  int64     `json:"version_num"`
	StoredPath  string    `json:"stored_path"`
	ContentHash string    `json:"content_hash"`
	HashAlgo    string    `json:"hash_algo"`
	SizeBytes   int64     `json:"size_bytes"`
	ModTime     time.Time `json:"mod_time"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateFileVersion records a new version snapshot and returns the created row.
func (db *DB) CreateFileVersion(ctx context.Context, v *FileVersion) (*FileVersion, error) {
	// Determine next version number for this job+path.
	var maxVer int64
	row := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version_num), 0) FROM file_versions WHERE job_id = ? AND rel_path = ?`,
		v.JobID, v.RelPath,
	)
	if err := row.Scan(&maxVer); err != nil {
		return nil, err
	}
	v.VersionNum = maxVer + 1

	res, err := db.ExecContext(ctx,
		`INSERT INTO file_versions (job_id, rel_path, version_num, stored_path, sha256, hash_algo, size_bytes, mod_time)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		v.JobID, v.RelPath, v.VersionNum, v.StoredPath, v.ContentHash, v.HashAlgo, v.SizeBytes, v.ModTime,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return db.GetFileVersion(ctx, id)
}

// GetFileVersion retrieves a version by ID.
func (db *DB) GetFileVersion(ctx context.Context, id int64) (*FileVersion, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, job_id, rel_path, version_num, stored_path, sha256, hash_algo, size_bytes, mod_time, created_at
		 FROM file_versions WHERE id = ?`, id,
	)
	v := &FileVersion{}
	err := row.Scan(&v.ID, &v.JobID, &v.RelPath, &v.VersionNum, &v.StoredPath,
		&v.ContentHash, &v.HashAlgo, &v.SizeBytes, &v.ModTime, &v.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return v, err
}

// ListFileVersions returns all versions for a given job and path, newest first.
func (db *DB) ListFileVersions(ctx context.Context, jobID int64, relPath string) ([]*FileVersion, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, job_id, rel_path, version_num, stored_path, sha256, hash_algo, size_bytes, mod_time, created_at
		 FROM file_versions WHERE job_id = ? AND rel_path = ? ORDER BY version_num DESC`,
		jobID, relPath,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*FileVersion
	for rows.Next() {
		v := &FileVersion{}
		if err := rows.Scan(&v.ID, &v.JobID, &v.RelPath, &v.VersionNum, &v.StoredPath,
			&v.ContentHash, &v.HashAlgo, &v.SizeBytes, &v.ModTime, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
