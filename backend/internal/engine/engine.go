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

	"github.com/tidemarq/tidemarq/internal/manifest"
)

// Config holds the parameters for a single sync run.
type Config struct {
	JobID            int64
	SourcePath       string
	DestinationPath  string
	BandwidthLimitKB int64 // 0 = unlimited
	Workers          int   // 0 = defaultWorkers
}

// Result summarises the outcome of a sync run.
type Result struct {
	FilesCopied  int
	FilesSkipped int
	BytesCopied  int64
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

// Run executes a one-way-backup sync: copies new or changed files from source
// to destination. Files deleted from source are never removed from destination.
// The run is idempotent: a second call with no source changes produces zero copies.
func (e *Engine) Run(ctx context.Context, cfg Config) (*Result, error) {
	files, err := scanDir(ctx, cfg.SourcePath, cfg.Workers)
	if err != nil {
		return nil, fmt.Errorf("scanning source: %w", err)
	}

	workers := cfg.Workers
	if workers <= 0 {
		workers = defaultWorkers
	}

	taskCh := make(chan FileInfo, len(files))
	for _, fi := range files {
		taskCh <- fi
	}
	close(taskCh)

	result := &Result{}
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fi := range taskCh {
				if ctx.Err() != nil {
					return
				}
				copied, skipped, written, ferr := e.syncFile(ctx, cfg, fi)
				mu.Lock()
				if ferr != nil {
					result.Errors = append(result.Errors, FileError{Path: fi.RelPath, Err: ferr})
				} else {
					result.FilesCopied += copied
					result.FilesSkipped += skipped
					result.BytesCopied += written
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return result, nil
}

// syncFile decides whether to transfer fi and, if so, performs the copy.
// Returns (filesCopied, filesSkipped, bytesWritten, error).
func (e *Engine) syncFile(ctx context.Context, cfg Config, fi FileInfo) (int, int, int64, error) {
	srcHash, err := hashFile(fi.AbsPath)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("hashing source: %w", err)
	}

	entry, err := e.manifest.Get(ctx, cfg.JobID, fi.RelPath)
	if err != nil && !errors.Is(err, manifest.ErrNotFound) {
		return 0, 0, 0, fmt.Errorf("reading manifest: %w", err)
	}

	// Skip when SHA-256 matches the last recorded sync.
	if entry != nil && entry.SHA256 == srcHash {
		return 0, 1, 0, nil
	}

	destPath := filepath.Join(cfg.DestinationPath, filepath.FromSlash(fi.RelPath))
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return 0, 0, 0, fmt.Errorf("creating destination directory: %w", err)
	}

	written, err := copyFile(fi.AbsPath, destPath, cfg.BandwidthLimitKB)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("copying file: %w", err)
	}

	// Verify integrity of the copy.
	destHash, err := hashFile(destPath)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("hashing destination: %w", err)
	}
	if destHash != srcHash {
		return 0, 0, 0, fmt.Errorf("checksum mismatch after transfer: source %s destination %s", srcHash, destHash)
	}

	// Preserve original metadata.
	if err := os.Chmod(destPath, fi.Permissions); err != nil {
		return 0, 0, 0, fmt.Errorf("setting permissions: %w", err)
	}
	if err := os.Chtimes(destPath, fi.ModTime, fi.ModTime); err != nil {
		return 0, 0, 0, fmt.Errorf("setting mtime: %w", err)
	}

	// Record the successful transfer in the manifest.
	if err := e.manifest.Put(ctx, &manifest.Entry{
		JobID:       cfg.JobID,
		RelPath:     fi.RelPath,
		SHA256:      srcHash,
		SizeBytes:   written,
		ModTime:     fi.ModTime,
		Permissions: fi.Permissions,
		SyncedAt:    time.Now(),
	}); err != nil {
		return 0, 0, 0, fmt.Errorf("updating manifest: %w", err)
	}

	return 1, 0, written, nil
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

// copyFile copies src to dst, applying an optional bandwidth limit (KB/s).
// Returns the number of bytes written.
func copyFile(src, dst string, bwLimitKBs int64) (int64, error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return 0, err
	}
	defer dstFile.Close()

	r := newThrottledReader(srcFile, bwLimitKBs)
	return io.Copy(dstFile, r)
}
