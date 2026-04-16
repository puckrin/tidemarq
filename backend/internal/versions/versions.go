// Package versions manages file version history and quarantine for sync jobs.
package versions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/hasher"
)

// ErrNotFound is returned when a version or quarantine record does not exist.
var ErrNotFound = errors.New("not found")

// quarantineDirName is the folder created inside the destination root to hold
// soft-deleted files. Using a dot-prefix keeps it hidden on Unix systems.
const quarantineDirName = ".tidemarq-quarantine"

// Service manages version snapshots and quarantine entries.
type Service struct {
	db            *db.DB
	versionsDir   string
	retentionDays int
}

// New creates a new versions Service.
// versionsDir is where snapshot files are stored.
// retentionDays is how long quarantine entries are kept before expiry.
func New(database *db.DB, versionsDir string, retentionDays int) *Service {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	return &Service{
		db:            database,
		versionsDir:   versionsDir,
		retentionDays: retentionDays,
	}
}

// Snapshot saves the current state of destPath as a new version before it is overwritten.
// relPath is the job-relative path; jobID identifies the owning job.
// algo is the hash algorithm to use (e.g. "sha256", "blake3").
func (s *Service) Snapshot(ctx context.Context, jobID int64, relPath, destPath, algo string) (*db.FileVersion, error) {
	info, err := os.Stat(destPath)
	if os.IsNotExist(err) {
		return nil, nil // nothing to snapshot
	}
	if err != nil {
		return nil, err
	}

	hash, err := hasher.HashFile(algo, destPath)
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
		JobID:       jobID,
		RelPath:     relPath,
		StoredPath:  storedPath,
		ContentHash: hash,
		HashAlgo:    algo,
		SizeBytes:   info.Size(),
		ModTime:     info.ModTime(),
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

// Quarantine moves the file at destPath into the job's destination-local quarantine
// folder (<destRoot>/.tidemarq-quarantine/<relPath>/<unix_nano>) and records it in
// the DB. Keeping the quarantine inside the destination means the rename is always
// on the same filesystem, avoiding a full copy across volumes.
// algo is the hash algorithm to use (e.g. "sha256", "blake3").
func (s *Service) Quarantine(ctx context.Context, jobID int64, relPath, destPath, destRoot, algo string) (*db.QuarantineEntry, error) {
	info, err := os.Stat(destPath)
	if os.IsNotExist(err) {
		return nil, nil // file already gone
	}
	if err != nil {
		return nil, err
	}

	hash, err := hasher.HashFile(algo, destPath)
	if err != nil {
		return nil, err
	}

	quarantinePath := filepath.Join(
		destRoot,
		quarantineDirName,
		filepath.FromSlash(relPath),
		fmt.Sprintf("%d", time.Now().UnixNano()),
	)
	if err := os.MkdirAll(filepath.Dir(quarantinePath), 0o755); err != nil {
		return nil, err
	}
	// Rename is an atomic, zero-copy operation on the same filesystem.
	// Fall back to copy+delete if we somehow cross a volume boundary.
	if err := os.Rename(destPath, quarantinePath); err != nil {
		if err2 := copyFile(destPath, quarantinePath); err2 != nil {
			return nil, fmt.Errorf("quarantine copy: %w", err2)
		}
		if err2 := os.Remove(destPath); err2 != nil {
			return nil, fmt.Errorf("quarantine remove original: %w", err2)
		}
	}

	// Remove any empty ancestor directories left behind in the destination
	// (bottom-up, stopping at destRoot).
	pruneEmptyDirs(filepath.Dir(destPath), destRoot)

	now := time.Now().UTC()
	return s.db.CreateQuarantineEntry(ctx, &db.QuarantineEntry{
		JobID:          jobID,
		RelPath:        relPath,
		QuarantinePath: quarantinePath,
		ContentHash:    hash,
		HashAlgo:       algo,
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

// RestoreQuarantine moves the quarantined file back to its original location in the
// destination. Since the quarantine lives inside the destination directory, this is
// again a same-filesystem rename — no data is copied.
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
	// Prefer atomic rename; fall back to copy if crossing a volume.
	if err := os.Rename(e.QuarantinePath, destPath); err != nil {
		if err2 := copyFile(e.QuarantinePath, destPath); err2 != nil {
			return fmt.Errorf("restore copy: %w", err2)
		}
		if err2 := os.Remove(e.QuarantinePath); err2 != nil {
			return fmt.Errorf("restore remove quarantine: %w", err2)
		}
	}

	// Clean up empty subdirectories in the quarantine tree, including the
	// .tidemarq-quarantine root itself if it is now empty.
	pruneEmptyDirs(filepath.Dir(e.QuarantinePath), destBasePath)

	return s.db.MarkQuarantineRemoved(ctx, id, "restored")
}

// DeleteQuarantine permanently removes the quarantined file from disk and marks the
// DB record as deleted (keeping it for the "recently removed" history list).
// destBasePath is the job's destination root, used to bound the empty-dir cleanup.
func (s *Service) DeleteQuarantine(ctx context.Context, id int64, destBasePath string) error {
	e, err := s.GetQuarantineEntry(ctx, id)
	if err != nil {
		return err
	}
	// Remove the file; ignore "not found" — it may have already been cleaned up.
	if err := os.Remove(e.QuarantinePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting quarantined file: %w", err)
	}

	// Clean up empty subdirectories in the quarantine tree, including the
	// .tidemarq-quarantine root itself if it is now empty.
	pruneEmptyDirs(filepath.Dir(e.QuarantinePath), destBasePath)

	return s.db.MarkQuarantineRemoved(ctx, id, "deleted")
}

// ListRemovedQuarantine returns quarantine entries that have been restored or deleted,
// optionally filtered by job. Pass jobID=0 to list across all jobs.
func (s *Service) ListRemovedQuarantine(ctx context.Context, jobID int64) ([]*db.QuarantineEntry, error) {
	return s.db.ListRemovedQuarantineEntries(ctx, jobID)
}

// ClearRemovedQuarantine permanently removes all non-active quarantine DB records,
// optionally scoped to a single job. Pass jobID=0 to clear all.
func (s *Service) ClearRemovedQuarantine(ctx context.Context, jobID int64) error {
	return s.db.ClearRemovedQuarantineEntries(ctx, jobID)
}

// pruneEmptyDirs removes dir and each empty ancestor up to (but not including)
// root, walking bottom-up. It stops as soon as a non-empty directory is found
// or the root boundary is reached. Errors (permissions, non-empty dirs) are
// silently ignored — cleanup is best-effort.
func pruneEmptyDirs(dir, root string) {
	root = filepath.Clean(root)
	for {
		dir = filepath.Clean(dir)
		// Stop at or above the root boundary.
		rel, err := filepath.Rel(root, dir)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
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
