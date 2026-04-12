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

	v, err := svc.Snapshot(context.Background(), jobID, "file.txt", destPath, "sha256")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if v == nil {
		t.Fatal("expected a version to be created")
	}
	if v.VersionNum != 1 {
		t.Errorf("expected version_num=1, got %d", v.VersionNum)
	}
	if v.ContentHash == "" {
		t.Error("expected ContentHash to be set")
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
		v, err := svc.Snapshot(context.Background(), jobID, "file.txt", destPath, "sha256")
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

	v, err := svc.Snapshot(context.Background(), jobID, "missing.txt", "/nonexistent/path/file.txt", "sha256")
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

	v, err := svc.Snapshot(context.Background(), jobID, "file.txt", destPath, "sha256")
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

	e, err := svc.Quarantine(context.Background(), jobID, "file.txt", destPath, destDir, "sha256")
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

	e, err := svc.Quarantine(context.Background(), jobID, "file.txt", destPath, destDir, "sha256")
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

// TestQuarantine_PrunesEmptyDestDirs verifies that empty ancestor directories are
// removed from the destination after quarantine.
func TestQuarantine_PrunesEmptyDestDirs(t *testing.T) {
	svc, _, jobID := newTestService(t)

	destDir := t.TempDir()
	// Create a nested file: <destDir>/a/b/c/file.txt
	nestedDir := filepath.Join(destDir, "a", "b", "c")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatal(err)
	}
	destPath := filepath.Join(nestedDir, "file.txt")
	if err := os.WriteFile(destPath, []byte("nested"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Quarantine(context.Background(), jobID, "a/b/c/file.txt", destPath, destDir, "sha256"); err != nil {
		t.Fatalf("Quarantine: %v", err)
	}

	// All three ancestor dirs should have been pruned.
	for _, dir := range []string{
		filepath.Join(destDir, "a", "b", "c"),
		filepath.Join(destDir, "a", "b"),
		filepath.Join(destDir, "a"),
	} {
		if _, err := os.Stat(dir); err == nil {
			t.Errorf("expected empty dir to be removed: %s", dir)
		}
	}
	// The destination root itself must NOT be removed.
	if _, err := os.Stat(destDir); err != nil {
		t.Errorf("destination root should not be removed: %v", err)
	}
}

// TestQuarantine_PrunesPartialDirs verifies that a directory with a remaining
// sibling file is NOT removed after one of its files is quarantined.
func TestQuarantine_PrunesPartialDirs(t *testing.T) {
	svc, _, jobID := newTestService(t)

	destDir := t.TempDir()
	subDir := filepath.Join(destDir, "docs")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	fileA := filepath.Join(subDir, "a.txt")
	fileB := filepath.Join(subDir, "b.txt")
	if err := os.WriteFile(fileA, []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

	// Quarantine only one file.
	if _, err := svc.Quarantine(context.Background(), jobID, "docs/a.txt", fileA, destDir, "sha256"); err != nil {
		t.Fatalf("Quarantine a.txt: %v", err)
	}

	// docs/ should still exist (b.txt is still there).
	if _, err := os.Stat(subDir); err != nil {
		t.Errorf("docs/ should still exist with b.txt remaining: %v", err)
	}

	// Quarantine the second file.
	if _, err := svc.Quarantine(context.Background(), jobID, "docs/b.txt", fileB, destDir, "sha256"); err != nil {
		t.Fatalf("Quarantine b.txt: %v", err)
	}

	// Now docs/ should be pruned.
	if _, err := os.Stat(subDir); err == nil {
		t.Error("expected docs/ to be pruned after both files quarantined")
	}
}

// TestRestoreQuarantine_PrunesQuarantineDirs verifies the quarantine tree is
// cleaned up after the last entry for a path is restored.
func TestRestoreQuarantine_PrunesQuarantineDirs(t *testing.T) {
	svc, _, jobID := newTestService(t)

	destDir := t.TempDir()
	subDir := filepath.Join(destDir, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	destPath := filepath.Join(subDir, "file.txt")
	if err := os.WriteFile(destPath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	e, err := svc.Quarantine(context.Background(), jobID, "sub/file.txt", destPath, destDir, "sha256")
	if err != nil {
		t.Fatalf("Quarantine: %v", err)
	}
	quarantineSubDir := filepath.Dir(e.QuarantinePath)

	if err := svc.RestoreQuarantine(context.Background(), e.ID, destDir); err != nil {
		t.Fatalf("RestoreQuarantine: %v", err)
	}

	// The quarantine sub-directory should be pruned.
	if _, err := os.Stat(quarantineSubDir); err == nil {
		t.Error("expected quarantine sub-dir to be pruned after restore")
	}
	// The .tidemarq-quarantine root should also be gone (was the only entry).
	quarantineRoot := filepath.Join(destDir, ".tidemarq-quarantine")
	if _, err := os.Stat(quarantineRoot); err == nil {
		t.Error("expected .tidemarq-quarantine to be removed when empty")
	}
}

// TestDeleteQuarantine_PrunesQuarantineDirs verifies the quarantine tree is
// cleaned up after the entry is permanently deleted.
func TestDeleteQuarantine_PrunesQuarantineDirs(t *testing.T) {
	svc, _, jobID := newTestService(t)

	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "file.txt")
	if err := os.WriteFile(destPath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	e, err := svc.Quarantine(context.Background(), jobID, "file.txt", destPath, destDir, "sha256")
	if err != nil {
		t.Fatalf("Quarantine: %v", err)
	}
	quarantineSubDir := filepath.Dir(e.QuarantinePath)

	if err := svc.DeleteQuarantine(context.Background(), e.ID, destDir); err != nil {
		t.Fatalf("DeleteQuarantine: %v", err)
	}

	// The quarantine sub-directory should be pruned.
	if _, err := os.Stat(quarantineSubDir); err == nil {
		t.Error("expected quarantine sub-dir to be pruned after delete")
	}
	// The .tidemarq-quarantine root should also be gone.
	quarantineRoot := filepath.Join(destDir, ".tidemarq-quarantine")
	if _, err := os.Stat(quarantineRoot); err == nil {
		t.Error("expected .tidemarq-quarantine to be removed when empty")
	}
}

// TestRestoreQuarantine_QuarantineDirRetained_WhenSiblingExists verifies that
// .tidemarq-quarantine is NOT removed when another active quarantine entry
// still has its file stored there.
func TestRestoreQuarantine_QuarantineDirRetained_WhenSiblingExists(t *testing.T) {
	svc, _, jobID := newTestService(t)

	destDir := t.TempDir()

	// Quarantine two independent files.
	pathA := filepath.Join(destDir, "a.txt")
	pathB := filepath.Join(destDir, "b.txt")
	if err := os.WriteFile(pathA, []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathB, []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

	eA, err := svc.Quarantine(context.Background(), jobID, "a.txt", pathA, destDir, "sha256")
	if err != nil {
		t.Fatalf("Quarantine a: %v", err)
	}
	eB, err := svc.Quarantine(context.Background(), jobID, "b.txt", pathB, destDir, "sha256")
	if err != nil {
		t.Fatalf("Quarantine b: %v", err)
	}
	_ = eB // still active

	quarantineRoot := filepath.Join(destDir, ".tidemarq-quarantine")

	// Restore only a.txt.
	if err := svc.RestoreQuarantine(context.Background(), eA.ID, destDir); err != nil {
		t.Fatalf("RestoreQuarantine: %v", err)
	}

	// .tidemarq-quarantine must still exist because b.txt is still in there.
	if _, err := os.Stat(quarantineRoot); err != nil {
		t.Errorf(".tidemarq-quarantine should remain while b.txt is still active: %v", err)
	}
	// b.txt quarantine file must still be present.
	if _, err := os.Stat(eB.QuarantinePath); err != nil {
		t.Errorf("b.txt quarantine file should not have been removed: %v", err)
	}
}

// TestDeleteQuarantine_QuarantineDirRetained_WhenSiblingExists mirrors the
// restore test but for permanent deletion.
func TestDeleteQuarantine_QuarantineDirRetained_WhenSiblingExists(t *testing.T) {
	svc, _, jobID := newTestService(t)

	destDir := t.TempDir()

	pathA := filepath.Join(destDir, "a.txt")
	pathB := filepath.Join(destDir, "b.txt")
	if err := os.WriteFile(pathA, []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathB, []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

	eA, err := svc.Quarantine(context.Background(), jobID, "a.txt", pathA, destDir, "sha256")
	if err != nil {
		t.Fatalf("Quarantine a: %v", err)
	}
	eB, err := svc.Quarantine(context.Background(), jobID, "b.txt", pathB, destDir, "sha256")
	if err != nil {
		t.Fatalf("Quarantine b: %v", err)
	}
	_ = eB

	quarantineRoot := filepath.Join(destDir, ".tidemarq-quarantine")

	if err := svc.DeleteQuarantine(context.Background(), eA.ID, destDir); err != nil {
		t.Fatalf("DeleteQuarantine: %v", err)
	}

	if _, err := os.Stat(quarantineRoot); err != nil {
		t.Errorf(".tidemarq-quarantine should remain while b.txt is still active: %v", err)
	}
	if _, err := os.Stat(eB.QuarantinePath); err != nil {
		t.Errorf("b.txt quarantine file should not have been removed: %v", err)
	}
}

// TestListVersions returns versions for a specific path.
func TestListVersions(t *testing.T) {
	svc, _, jobID := newTestService(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	for i := 0; i < 3; i++ {
		_ = os.WriteFile(path, []byte("v"), 0644)
		_, _ = svc.Snapshot(context.Background(), jobID, "file.txt", path, "sha256")
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
