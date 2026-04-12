package manifest_test

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/manifest"
	"github.com/tidemarq/tidemarq/migrations"
)

func newTestStore(t *testing.T) (*manifest.Store, int64) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	if err := d.Migrate(migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Create a job to satisfy the foreign key constraint.
	job, err := d.CreateJob(context.Background(), db.CreateJobParams{
		Name:            "test-job",
		SourcePath:      "/src",
		DestinationPath: "/dst",
		Mode:            "one-way-backup",
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	return manifest.New(d), job.ID
}

func TestStore_GetNotFound(t *testing.T) {
	store, jobID := newTestStore(t)
	_, err := store.Get(context.Background(), jobID, "nonexistent.txt")
	if !errors.Is(err, manifest.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestStore_PutAndGet(t *testing.T) {
	store, jobID := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	entry := &manifest.Entry{
		JobID:       jobID,
		RelPath:     "subdir/file.txt",
		ContentHash: "abc123",
		HashAlgo:    "sha256",
		SizeBytes:   1024,
		ModTime:     now,
		Permissions: fs.FileMode(0644),
		SyncedAt:    now,
	}

	if err := store.Put(ctx, entry); err != nil {
		t.Fatalf("put: %v", err)
	}

	got, err := store.Get(ctx, jobID, "subdir/file.txt")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if got.ContentHash != entry.ContentHash {
		t.Errorf("ContentHash: got %q, want %q", got.ContentHash, entry.ContentHash)
	}
	if got.HashAlgo != entry.HashAlgo {
		t.Errorf("HashAlgo: got %q, want %q", got.HashAlgo, entry.HashAlgo)
	}
	if got.SizeBytes != entry.SizeBytes {
		t.Errorf("SizeBytes: got %d, want %d", got.SizeBytes, entry.SizeBytes)
	}
	if got.Permissions != entry.Permissions {
		t.Errorf("Permissions: got %v, want %v", got.Permissions, entry.Permissions)
	}
}

func TestStore_Put_Upsert(t *testing.T) {
	store, jobID := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	e := &manifest.Entry{
		JobID:       jobID,
		RelPath:     "file.txt",
		ContentHash: "first-hash",
		HashAlgo:    "sha256",
		SyncedAt:    now,
	}
	if err := store.Put(ctx, e); err != nil {
		t.Fatalf("first put: %v", err)
	}

	e.ContentHash = "second-hash"
	if err := store.Put(ctx, e); err != nil {
		t.Fatalf("second put: %v", err)
	}

	got, err := store.Get(ctx, jobID, "file.txt")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ContentHash != "second-hash" {
		t.Errorf("ContentHash: got %q, want %q", got.ContentHash, "second-hash")
	}
}

func TestStore_List(t *testing.T) {
	store, jobID := newTestStore(t)
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Second)
	paths := []string{"a.txt", "b.txt", "c.txt"}
	for _, p := range paths {
		if err := store.Put(ctx, &manifest.Entry{JobID: jobID, RelPath: p, ContentHash: p, HashAlgo: "sha256", SyncedAt: now}); err != nil {
			t.Fatalf("put %q: %v", p, err)
		}
	}

	entries, err := store.List(ctx, jobID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(entries) != len(paths) {
		t.Fatalf("len: got %d, want %d", len(entries), len(paths))
	}
	for i, e := range entries {
		if e.RelPath != paths[i] {
			t.Errorf("entry[%d].RelPath: got %q, want %q", i, e.RelPath, paths[i])
		}
	}
}
