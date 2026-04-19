// Package mountfs defines the filesystem abstraction used by the sync engine,
// implemented by local paths, SFTP, and SMB mounts.
package mountfs

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileInfo is the minimal file metadata the engine needs.
type FileInfo struct {
	Name    string
	Size    int64
	ModTime time.Time
	IsDir   bool
	Mode    fs.FileMode
}

// MountFS abstracts filesystem operations over a rooted path.
// All path arguments are relative to the mount root.
type MountFS interface {
	// Stat returns metadata for the file at relPath.
	Stat(relPath string) (FileInfo, error)

	// ReadDir returns the immediate children of relPath.
	ReadDir(relPath string) ([]FileInfo, error)

	// Open returns a ReadCloser for the file at relPath.
	Open(relPath string) (io.ReadCloser, error)

	// Create creates or truncates the file at relPath and returns a WriteCloser.
	Create(relPath string) (io.WriteCloser, error)

	// MkdirAll ensures the directory at relPath exists (creating parents as needed).
	MkdirAll(relPath string) error

	// Remove deletes the file or empty directory at relPath.
	Remove(relPath string) error

	// Rename moves oldPath to newPath.
	Rename(oldPath, newPath string) error

	// Close releases any resources associated with this mount.
	Close() error
}

// LocalFS is a MountFS backed by the local filesystem under root.
type LocalFS struct {
	root string
}

// NewLocalFS creates a LocalFS rooted at root.
func NewLocalFS(root string) *LocalFS {
	return &LocalFS{root: filepath.Clean(root)}
}

// abs resolves relPath against the LocalFS root and verifies the result is
// still within root. It returns an error for any path that would escape root
// (e.g. "../../etc").
func (l *LocalFS) abs(relPath string) (string, error) {
	if relPath == "" || relPath == "." {
		return l.root, nil
	}
	abs := filepath.Join(l.root, filepath.FromSlash(relPath))

	// filepath.Rel returns a path relative to root; if that path starts with
	// ".." the joined result is outside the root tree.
	rel, err := filepath.Rel(l.root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path %q escapes mount root", relPath)
	}
	return abs, nil
}

func (l *LocalFS) Stat(relPath string) (FileInfo, error) {
	p, err := l.abs(relPath)
	if err != nil {
		return FileInfo{}, err
	}
	info, err := os.Stat(p)
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{
		Name:    info.Name(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
		Mode:    info.Mode(),
	}, nil
}

func (l *LocalFS) ReadDir(relPath string) ([]FileInfo, error) {
	p, err := l.abs(relPath)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	infos := make([]FileInfo, 0, len(entries))
	for _, e := range entries {
		fi, err := e.Info()
		if err != nil {
			continue
		}
		infos = append(infos, FileInfo{
			Name:    fi.Name(),
			Size:    fi.Size(),
			ModTime: fi.ModTime(),
			IsDir:   fi.IsDir(),
			Mode:    fi.Mode(),
		})
	}
	return infos, nil
}

func (l *LocalFS) Open(relPath string) (io.ReadCloser, error) {
	p, err := l.abs(relPath)
	if err != nil {
		return nil, err
	}
	return os.Open(p)
}

func (l *LocalFS) Create(relPath string) (io.WriteCloser, error) {
	p, err := l.abs(relPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	return os.Create(p)
}

func (l *LocalFS) MkdirAll(relPath string) error {
	p, err := l.abs(relPath)
	if err != nil {
		return err
	}
	return os.MkdirAll(p, 0o755)
}

func (l *LocalFS) Remove(relPath string) error {
	p, err := l.abs(relPath)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

func (l *LocalFS) Rename(oldPath, newPath string) error {
	old, err := l.abs(oldPath)
	if err != nil {
		return err
	}
	nw, err := l.abs(newPath)
	if err != nil {
		return err
	}
	return os.Rename(old, nw)
}

func (l *LocalFS) Close() error { return nil }

// Root returns the absolute root path of this LocalFS.
func (l *LocalFS) Root() string { return l.root }

// Chtimes updates the access and modification times of the file at relPath.
func (l *LocalFS) Chtimes(relPath string, mtime time.Time) error {
	p, err := l.abs(relPath)
	if err != nil {
		return err
	}
	return os.Chtimes(p, mtime, mtime)
}

// Chmod sets the permission bits of the file at relPath.
func (l *LocalFS) Chmod(relPath string, mode fs.FileMode) error {
	p, err := l.abs(relPath)
	if err != nil {
		return err
	}
	return os.Chmod(p, mode)
}
