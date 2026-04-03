// Package engine implements the core file synchronisation logic.
package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tidemarq/tidemarq/internal/conflicts"
	"github.com/tidemarq/tidemarq/internal/manifest"
	"github.com/tidemarq/tidemarq/internal/versions"
)

// errMidFilePause is returned by syncFile when a pause fires during a file transfer.
var errMidFilePause = errors.New("paused mid-file")

// Config holds the parameters for a single sync run.
type Config struct {
	JobID            int64
	Mode             string // one-way-backup | one-way-mirror | two-way
	ConflictStrategy string // ask-user | newest-wins | largest-wins | source-wins | destination-wins
	SourcePath       string
	DestinationPath  string
	BandwidthLimitKB int64            // 0 = unlimited
	Workers          int              // 0 = defaultWorkers
	OnProgress       func(Progress)   // called after each file is processed; may be nil
	PauseCh          <-chan struct{}   // closed or sent on to request a graceful pause
	VersionsSvc      *versions.Service // may be nil; used to snapshot before overwrite
	ConflictsSvc     *conflicts.Service // may be nil; used to record conflicts
}

// Progress is emitted after each file is processed during a run.
type Progress struct {
	FilesDone  int
	FilesTotal int
	BytesDone  int64
	RateKBs    float64 // transfer rate over the last interval
}

// Result summarises the outcome of a sync run.
type Result struct {
	FilesCopied  int
	FilesSkipped int
	BytesCopied  int64
	Conflicts    int  // files that had conflicts (auto-resolved or pending)
	Quarantined  int  // files moved to quarantine
	Paused       bool // true if the run was halted by a pause signal
	Errors       []FileError
}

// FileError records a transfer failure for a specific file.
type FileError struct {
	Path string
	Err  error
}

// Engine executes sync jobs using the provided manifest store.
type Engine struct {
	manifest *manifest.Store
}

// New creates an Engine backed by the given manifest store.
func New(m *manifest.Store) *Engine {
	return &Engine{manifest: m}
}

// Run executes a sync job in the mode specified by cfg.Mode.
func (e *Engine) Run(ctx context.Context, cfg Config) (*Result, error) {
	switch cfg.Mode {
	case "one-way-mirror":
		return e.runMirror(ctx, cfg)
	case "two-way":
		return e.runTwoWay(ctx, cfg)
	default: // one-way-backup
		return e.runBackup(ctx, cfg)
	}
}

// -------------------------------------------------------------------------
// one-way-backup: copy new/changed source files to dest; never delete dest.
// -------------------------------------------------------------------------

func (e *Engine) runBackup(ctx context.Context, cfg Config) (*Result, error) {
	files, err := scanDir(ctx, cfg.SourcePath, cfg.Workers)
	if err != nil {
		return nil, fmt.Errorf("scanning source: %w", err)
	}

	total := len(files)
	result := &Result{}
	var mu sync.Mutex
	done := 0
	var bytesDone int64
	rt := &rateTracker{start: time.Now()}

	for _, fi := range files {
		if paused, ctxErr := checkPause(ctx, cfg.PauseCh); paused {
			result.Paused = true
			return result, nil
		} else if ctxErr != nil {
			return result, ctxErr
		}

		copied, skipped, written, ferr := e.syncFile(ctx, cfg, fi, cfg.SourcePath, cfg.DestinationPath, false)
		if errors.Is(ferr, errMidFilePause) {
			result.Paused = true
			return result, nil
		}

		mu.Lock()
		if ferr != nil {
			result.Errors = append(result.Errors, FileError{Path: fi.RelPath, Err: ferr})
		} else {
			result.FilesCopied += copied
			result.FilesSkipped += skipped
			result.BytesCopied += written
		}
		done++
		bytesDone += written
		mu.Unlock()

		emitProgress(cfg.OnProgress, done, total, bytesDone, rt)
	}
	return result, nil
}

// -------------------------------------------------------------------------
// one-way-mirror: like backup, but quarantines dest files deleted from source.
// -------------------------------------------------------------------------

