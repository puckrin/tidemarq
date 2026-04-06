package conflicts_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tidemarq/tidemarq/internal/conflicts"
	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/migrations"
)

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if err := d.Migrate(migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return d
}

func seedJob(t *testing.T, d *db.DB) int64 {
	t.Helper()
	j, err := d.CreateJob(context.Background(), db.CreateJobParams{
		Name:            "test",
		SourcePath:      t.TempDir(),
		DestinationPath: t.TempDir(),
		Mode:            "two-way",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	return j.ID
}

// TestDetect_NoConflict verifies that if only one side changed, Detect returns false.
func TestDetect_NoConflict(t *testing.T) {
	syncedHash := "aaaa"
	src := conflicts.FileState{Exists: true, SHA256: "bbbb"} // src changed
	dest := conflicts.FileState{Exists: true, SHA256: "aaaa"} // dest unchanged

	isConflict, srcChanged, destChanged := conflicts.Detect(syncedHash, src, dest)
	if isConflict {
		t.Error("expected no conflict when only src changed")
	}
	if !srcChanged {
		t.Error("expected srcChanged=true")
	}
	if destChanged {
		t.Error("expected destChanged=false")
	}
}

// TestDetect_Conflict verifies that both-sides-changed is detected.
func TestDetect_Conflict(t *testing.T) {
	syncedHash := "aaaa"
	src := conflicts.FileState{Exists: true, SHA256: "bbbb"}
	dest := conflicts.FileState{Exists: true, SHA256: "cccc"}

	isConflict, _, _ := conflicts.Detect(syncedHash, src, dest)
	if !isConflict {
		t.Error("expected conflict when both sides changed")
	}
}

// TestDetect_NeitherChanged verifies idempotency: no conflict when nothing changed.
func TestDetect_NeitherChanged(t *testing.T) {
	syncedHash := "aaaa"
	src := conflicts.FileState{Exists: true, SHA256: "aaaa"}
	dest := conflicts.FileState{Exists: true, SHA256: "aaaa"}

	isConflict, srcChanged, destChanged := conflicts.Detect(syncedHash, src, dest)
	if isConflict || srcChanged || destChanged {
		t.Error("expected no changes when both match synced hash")
	}
}

// TestRecord_AndGet verifies that a conflict can be recorded and retrieved.
func TestRecord_AndGet(t *testing.T) {
	d := newTestDB(t)
	jobID := seedJob(t, d)
	svc := conflicts.New(d)

	src := conflicts.FileState{Exists: true, SHA256: "src-hash", Size: 100, ModTime: time.Now()}
	dest := conflicts.FileState{Exists: true, SHA256: "dest-hash", Size: 200, ModTime: time.Now()}

	c, err := svc.Record(context.Background(), jobID, "dir/file.txt", "ask-user", src, dest)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}
	if c.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if c.Status != "pending" {
		t.Errorf("expected status=pending, got %s", c.Status)
	}

	got, err := svc.Get(context.Background(), c.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.RelPath != "dir/file.txt" {
		t.Errorf("unexpected RelPath: %s", got.RelPath)
	}
}

// TestList verifies filtering by job ID.
func TestList_FiltersByJob(t *testing.T) {
	d := newTestDB(t)
	jobID := seedJob(t, d)
	svc := conflicts.New(d)

	src := conflicts.FileState{Exists: true, SHA256: "s", ModTime: time.Now()}
	dest := conflicts.FileState{Exists: true, SHA256: "d", ModTime: time.Now()}

	_, _ = svc.Record(context.Background(), jobID, "a.txt", "ask-user", src, dest)
	_, _ = svc.Record(context.Background(), jobID, "b.txt", "ask-user", src, dest)

	all, err := svc.List(context.Background(), 0)
	if err != nil {
		t.Fatalf("List all: %v", err)
	}
	if len(all) < 2 {
		t.Errorf("expected at least 2 conflicts, got %d", len(all))
	}

	filtered, err := svc.List(context.Background(), jobID)
	if err != nil {
		t.Fatalf("List by job: %v", err)
	}
	if len(filtered) != 2 {
		t.Errorf("expected 2 conflicts for job, got %d", len(filtered))
	}
}

// TestAutoResolve_SourceWins verifies source-wins copies source path.
func TestAutoResolve_SourceWins(t *testing.T) {
	src := conflicts.FileState{SHA256: "src", Size: 50, ModTime: time.Now()}
	dest := conflicts.FileState{SHA256: "dest", Size: 100, ModTime: time.Now()}

	winner, err := conflicts.AutoResolve("source-wins", "/src/file", "/dest/file", src, dest)
	if err != nil {
		t.Fatalf("AutoResolve: %v", err)
	}
	if winner != "/src/file" {
		t.Errorf("expected source to win, got %s", winner)
	}
}

// TestAutoResolve_DestinationWins verifies destination-wins keeps dest.
func TestAutoResolve_DestinationWins(t *testing.T) {
	src := conflicts.FileState{SHA256: "src", ModTime: time.Now()}
	dest := conflicts.FileState{SHA256: "dest", ModTime: time.Now()}

	winner, err := conflicts.AutoResolve("destination-wins", "/src/file", "/dest/file", src, dest)
	if err != nil {
		t.Fatalf("AutoResolve: %v", err)
	}
	if winner != "/dest/file" {
		t.Errorf("expected dest to win, got %s", winner)
	}
}

// TestAutoResolve_NewestWins verifies newer mtime wins.
func TestAutoResolve_NewestWins(t *testing.T) {
	older := time.Now().Add(-time.Hour)
	newer := time.Now()

	src := conflicts.FileState{SHA256: "src", ModTime: newer}
	dest := conflicts.FileState{SHA256: "dest", ModTime: older}

	winner, err := conflicts.AutoResolve("newest-wins", "/src/file", "/dest/file", src, dest)
	if err != nil {
		t.Fatalf("AutoResolve: %v", err)
	}
	if winner != "/src/file" {
		t.Errorf("expected newer src to win, got %s", winner)
	}
}

// TestAutoResolve_LargestWins verifies larger file wins.
func TestAutoResolve_LargestWins(t *testing.T) {
	src := conflicts.FileState{SHA256: "src", Size: 100, ModTime: time.Now()}
	dest := conflicts.FileState{SHA256: "dest", Size: 200, ModTime: time.Now()}

	winner, err := conflicts.AutoResolve("largest-wins", "/src/file", "/dest/file", src, dest)
	if err != nil {
		t.Fatalf("AutoResolve: %v", err)
	}
	if winner != "/dest/file" {
		t.Errorf("expected larger dest to win, got %s", winner)
	}
}

// TestAutoResolve_AskUser_RenamesDest verifies ask-user renames dest with conflict suffix.
func TestAutoResolve_AskUser_RenamesDest(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "file.txt")
	destPath := filepath.Join(dir, "dest_file.txt")

	if err := os.WriteFile(srcPath, []byte("src content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destPath, []byte("dest content"), 0644); err != nil {
		t.Fatal(err)
	}

	src := conflicts.FileState{SHA256: "src", ModTime: time.Now()}
	dest := conflicts.FileState{SHA256: "dest", ModTime: time.Now()}

	winner, err := conflicts.AutoResolve("ask-user", srcPath, destPath, src, dest)
	if err != nil {
		t.Fatalf("AutoResolve ask-user: %v", err)
	}
	// Source should be returned as winner.
	if winner != srcPath {
		t.Errorf("expected srcPath as winner, got %s", winner)
	}
	// Dest file should have been renamed, not still exist at original path.
	if _, err := os.Stat(destPath); err == nil {
		t.Error("expected dest to be renamed away but it still exists at original path")
	}
	// A .conflict.* file should exist in the same directory.
	entries, _ := os.ReadDir(dir)
	found := false
	for _, e := range entries {
		if len(e.Name()) > len("dest_file.txt.conflict.") &&
			e.Name()[:len("dest_file.txt.conflict.")] == "dest_file.txt.conflict." {
			found = true
		}
	}
	if !found {
		t.Error("expected a .conflict.<timestamp> file to be created")
	}
}
