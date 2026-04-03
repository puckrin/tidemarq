// Package versions manages file version history and quarantine for sync jobs.
package versions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/tidemarq/tidemarq/internal/db"
)

// ErrNotFound is returned when a version or quarantine record does not exist.
var ErrNotFound = errors.New("not found")

// Service manages version snapshots and quarantine entries.
type Service struct {
	db           *db.DB
	versionsDir  string
	quarantineDir string
	retentionDays int
}

// New creates a new versions Service.
// versionsDir is where snapshot files are stored.
// quarantineDir is where soft-deleted files are held.
// retentionDays is how long quarantine entries are kept before expiry.
func New(database *db.DB, versionsDir, quarantineDir string, retentionDays int) *Service {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	return &Service{
		db:            database,
		versionsDir:   versionsDir,
		quarantineDir: quarantineDir,
		retentionDays: retentionDays,
	}
}

// Snapshot saves the current state of destPath as a new version before it is overwritten.
// relPath is the job-relative path; jobID identifies the owning job.
func (s *Service) Snapshot(ctx context.Context, jobID int64, relPath, destPath string) (*db.FileVersion, error) {
	info, err := os.Stat(destPath)
	if os.IsNotExist(err) {
		return nil, nil // nothing to snapshot
	}
	if err != nil {
		return nil, err
	}

	hash, err := hashFile(destPath)
	if err != nil {
		return nil, err
	}

	// Build storage path: versionsDir/<jobID>/<relPath>/<unix-nano>
	storedPath := filepath.Join(
		s.versionsDir,
		fmt.Sprintf("%d", jobID),
		filepath.FromSlash(relPath),
		fmt.Sprintf("%d", time.Now().UnixNano()),
	)
	if err := os.MkdirAll(filepath.Dir(storedPath), 0o755); err != nil {
		return nil, err
	}
	if err := copyFile(destPath, storedPath); err != nil {
		return nil, err
	}

	return s.db.CreateFileVersion(ctx, &db.FileVersion{
		JobID:     jobID,
		RelPath:   relPath,
		StoredPath: storedPath,
		SHA256:    hash,
		SizeBytes: info.Size(),
		ModTime:   info.ModTime(),
	})
}

// ListVersions returns all stored versions for a file, newest first.
func (s *Service) ListVersions(ctx context.Context, jobID int64, relPath string) ([]*db.FileVersion, error) {
	return s.db.ListFileVersions(ctx, jobID, relPath)
}

// GetVersion retrieves a single version by ID.
func (s *Service) GetVersion(ctx context.Context, id int64) (*db.FileVersion, error) {
	v, err := s.db.GetFileVersion(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		return nil, ErrNotFound
	}
	return v, err
}

// RestoreVersion copies the stored version back to the destination path.
func (s *Service) RestoreVersion(ctx context.Context, id int64, destBasePath string) error {
	v, err := s.GetVersion(ctx, id)
	if err != nil {
		return err
	}

	destPath := filepath.Join(destBasePath, filepath.FromSlash(v.RelPath))
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	return copyFile(v.StoredPath, destPath)
}

// Quarantine moves the file at destPath into the quarantine store and records it in the DB.
func (s *Service) Quarantine(ctx context.Context, jobID int64, relPath, destPath string) (*db.QuarantineEntry, error) {
	info, err := os.Stat(destPath)
	if os.IsNotExist(err) {
		return nil, nil // file already gone
	}
	if err != nil {
		return nil, err
	}

	hash, err := hashFile(destPath)
	if err != nil {
		return nil, err
	}

	quarantinePath := filepath.Join(
		s.quarantineDir,
		fmt.Sprintf("%d", jobID),
		filepath.FromSlash(relPath),
		fmt.Sprintf("%d", time.Now().UnixNano()),
	)
	if err := os.MkdirAll(filepath.Dir(quarantinePath), 0o755); err != nil {
		return nil, err
	}
	// Move (rename) where possible; fall back to copy+delete across volumes.
	if err := os.Rename(destPath, quarantinePath); err != nil {
		if err2 := copyFile(destPath, quarantinePath); err2 != nil {
			return nil, fmt.Errorf("quarantine copy: %w", err2)
		}
		if err2 := os.Remove(destPath); err2 != nil {
			return nil, fmt.Errorf("quarantine remove original: %w", err2)
		}
	}

	now := time.Now().UTC()
	return s.db.CreateQuarantineEntry(ctx, &db.QuarantineEntry{
		JobID:          jobID,
		RelPath:        relPath,
		QuarantinePath: quarantinePath,
		SHA256:         hash,
		SizeBytes:      info.Size(),
		DeletedAt:      now,
		ExpiresAt:      now.AddDate(0, 0, s.retentionDays),
	})
}

// ListQuarantine returns all quarantine entries, optionally filtered by job.
func (s *Service) ListQuarantine(ctx context.Context, jobID int64) ([]*db.QuarantineEntry, error) {
	return s.db.ListQuarantineEntries(ctx, jobID)
}

// GetQuarantineEntry retrieves a single quarantine entry by ID.
func (s *Service) GetQuarantineEntry(ctx context.Context, id int64) (*db.QuarantineEntry, error) {
	e, err := s.db.GetQuarantineEntry(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		return nil, ErrNotFound
	}
	return e, err
}

// RestoreQuarantine copies the quarantined file back to the destination.
func (s *Service) RestoreQuarantine(ctx context.Context, id int64, destBasePath string) error {
	e, err := s.GetQuarantineEntry(ctx, id)
	if err != nil {
		return err
	}
	if time.Now().UTC().After(e.ExpiresAt) {
		return fmt.Errorf("quarantine entry %d has expired", id)
	}

	destPath := filepath.Join(destBasePath, filepath.FromSlash(e.RelPath))
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	if err := copyFile(e.QuarantinePath, destPath); err != nil {
		return err
	}
	return s.db.DeleteQuarantineEntry(ctx, id)
}

// hashFile computes SHA-256 of the file at path.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// copyFile copies src to dst atomically via a temp file.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	srcInfo, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst+".tmp", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode()&fs.ModePerm)
	if err != nil {
		return err
	}
	tmpPath := dst + ".tmp"

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	// On Windows, Rename fails if the destination already exists.
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, dst)
}