func (e *Engine) runMirror(ctx context.Context, cfg Config) (*Result, error) {
	srcFiles, err := scanDir(ctx, cfg.SourcePath, cfg.Workers)
	if err != nil {
		return nil, fmt.Errorf("scanning source: %w", err)
	}
	destFiles, err := scanDir(ctx, cfg.DestinationPath, cfg.Workers)
	if err != nil {
		return nil, fmt.Errorf("scanning destination: %w", err)
	}

	srcIndex := indexFiles(srcFiles)
	total := len(srcFiles) + len(destFiles)
	result := &Result{}
	done := 0
	var bytesDone int64
	rt := &rateTracker{start: time.Now()}

	// Copy new/changed source files to destination.
	for _, fi := range srcFiles {
		if paused, ctxErr := checkPause(ctx, cfg.PauseCh); paused {
			result.Paused = true
			return result, nil
		} else if ctxErr != nil {
			return result, ctxErr
		}

		copied, skipped, written, ferr := e.syncFile(ctx, cfg, fi, cfg.SourcePath, cfg.DestinationPath, false)
		if errors.Is(ferr, errMidFilePause) {
			result.Paused = true
			return result, nil
		}
		if ferr != nil {
			result.Errors = append(result.Errors, FileError{Path: fi.RelPath, Err: ferr})
		} else {
			result.FilesCopied += copied
			result.FilesSkipped += skipped
			result.BytesCopied += written
		}
		done++
		bytesDone += written
		emitProgress(cfg.OnProgress, done, total, bytesDone, rt)
	}

	// Quarantine destination files that no longer exist in source.
	for _, destFi := range destFiles {
		if _, inSrc := srcIndex[destFi.RelPath]; inSrc {
			done++
			emitProgress(cfg.OnProgress, done, total, bytesDone, rt)
			continue
		}
		destPath := filepath.Join(cfg.DestinationPath, filepath.FromSlash(destFi.RelPath))
		if cfg.VersionsSvc != nil {
			if _, err := cfg.VersionsSvc.Quarantine(ctx, cfg.JobID, destFi.RelPath, destPath); err != nil {
				result.Errors = append(result.Errors, FileError{Path: destFi.RelPath, Err: fmt.Errorf("quarantine: %w", err)})
			} else {
				result.Quarantined++
				_ = e.manifest.Delete(ctx, cfg.JobID, destFi.RelPath)
			}
		}
		done++
		emitProgress(cfg.OnProgress, done, total, bytesDone, rt)
	}

	return result, nil
}

// -------------------------------------------------------------------------
// two-way: bidirectional sync with conflict detection.
// -------------------------------------------------------------------------

