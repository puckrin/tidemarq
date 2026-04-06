package versions_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/versions"
	"github.com/tidemarq/tidemarq/migrations"
)

func newTestService(t *testing.T) (*versions.Service, *db.DB, int64) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if err := d.Migrate(migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	j, err := d.CreateJob(context.Background(), db.CreateJobParams{
		Name:            "test",
		SourcePath:      t.TempDir(),
		DestinationPath: t.TempDir(),
		Mode:            "two-way",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	versionsDir := t.TempDir()
	svc := versions.New(d, versionsDir, 30)
	return svc, d, j.ID
}

// TestSnapshot_CreatesVersion verifies that snapshotting a file stores a version record.
func TestSnapshot_CreatesVersion(t *testing.T) {
	svc, _, jobID := newTestService(t)

	dir := t.TempDir()
	destPath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(destPath, []byte("original content"), 0644); err != nil {
		t.Fatal(err)
	}

	v, err := svc.Snapshot(context.Background(), jobID, "file.txt", destPath)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if v == nil {
		t.Fatal("expected a version to be created")
	}
	if v.VersionNum != 1 {
		t.Errorf("expected version_num=1, got %d", v.VersionNum)
	}
	if v.SHA256 == "" {
		t.Error("expected SHA256 to be set")
	}

	// Stored file should exist and have the right content.
	data, err := os.ReadFile(v.StoredPath)
	if err != nil {
		t.Fatalf("reading stored version: %v", err)
	}
	if string(data) != "original content" {
		t.Errorf("stored content mismatch: %s", data)
	}
}

// TestSnapshot_VersionNumbers_Increment verifies sequential versioning.
func TestSnapshot_VersionNumbers_Increment(t *testing.T) {
	svc, _, jobID := newTestService(t)

	dir := t.TempDir()
	destPath := filepath.Join(dir, "file.txt")

	for i := 1; i <= 3; i++ {
		if err := os.WriteFile(destPath, []byte("content v"+string(rune('0'+i))), 0644); err != nil {
			t.Fatal(err)
		}
		v, err := svc.Snapshot(context.Background(), jobID, "file.txt", destPath)
		if err != nil {
			t.Fatalf("Snapshot %d: %v", i, err)
		}
		if v.VersionNum != int64(i) {
			t.Errorf("expected version_num=%d, got %d", i, v.VersionNum)
		}
	}
}

// TestSnapshot_NonExistentFile returns nil without error.
func TestSnapshot_NonExistentFile(t *testing.T) {
	svc, _, jobID := newTestService(t)

	v, err := svc.Snapshot(context.Background(), jobID, "missing.txt", "/nonexistent/path/file.txt")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if v != nil {
		t.Error("expected nil version for missing file")
	}
}

// TestRestoreVersion copies the stored version back to destination.
func TestRestoreVersion(t *testing.T) {
	svc, _, jobID := newTestService(t)

	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "file.txt")
	if err := os.WriteFile(destPath, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	v, err := svc.Snapshot(context.Background(), jobID, "file.txt", destPath)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	// Overwrite the destination.
	if err := os.WriteFile(destPath, []byte("overwritten"), 0644); err != nil {
		t.Fatal(err)
	}

	// Restore the version.
	if err := svc.RestoreVersion(context.Background(), v.ID, destDir); err != nil {
		t.Fatalf("RestoreVersion: %v", err)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading restored file: %v", err)
	}
	if string(data) != "original" {
		t.Errorf("expected restored content 'original', got %q", data)
	}
}

// TestQuarantine_MovesFile verifies that quarantine removes file from dest and stores it.
func TestQuarantine_MovesFile(t *testing.T) {
	svc, _, jobID := newTestService(t)

	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "file.txt")
	if err := os.WriteFile(destPath, []byte("to be quarantined"), 0644); err != nil {
		t.Fatal(err)
	}

	e, err := svc.Quarantine(context.Background(), jobID, "file.txt", destPath, destDir)
	if err != nil {
		t.Fatalf("Quarantine: %v", err)
	}
	if e == nil {
		t.Fatal("expected quarantine entry")
	}

	// File should no longer be at dest.
	if _, err := os.Stat(destPath); err == nil {
		t.Error("expected file to be removed from destination after quarantine")
	}

	// Quarantine path should contain the file.
	if _, err := os.Stat(e.QuarantinePath); err != nil {
		t.Errorf("quarantine file not found at %s: %v", e.QuarantinePath, err)
	}
}

// TestRestoreQuarantine copies the quarantined file back.
func TestRestoreQuarantine(t *testing.T) {
	svc, _, jobID := newTestService(t)

	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "file.txt")
	if err := os.WriteFile(destPath, []byte("quarantine me"), 0644); err != nil {
		t.Fatal(err)
	}

	e, err := svc.Quarantine(context.Background(), jobID, "file.txt", destPath, destDir)
	if err != nil {
		t.Fatalf("Quarantine: %v", err)
	}

	if err := svc.RestoreQuarantine(context.Background(), e.ID, destDir); err != nil {
		t.Fatalf("RestoreQuarantine: %v", err)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading restored file: %v", err)
	}
	if string(data) != "quarantine me" {
		t.Errorf("unexpected restored content: %q", data)
	}
}

// TestListVersions returns versions for a specific path.
func TestListVersions(t *testing.T) {
	svc, _, jobID := newTestService(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	for i := 0; i < 3; i++ {
		_ = os.WriteFile(path, []byte("v"), 0644)
		_, _ = svc.Snapshot(context.Background(), jobID, "file.txt", path)
	}

	list, err := svc.ListVersions(context.Background(), jobID, "file.txt")
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("expected 3 versions, got %d", len(list))
	}
	// Should be newest first.
	if list[0].VersionNum < list[1].VersionNum {
		t.Error("expected versions in descending order")
	}
}
