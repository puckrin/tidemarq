// Package mountfs defines the filesystem abstraction used by the sync engine,
// implemented by local paths, SFTP, and SMB mounts.
package mountfs

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
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
	return &LocalFS{root: root}
}

func (l *LocalFS) abs(relPath string) string {
	if relPath == "" || relPath == "." {
		return l.root
	}
	return filepath.Join(l.root, filepath.FromSlash(relPath))
}

func (l *LocalFS) Stat(relPath string) (FileInfo, error) {
	info, err := os.Stat(l.abs(relPath))
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
	entries, err := os.ReadDir(l.abs(relPath))
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
	return os.Open(l.abs(relPath))
}

func (l *LocalFS) Create(relPath string) (io.WriteCloser, error) {
	p := l.abs(relPath)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	return os.Create(p)
}

func (l *LocalFS) MkdirAll(relPath string) error {
	return os.MkdirAll(l.abs(relPath), 0o755)
}

func (l *LocalFS) Remove(relPath string) error {
	return os.Remove(l.abs(relPath))
}

func (l *LocalFS) Rename(oldPath, newPath string) error {
	return os.Rename(l.abs(oldPath), l.abs(newPath))
}

func (l *LocalFS) Close() error { return nil }

// Root returns the absolute root path of this LocalFS.
func (l *LocalFS) Root() string { return l.root }

// Chtimes updates the access and modification times of the file at relPath.
func (l *LocalFS) Chtimes(relPath string, mtime time.Time) error {
	return os.Chtimes(l.abs(relPath), mtime, mtime)
}

// Chmod sets the permission bits of the file at relPath.
func (l *LocalFS) Chmod(relPath string, mode fs.FileMode) error {
	return os.Chmod(l.abs(relPath), mode)
}