func (e *Engine) runTwoWay(ctx context.Context, cfg Config) (*Result, error) {
	srcFiles, err := scanDir(ctx, cfg.SourcePath, cfg.Workers)
	if err != nil {
		return nil, fmt.Errorf("scanning source: %w", err)
	}
	destFiles, err := scanDir(ctx, cfg.DestinationPath, cfg.Workers)
	if err != nil {
		return nil, fmt.Errorf("scanning destination: %w", err)
	}

	srcIndex := indexFiles(srcFiles)
	destIndex := indexFiles(destFiles)
	strategy := cfg.ConflictStrategy
	if strategy == "" {
		strategy = "ask-user"
	}

	total := len(srcFiles) + len(destFiles)
	result := &Result{}
	done := 0
	var bytesDone int64
	rt := &rateTracker{start: time.Now()}

	// --- Source files ---
	for _, srcFi := range srcFiles {
		if paused, ctxErr := checkPause(ctx, cfg.PauseCh); paused {
			result.Paused = true
			return result, nil
		} else if ctxErr != nil {
			return result, ctxErr
		}

		srcPath := filepath.Join(cfg.SourcePath, filepath.FromSlash(srcFi.RelPath))
		destPath := filepath.Join(cfg.DestinationPath, filepath.FromSlash(srcFi.RelPath))

		srcState, err := conflicts.StatFile(srcPath)
		if err != nil {
			result.Errors = append(result.Errors, FileError{Path: srcFi.RelPath, Err: err})
			done++
			continue
		}

		entry, err := e.manifest.Get(ctx, cfg.JobID, srcFi.RelPath)
		if err != nil && !errors.Is(err, manifest.ErrNotFound) {
			result.Errors = append(result.Errors, FileError{Path: srcFi.RelPath, Err: err})
			done++
			continue
		}

		destState, err := conflicts.StatFile(destPath)
		if err != nil {
			result.Errors = append(result.Errors, FileError{Path: srcFi.RelPath, Err: err})
			done++
			continue
		}

		if entry == nil {
			// File is new on source side.
			if !destState.Exists || destState.SHA256 == srcState.SHA256 {
				// Dest doesn't have it or already matches — copy src→dest.
				written, ferr := e.transferFile(ctx, cfg, srcPath, destPath, srcFi, srcState.SHA256)
				if errors.Is(ferr, errMidFilePause) {
					result.Paused = true
					return result, nil
				}
				if ferr != nil {
					result.Errors = append(result.Errors, FileError{Path: srcFi.RelPath, Err: ferr})
				} else {
					result.FilesCopied++
					result.BytesCopied += written
				}
			} else {
				// Both sides have a different file with no history — conflict.
				written, ferr := e.handleTwoWayConflict(ctx, cfg, srcFi.RelPath, srcPath, destPath, srcState, destState, strategy, result)
				result.BytesCopied += written
				if errors.Is(ferr, errMidFilePause) {
					result.Paused = true
					return result, nil
				}
				if ferr != nil {
					result.Errors = append(result.Errors, FileError{Path: srcFi.RelPath, Err: ferr})
				}
			}
		} else {
			// File has sync history.
			isConflict, srcChanged, destChanged := conflicts.Detect(entry.SHA256, srcState, destState)

			switch {
			case isConflict:
				written, ferr := e.handleTwoWayConflict(ctx, cfg, srcFi.RelPath, srcPath, destPath, srcState, destState, strategy, result)
				result.BytesCopied += written
				if errors.Is(ferr, errMidFilePause) {
					result.Paused = true
					return result, nil
				}
				if ferr != nil {
					result.Errors = append(result.Errors, FileError{Path: srcFi.RelPath, Err: ferr})
				}

			case srcChanged && !destChanged:
				// Source updated — copy src→dest.
				if cfg.VersionsSvc != nil && destState.Exists {
					_, _ = cfg.VersionsSvc.Snapshot(ctx, cfg.JobID, srcFi.RelPath, destPath)
				}
				written, ferr := e.transferFile(ctx, cfg, srcPath, destPath, srcFi, srcState.SHA256)
				if errors.Is(ferr, errMidFilePause) {
					result.Paused = true
					return result, nil
				}
				if ferr != nil {
					result.Errors = append(result.Errors, FileError{Path: srcFi.RelPath, Err: ferr})
				} else {
					result.FilesCopied++
					result.BytesCopied += written
				}

			case !srcChanged && destChanged:
				// Dest updated — copy dest→src.
				if cfg.VersionsSvc != nil {
					_, _ = cfg.VersionsSvc.Snapshot(ctx, cfg.JobID, srcFi.RelPath, srcPath)
				}
				written, ferr := e.transferFile(ctx, cfg, destPath, srcPath, srcFi, destState.SHA256)
				if errors.Is(ferr, errMidFilePause) {
					result.Paused = true
					return result, nil
				}
				if ferr != nil {
					result.Errors = append(result.Errors, FileError{Path: srcFi.RelPath, Err: ferr})
				} else {
					result.FilesCopied++
					result.BytesCopied += written
				}

			default:
				// Neither side changed — skip.
				result.FilesSkipped++
			}
		}

		done++
		bytesDone += 0 // progress byte tracking handled per-file above
		emitProgress(cfg.OnProgress, done, total, bytesDone, rt)
	}

	// --- Dest-only files (new on destination side) ---
	for _, destFi := range destFiles {
		if _, inSrc := srcIndex[destFi.RelPath]; inSrc {
			done++
			emitProgress(cfg.OnProgress, done, total, bytesDone, rt)
			continue
		}

		if paused, ctxErr := checkPause(ctx, cfg.PauseCh); paused {
			result.Paused = true
			return result, nil
		} else if ctxErr != nil {
			return result, ctxErr
		}

		destPath := filepath.Join(cfg.DestinationPath, filepath.FromSlash(destFi.RelPath))
		srcPath := filepath.Join(cfg.SourcePath, filepath.FromSlash(destFi.RelPath))

		entry, err := e.manifest.Get(ctx, cfg.JobID, destFi.RelPath)
		if err != nil && !errors.Is(err, manifest.ErrNotFound) {
			result.Errors = append(result.Errors, FileError{Path: destFi.RelPath, Err: err})
			done++
			continue
		}

		if entry == nil {
			// Genuinely new file on dest — copy dest→src.
			destState, _ := conflicts.StatFile(destPath)
			written, ferr := e.transferFile(ctx, cfg, destPath, srcPath, destFi, destState.SHA256)
			if errors.Is(ferr, errMidFilePause) {
				result.Paused = true
				return result, nil
			}
			if ferr != nil {
				result.Errors = append(result.Errors, FileError{Path: destFi.RelPath, Err: ferr})
			} else {
				result.FilesCopied++
				result.BytesCopied += written
			}
		} else {
			// In manifest but no longer on source — source deleted it.
			// Propagate: quarantine from dest.
			if !inDestIndex(destIndex, destFi.RelPath) {
				done++
				continue
			}
			if cfg.VersionsSvc != nil {
				if _, err := cfg.VersionsSvc.Quarantine(ctx, cfg.JobID, destFi.RelPath, destPath); err != nil {
					result.Errors = append(result.Errors, FileError{Path: destFi.RelPath, Err: fmt.Errorf("quarantine: %w", err)})
				} else {
					result.Quarantined++
					_ = e.manifest.Delete(ctx, cfg.JobID, destFi.RelPath)
				}
			}
		}

		done++
		emitProgress(cfg.OnProgress, done, total, bytesDone, rt)
	}

	return result, nil
}

