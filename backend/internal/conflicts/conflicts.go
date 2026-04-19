// Package conflicts provides conflict detection and resolution for two-way sync jobs.
package conflicts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/hasher"
)

// ErrNotFound is returned when a conflict record does not exist.
var ErrNotFound = errors.New("conflict not found")

// ErrAlreadyResolved is returned when Resolve is called on a non-pending conflict.
var ErrAlreadyResolved = errors.New("conflict is already resolved")

// FileState holds the current on-disk state of one side of a file.
type FileState struct {
	AbsPath     string
	ContentHash string
	HashAlgo    string
	Size        int64
	ModTime     time.Time
	Exists      bool
}

// StatFile reads the current state of a file at absPath using the given hash algorithm.
func StatFile(absPath, algo string) (FileState, error) {
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return FileState{AbsPath: absPath, Exists: false}, nil
	}
	if err != nil {
		return FileState{}, err
	}

	hash, err := hasher.HashFile(algo, absPath)
	if err != nil {
		return FileState{}, err
	}

	return FileState{
		AbsPath:     absPath,
		ContentHash: hash,
		HashAlgo:    algo,
		Size:        info.Size(),
		ModTime:     info.ModTime(),
		Exists:      true,
	}, nil
}

// Detect checks whether src and dest represent a conflict given the last known
// synced hash. A conflict requires both sides to have changed AND to have diverged
// (different content from each other). If both sides changed but converged on the
// same content (e.g. after conflict resolution) isConflict is false.
// Returns (isConflict, srcChanged, destChanged).
func Detect(syncedHash string, src, dest FileState) (isConflict, srcChanged, destChanged bool) {
	srcChanged = src.Exists && src.ContentHash != syncedHash
	destChanged = dest.Exists && dest.ContentHash != syncedHash
	isConflict = srcChanged && destChanged && src.ContentHash != dest.ContentHash
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
// conflictPath is the path of the .conflict.<ts> file created by AutoResolve for ask-user
// conflicts; pass empty string if no such file was created.
func (s *Service) Record(ctx context.Context, jobID int64, relPath, strategy, conflictPath string, src, dest FileState) (*db.Conflict, error) {
	return s.db.CreateConflict(ctx, db.CreateConflictParams{
		JobID:           jobID,
		RelPath:         relPath,
		SrcContentHash:  src.ContentHash,
		DestContentHash: dest.ContentHash,
		SrcHashAlgo:     src.HashAlgo,
		DestHashAlgo:    dest.HashAlgo,
		SrcModTime:      src.ModTime,
		DestModTime:     dest.ModTime,
		SrcSize:         src.Size,
		DestSize:        dest.Size,
		Strategy:        strategy,
		ConflictPath:    conflictPath,
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

// ClearResolved deletes all resolved conflict records, optionally scoped to a
// single job.  Pass jobID=0 to clear across all jobs.
func (s *Service) ClearResolved(ctx context.Context, jobID int64) error {
	return s.db.DeleteResolvedConflicts(ctx, jobID)
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
		return ErrAlreadyResolved
	}

	switch action {
	case "keep-source":
		// Copy source → dest only if source exists on disk.
		if _, statErr := os.Stat(srcPath); statErr == nil {
			if err := copyFile(srcPath, destPath); err != nil {
				return fmt.Errorf("keep-source copy: %w", err)
			}
		}
		// Clean up any .conflict.<ts> file that was preserved from an older
		// conflict detection run.
		if c.ConflictPath != nil && *c.ConflictPath != "" {
			_ = os.Remove(*c.ConflictPath)
		}

	case "keep-dest":
		if c.ConflictPath != nil && *c.ConflictPath != "" {
			// Legacy path: AutoResolve had already moved dest to conflict_path
			// and placed the source copy at dest.  Undo that.
			if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("keep-dest remove source copy: %w", err)
			}
			if err := os.Rename(*c.ConflictPath, destPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("keep-dest restore: %w", err)
			}
		} else {
			// Normal path: dest still holds the original destination content.
			// Copy dest → src so both sides become consistent, if dest exists.
			if _, statErr := os.Stat(destPath); statErr == nil {
				if err := copyFile(destPath, srcPath); err != nil {
					return fmt.Errorf("keep-dest copy to source: %w", err)
				}
			}
		}

	case "keep-both":
		if c.ConflictPath != nil && *c.ConflictPath != "" {
			// Legacy path: both files are already in place from AutoResolve.
		} else {
			// Normal path: rename dest aside with a conflict suffix so the
			// destination version is preserved, then copy source into dest.
			// Skip if dest doesn't exist — nothing to preserve.
			if _, statErr := os.Stat(destPath); statErr == nil {
				ts := time.Now().UTC().Format("20060102T150405")
				cp := destPath + ".conflict." + ts
				if err := os.Rename(destPath, cp); err != nil {
					return fmt.Errorf("keep-both rename: %w", err)
				}
				if _, statErr := os.Stat(srcPath); statErr == nil {
					if err := copyFile(srcPath, destPath); err != nil {
						return fmt.Errorf("keep-both copy: %w", err)
					}
				}
			}
		}

	default:
		return fmt.Errorf("unknown resolution action %q", action)
	}

	return s.db.ResolveConflict(ctx, id, action, "resolved")
}

// AutoResolve applies the job's configured strategy automatically during a sync run.
// Returns the path that should be used as the source of truth after resolution.
// For ask-user, it also renames the dest copy to a .conflict.<ts> file and returns
// that path as conflictPath so it can be stored and later cleaned up on resolution.
// For all other strategies conflictPath is empty.
func AutoResolve(strategy string, srcPath, destPath string, src, dest FileState) (winner, conflictPath string, err error) {
	switch strategy {
	case "source-wins":
		return srcPath, "", nil

	case "destination-wins":
		return destPath, "", nil

	case "newest-wins":
		if src.ModTime.After(dest.ModTime) {
			return srcPath, "", nil
		}
		return destPath, "", nil

	case "largest-wins":
		if src.Size >= dest.Size {
			return srcPath, "", nil
		}
		return destPath, "", nil

	case "ask-user":
		// Leave the filesystem untouched — the conflict is queued for manual
		// resolution.  Returning destPath as the winner causes the engine to skip
		// the file copy so neither side is modified until the user decides.
		return destPath, "", nil

	default:
		return "", "", fmt.Errorf("unknown conflict strategy %q", strategy)
	}
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
		_ = os.Remove(tmpPath)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	// On Windows, Rename fails if the destination already exists.
	if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, dst)
}
