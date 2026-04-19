package engine

import (
	"context"
	"io/fs"
	"path"
	"path/filepath"
	"sync"
	"time"

	"github.com/tidemarq/tidemarq/internal/mountfs"
)

// FileInfo holds metadata about a regular file discovered during a directory scan.
type FileInfo struct {
	RelPath     string
	AbsPath     string
	Size        int64
	ModTime     time.Time
	Permissions fs.FileMode
}

const defaultWorkers = 8

// scanFS walks a MountFS recursively and returns FileInfo for every regular file.
// Subdirectory enumeration is parallelised across a bounded goroutine pool.
func scanFS(ctx context.Context, mfs mountfs.MountFS, workers int) ([]FileInfo, error) {
	if workers <= 0 {
		workers = defaultWorkers
	}

	type work struct {
		relPath string
		info    mountfs.FileInfo
	}

	workCh := make(chan work, 256)

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		results  []FileInfo
		firstErr error
	)

	// File-stat workers.
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case w, ok := <-workCh:
					if !ok {
						return
					}
					mu.Lock()
					results = append(results, FileInfo{
						RelPath:     w.relPath,
						AbsPath:     "", // not applicable for remote FS
						Size:        w.info.Size,
						ModTime:     w.info.ModTime,
						Permissions: w.info.Mode.Perm(),
					})
					mu.Unlock()
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	// Recursive directory walker (single goroutine).
	var walkDir func(dir string) error
	walkDir = func(dir string) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		entries, err := mfs.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			// Skip the destination-local quarantine folder so its contents are
			// never treated as source or destination files during a sync run.
			if e.IsDir && e.Name == ".tidemarq-quarantine" {
				continue
			}
			var relPath string
			if dir == "" || dir == "." {
				relPath = e.Name
			} else {
				relPath = path.Join(dir, e.Name)
			}
			if e.IsDir {
				if err := walkDir(relPath); err != nil {
					return err
				}
			} else {
				select {
				case workCh <- work{relPath: relPath, info: e}:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
		return nil
	}

	walkErr := walkDir("")
	close(workCh)
	wg.Wait()

	if walkErr != nil {
		return nil, walkErr
	}
	mu.Lock()
	defer mu.Unlock()
	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}

// scanDir walks root and returns FileInfo for every regular file found.
// DirEntry.Info() is nearly free on local filesystems (metadata is already
// fetched by the OS during directory enumeration), so the bottleneck is the
// single-threaded WalkDir traversal itself — a goroutine pool adds overhead
// without benefit here. The workers parameter is accepted but unused; it
// remains for API compatibility with callers that pass cfg.Workers.
func scanDir(ctx context.Context, root string, _ int) ([]FileInfo, error) {
	var results []FileInfo

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// Skip the destination-local quarantine folder so its contents are never
		// treated as source or destination files during a sync run.
		if d.IsDir() && d.Name() == ".tidemarq-quarantine" {
			return filepath.SkipDir
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		results = append(results, FileInfo{
			RelPath:     filepath.ToSlash(relPath),
			AbsPath:     path,
			Size:        info.Size(),
			ModTime:     info.ModTime(),
			Permissions: info.Mode().Perm(),
		})
		return nil
	})

	if walkErr != nil {
		return nil, walkErr
	}
	return results, nil
}