// handleTwoWayConflict applies the conflict strategy and records a conflict entry.
func (e *Engine) handleTwoWayConflict(
	ctx context.Context, cfg Config,
	relPath, srcPath, destPath string,
	srcState, destState conflicts.FileState,
	strategy string,
	result *Result,
) (int64, error) {
	result.Conflicts++

	// Record the conflict in the DB if a service is available.
	if cfg.ConflictsSvc != nil {
		_, _ = cfg.ConflictsSvc.Record(ctx, cfg.JobID, relPath, strategy, srcState, destState)
	}

	// Snapshot before auto-resolve overwrites anything.
	if cfg.VersionsSvc != nil && destState.Exists {
		_, _ = cfg.VersionsSvc.Snapshot(ctx, cfg.JobID, relPath, destPath)
	}

	// For ask-user: AutoResolve renames dest with .conflict suffix and returns srcPath.
	// For auto strategies: returns the winning path.
	winnerPath, err := conflicts.AutoResolve(strategy, srcPath, destPath, srcState, destState)
	if err != nil {
		return 0, err
	}

	// If winner is the destination, no copy needed.
	if winnerPath == destPath {
		return 0, nil
	}

	// Determine source FileInfo for metadata preservation.
	fi := FileInfo{RelPath: relPath, AbsPath: srcPath, ModTime: srcState.ModTime}
	srcHash := srcState.SHA256
	if winnerPath == destPath {
		fi.AbsPath = destPath
		srcHash = destState.SHA256
	}

	return e.transferFile(ctx, cfg, winnerPath, destPath, fi, srcHash)
}

// transferFile copies from srcAbsPath to dstAbsPath, verifies integrity, updates the manifest.
// fi provides RelPath and metadata for the manifest record.
// syncedHash is the hash we expect srcAbsPath to have (pre-computed to avoid double-hashing).
func (e *Engine) transferFile(ctx context.Context, cfg Config, srcAbsPath, dstAbsPath string, fi FileInfo, syncedHash string) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(dstAbsPath), 0o755); err != nil {
		return 0, fmt.Errorf("mkdir: %w", err)
	}

	written, paused, err := copyFile(srcAbsPath, dstAbsPath, cfg.BandwidthLimitKB, cfg.PauseCh)
	if err != nil {
		return 0, fmt.Errorf("copying file: %w", err)
	}
	if paused {
		os.Remove(dstAbsPath) //nolint:errcheck
		return 0, errMidFilePause
	}

	destHash, err := hashFile(dstAbsPath)
	if err != nil {
		return 0, fmt.Errorf("hashing destination: %w", err)
	}
	if destHash != syncedHash {
		return 0, fmt.Errorf("checksum mismatch after transfer: src %s dest %s", syncedHash, destHash)
	}

	if err := os.Chmod(dstAbsPath, fi.Permissions); err != nil {
		// Non-fatal on Windows; log and continue.
		_ = err
	}
	if err := os.Chtimes(dstAbsPath, fi.ModTime, fi.ModTime); err != nil {
		return 0, fmt.Errorf("setting mtime: %w", err)
	}

	if err := e.manifest.Put(ctx, &manifest.Entry{
		JobID:       cfg.JobID,
		RelPath:     fi.RelPath,
		SHA256:      syncedHash,
		SizeBytes:   written,
		ModTime:     fi.ModTime,
		Permissions: fi.Permissions,
		SyncedAt:    time.Now(),
	}); err != nil {
		return 0, fmt.Errorf("updating manifest: %w", err)
	}

	return written, nil
}

