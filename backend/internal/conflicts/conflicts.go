// Package conflicts provides conflict detection and resolution for two-way sync jobs.
package conflicts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/tidemarq/tidemarq/internal/db"
)

// ErrNotFound is returned when a conflict record does not exist.
var ErrNotFound = errors.New("conflict not found")

// FileState holds the current on-disk state of one side of a file.
type FileState struct {
	AbsPath string
	SHA256  string
	Size    int64
	ModTime time.Time
	Exists  bool
}

// StatFile reads the current state of a file at absPath.
func StatFile(absPath string) (FileState, error) {
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return FileState{AbsPath: absPath, Exists: false}, nil
	}
	if err != nil {
		return FileState{}, err
	}

	hash, err := hashFile(absPath)
	if err != nil {
		return FileState{}, err
	}

	return FileState{
		AbsPath: absPath,
		SHA256:  hash,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Exists:  true,
	}, nil
}

// Detect checks whether src and dest represent a conflict given the last known
// synced hash. A conflict exists when BOTH sides have changed since the last sync.
// Returns (isConflict, srcChanged, destChanged).
func Detect(syncedHash string, src, dest FileState) (isConflict, srcChanged, destChanged bool) {
	srcChanged = src.Exists && src.SHA256 != syncedHash
	destChanged = dest.Exists && dest.SHA256 != syncedHash
	isConflict = srcChanged && destChanged
	return
}

// Service manages conflict records in the database.
type Service struct {
	db *db.DB
}

// New creates a new conflict Service.
func New(database *db.DB) *Service {
	return &Service{db: database}
}

// Record stores a new conflict in the database and returns its ID.
func (s *Service) Record(ctx context.Context, jobID int64, relPath, strategy string, src, dest FileState) (*db.Conflict, error) {
	return s.db.CreateConflict(ctx, db.CreateConflictParams{
		JobID:       jobID,
		RelPath:     relPath,
		SrcSHA256:   src.SHA256,
		DestSHA256:  dest.SHA256,
		SrcModTime:  src.ModTime,
		DestModTime: dest.ModTime,
		SrcSize:     src.Size,
		DestSize:    dest.Size,
		Strategy:    strategy,
	})
}

// Get retrieves a conflict by ID.
func (s *Service) Get(ctx context.Context, id int64) (*db.Conflict, error) {
	c, err := s.db.GetConflict(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		return nil, ErrNotFound
	}
	return c, err
}

// List returns all conflicts, optionally filtered by job (jobID=0 means all).
func (s *Service) List(ctx context.Context, jobID int64) ([]*db.Conflict, error) {
	return s.db.ListConflicts(ctx, jobID)
}

// Resolve marks a conflict as resolved and applies the chosen action to the filesystem.
// action must be one of: "keep-source", "keep-dest", "keep-both".
func (s *Service) Resolve(ctx context.Context, id int64, action, srcPath, destPath string) error {
	c, err := s.db.GetConflict(ctx, id)
	if errors.Is(err, db.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if c.Status != "pending" {
		return fmt.Errorf("conflict %d is already resolved", id)
	}

	switch action {
	case "keep-source":
		if err := copyFile(srcPath, destPath); err != nil {
			return fmt.Errorf("keep-source copy: %w", err)
		}
	case "keep-dest":
		// Destination is already correct; no filesystem change needed.
	case "keep-both":
		// Rename the destination copy with a conflict suffix, then copy source in.
		ts := time.Now().UTC().Format("20060102T150405")
		conflictPath := destPath + ".conflict." + ts
		if err := os.Rename(destPath, conflictPath); err != nil {
			return fmt.Errorf("keep-both rename: %w", err)
		}
		if err := copyFile(srcPath, destPath); err != nil {
			return fmt.Errorf("keep-both copy: %w", err)
		}
	default:
		return fmt.Errorf("unknown resolution action %q", action)
	}

	return s.db.ResolveConflict(ctx, id, action, "resolved")
}

// AutoResolve applies the job's configured strategy automatically during a sync run.
// Returns the path that should be used as the source of truth after resolution,
// and renames the dest copy when strategy is ask-user.
func AutoResolve(strategy string, srcPath, destPath string, src, dest FileState) (winner string, err error) {
	switch strategy {
	case "source-wins":
		return srcPath, nil

	case "destination-wins":
		return destPath, nil

	case "newest-wins":
		if src.ModTime.After(dest.ModTime) {
			return srcPath, nil
		}
		return destPath, nil

	case "largest-wins":
		if src.Size >= dest.Size {
			return srcPath, nil
		}
		return destPath, nil

	case "ask-user":
		// Preserve both: rename dest with conflict suffix, leave source to be copied normally.
		ts := time.Now().UTC().Format("20060102T150405")
		conflictPath := destPath + ".conflict." + ts
		if err := os.Rename(destPath, conflictPath); err != nil {
			return "", fmt.Errorf("ask-user rename dest: %w", err)
		}
		return srcPath, nil

	default:
		return "", fmt.Errorf("unknown conflict strategy %q", strategy)
	}
}

// hashFile computes the SHA-256 of the file at path.
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

// copyFile copies src to dst, creating intermediate directories as needed.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.CreateTemp(filepath.Dir(dst), ".tmp-")
	if err != nil {
		return err
	}
	tmpPath := out.Name()

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
