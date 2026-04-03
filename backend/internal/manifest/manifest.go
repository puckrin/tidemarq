// Package manifest provides persistent storage for per-file sync records.
package manifest

import (
	"context"
	"errors"
	"io/fs"
	"time"

	"github.com/tidemarq/tidemarq/internal/db"
)

// ErrNotFound is returned when no manifest entry exists for the given path.
var ErrNotFound = errors.New("manifest: entry not found")

// Entry holds the recorded state of a file at the time of its last successful sync.
type Entry struct {
	JobID       int64
	RelPath     string
	SHA256      string
	SizeBytes   int64
	ModTime     time.Time
	Permissions fs.FileMode
	SyncedAt    time.Time
}

// Store persists manifest entries to the database.
type Store struct {
	db *db.DB
}

// New creates a Store backed by the given database.
func New(d *db.DB) *Store {
	return &Store{db: d}
}

// Get retrieves the manifest entry for relPath within jobID.
// Returns ErrNotFound if no entry exists.
func (s *Store) Get(ctx context.Context, jobID int64, relPath string) (*Entry, error) {
	e, err := s.db.GetManifestEntry(ctx, jobID, relPath)
	if errors.Is(err, db.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &Entry{
		JobID:       e.JobID,
		RelPath:     e.RelPath,
		SHA256:      e.SHA256,
		SizeBytes:   e.SizeBytes,
		ModTime:     e.ModTime,
		Permissions: e.Permissions,
		SyncedAt:    e.SyncedAt,
	}, nil
}

// Put creates or updates the manifest entry for a file.
func (s *Store) Put(ctx context.Context, e *Entry) error {
	return s.db.UpsertManifestEntry(ctx, &db.ManifestEntry{
		JobID:       e.JobID,
		RelPath:     e.RelPath,
		SHA256:      e.SHA256,
		SizeBytes:   e.SizeBytes,
		ModTime:     e.ModTime,
		Permissions: e.Permissions,
		SyncedAt:    e.SyncedAt,
	})
}

// Delete removes the manifest entry for relPath within jobID, if it exists.
func (s *Store) Delete(ctx context.Context, jobID int64, relPath string) error {
	return s.db.DeleteManifestEntry(ctx, jobID, relPath)
}

// List returns all manifest entries for jobID ordered by path.
func (s *Store) List(ctx context.Context, jobID int64) ([]*Entry, error) {
	dbEntries, err := s.db.ListManifestEntries(ctx, jobID)
	if err != nil {
		return nil, err
	}
	entries := make([]*Entry, len(dbEntries))
	for i, e := range dbEntries {
		entries[i] = &Entry{
			JobID:       e.JobID,
			RelPath:     e.RelPath,
			SHA256:      e.SHA256,
			SizeBytes:   e.SizeBytes,
			ModTime:     e.ModTime,
			Permissions: e.Permissions,
			SyncedAt:    e.SyncedAt,
		}
	}
	return entries, nil
}
