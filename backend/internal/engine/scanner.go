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
			for w := range workCh {
				mu.Lock()
				results = append(results, FileInfo{
					RelPath:     w.relPath,
					AbsPath:     "", // not applicable for remote FS
					Size:        w.info.Size,
					ModTime:     w.info.ModTime,
					Permissions: w.info.Mode.Perm(),
				})
				mu.Unlock()
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
				workCh <- work{relPath: relPath, info: e}
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
// Subdirectory enumeration is serialised (WalkDir), but stat calls are
// fanned out across a bounded goroutine pool of size workers.
func scanDir(ctx context.Context, root string, workers int) ([]FileInfo, error) {
	if workers <= 0 {
		workers = defaultWorkers
	}

	type work struct {
		path string
		info fs.DirEntry
	}

	workCh := make(chan work, 256)

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		results  []FileInfo
		firstErr error
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for w := range workCh {
				info, err := w.info.Info()
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					continue
				}
				relPath, err := filepath.Rel(root, w.path)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					continue
				}
				mu.Lock()
				results = append(results, FileInfo{
					RelPath:     filepath.ToSlash(relPath),
					AbsPath:     w.path,
					Size:        info.Size(),
					ModTime:     info.ModTime(),
					Permissions: info.Mode().Perm(),
				})
				mu.Unlock()
			}
		}()
	}

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !d.Type().IsRegular() {
			return nil
		}
		workCh <- work{path: path, info: d}
		return nil
	})

	close(workCh)
	wg.Wait()

	if walkErr != nil {
		return nil, walkErr
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}
