package engine

import (
	"context"
	"io/fs"
	"path/filepath"
	"sync"
	"time"
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