// syncFile is the one-way-backup path: decides whether to copy fi and does so.
// Returns (filesCopied, filesSkipped, bytesWritten, error).
func (e *Engine) syncFile(ctx context.Context, cfg Config, fi FileInfo, srcBase, destBase string, snapshot bool) (int, int, int64, error) {
	srcHash, err := hashFile(fi.AbsPath)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("hashing source: %w", err)
	}

	entry, err := e.manifest.Get(ctx, cfg.JobID, fi.RelPath)
	if err != nil && !errors.Is(err, manifest.ErrNotFound) {
		return 0, 0, 0, fmt.Errorf("reading manifest: %w", err)
	}

	destPath := filepath.Join(destBase, filepath.FromSlash(fi.RelPath))

	if entry != nil && entry.SHA256 == srcHash {
		if _, err := os.Stat(destPath); err == nil {
			return 0, 1, 0, nil
		}
	}

	if snapshot && cfg.VersionsSvc != nil {
		if _, statErr := os.Stat(destPath); statErr == nil {
			_, _ = cfg.VersionsSvc.Snapshot(ctx, cfg.JobID, fi.RelPath, destPath)
		}
	}

	written, err := e.transferFile(ctx, cfg, fi.AbsPath, destPath, fi, srcHash)
	if err != nil {
		return 0, 0, 0, err
	}
	return 1, 0, written, nil
}

// -------------------------------------------------------------------------
// helpers
// -------------------------------------------------------------------------

func checkPause(ctx context.Context, pauseCh <-chan struct{}) (paused bool, ctxErr error) {
	if pauseCh != nil {
		select {
		case <-pauseCh:
			return true, nil
		default:
		}
	}
	return false, ctx.Err()
}

func emitProgress(fn func(Progress), done, total int, bytesDone int64, rt *rateTracker) {
	if fn != nil {
		fn(Progress{
			FilesDone:  done,
			FilesTotal: total,
			BytesDone:  bytesDone,
			RateKBs:    rt.rate(bytesDone),
		})
	}
}

func indexFiles(files []FileInfo) map[string]FileInfo {
	m := make(map[string]FileInfo, len(files))
	for _, f := range files {
		m[f.RelPath] = f
	}
	return m
}

func inDestIndex(idx map[string]FileInfo, relPath string) bool {
	_, ok := idx[relPath]
	return ok
}

// hashFile computes the SHA-256 hash of the file at path and returns it as a hex string.
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

// copyFile copies src to dst, applying an optional bandwidth limit (KB/s) and
// honouring pauseCh mid-transfer. Returns bytes written and whether the copy
// was interrupted by a pause signal.
func copyFile(src, dst string, bwLimitKBs int64, pauseCh <-chan struct{}) (int64, bool, error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return 0, false, err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return 0, false, err
	}
	defer dstFile.Close()

	tr := newThrottledReader(srcFile, bwLimitKBs, pauseCh)
	written, err := io.Copy(dstFile, tr)

	if tr, ok := tr.(*throttledReader); ok && tr.paused {
		return written, true, nil
	}
	return written, false, err
}

// rateTracker computes a smoothed transfer rate.
type rateTracker struct {
	start time.Time
}

func (rt *rateTracker) rate(bytesDone int64) float64 {
	elapsed := time.Since(rt.start).Seconds()
	if elapsed < 0.001 {
		return 0
	}
	return float64(bytesDone) / 1024 / elapsed
}
