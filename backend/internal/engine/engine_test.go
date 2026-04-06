package engine_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tidemarq/tidemarq/internal/conflicts"
	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/engine"
	"github.com/tidemarq/tidemarq/internal/manifest"
	"github.com/tidemarq/tidemarq/internal/versions"
	"github.com/tidemarq/tidemarq/migrations"
)

// testEnv sets up a temporary SQLite DB, manifest store, engine, and a job,
// returning everything needed to run engine tests.
func testEnv(t *testing.T) (eng *engine.Engine, jobID int64, src, dst string) {
	t.Helper()

	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	if err := d.Migrate(migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	job, err := d.CreateJob(context.Background(), db.CreateJobParams{
		Name:            "test",
		SourcePath:      "/src",
		DestinationPath: "/dst",
		Mode:            "one-way-backup",
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	store := manifest.New(d)
	eng = engine.New(store)
	src = t.TempDir()
	dst = t.TempDir()
	return eng, job.ID, src, dst
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestEngine_CopiesFilesToDestination(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	writeFile(t, filepath.Join(src, "hello.txt"), "hello world")
	writeFile(t, filepath.Join(src, "sub", "deep.txt"), "deep content")

	result, err := eng.Run(context.Background(), engine.Config{
		JobID:           jobID,
		SourcePath:      src,
		DestinationPath: dst,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if result.FilesCopied != 2 {
		t.Errorf("FilesCopied: got %d, want 2", result.FilesCopied)
	}
	if result.FilesSkipped != 0 {
		t.Errorf("FilesSkipped: got %d, want 0", result.FilesSkipped)
	}

	// Verify files exist at destination.
	for _, rel := range []string{"hello.txt", "sub/deep.txt"} {
		if _, err := os.Stat(filepath.Join(dst, filepath.FromSlash(rel))); os.IsNotExist(err) {
			t.Errorf("destination missing: %s", rel)
		}
	}
}

func TestEngine_Idempotent(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	writeFile(t, filepath.Join(src, "file.txt"), "unchanged content")

	cfg := engine.Config{JobID: jobID, SourcePath: src, DestinationPath: dst}

	first, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if first.FilesCopied != 1 {
		t.Fatalf("first run: FilesCopied = %d, want 1", first.FilesCopied)
	}

	second, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if second.FilesCopied != 0 {
		t.Errorf("second run: FilesCopied = %d, want 0 (idempotency)", second.FilesCopied)
	}
	if second.FilesSkipped != 1 {
		t.Errorf("second run: FilesSkipped = %d, want 1", second.FilesSkipped)
	}
}

func TestEngine_CopiesModifiedFile(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	path := filepath.Join(src, "file.txt")
	writeFile(t, path, "original")

	cfg := engine.Config{JobID: jobID, SourcePath: src, DestinationPath: dst}

	if _, err := eng.Run(context.Background(), cfg); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// Modify the source file.
	writeFile(t, path, "modified content")

	result, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if result.FilesCopied != 1 {
		t.Errorf("FilesCopied: got %d, want 1", result.FilesCopied)
	}

	// Confirm destination has the updated content.
	got, err := os.ReadFile(filepath.Join(dst, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "modified content" {
		t.Errorf("destination content: got %q, want %q", got, "modified content")
	}
}

func TestEngine_BackupModeDoesNotDeleteFromDest(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	path := filepath.Join(src, "file.txt")
	writeFile(t, path, "data")

	cfg := engine.Config{JobID: jobID, SourcePath: src, DestinationPath: dst}

	if _, err := eng.Run(context.Background(), cfg); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// Delete from source.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	if _, err := eng.Run(context.Background(), cfg); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	// File must still exist in destination.
	if _, err := os.Stat(filepath.Join(dst, "file.txt")); os.IsNotExist(err) {
		t.Error("destination file was deleted in backup mode (should never happen)")
	}
}

func TestEngine_PreservesMetadata(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not preserved on Windows; verified on Linux in CI")
	}

	eng, jobID, src, dst := testEnv(t)

	srcPath := filepath.Join(src, "file.txt")
	writeFile(t, srcPath, "data")
	if err := os.Chmod(srcPath, 0600); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	if _, err := eng.Run(context.Background(), engine.Config{
		JobID: jobID, SourcePath: src, DestinationPath: dst,
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	info, err := os.Stat(filepath.Join(dst, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("permissions: got %o, want %o", info.Mode().Perm(), 0600)
	}
}

func TestEngine_PauseStopsAfterCurrentFile(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	// Create several files.
	for i := 0; i < 5; i++ {
		writeFile(t, filepath.Join(src, fmt.Sprintf("file%d.txt", i)), fmt.Sprintf("content %d", i))
	}

	// Signal pause immediately so it fires on the first check.
	pauseCh := make(chan struct{})
	close(pauseCh)

	result, err := eng.Run(context.Background(), engine.Config{
		JobID:           jobID,
		SourcePath:      src,
		DestinationPath: dst,
		PauseCh:         pauseCh,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Paused {
		t.Error("expected Result.Paused = true")
	}
}

func TestEngine_ResumeAfterPause(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	writeFile(t, filepath.Join(src, "a.txt"), "aaa")
	writeFile(t, filepath.Join(src, "b.txt"), "bbb")

	cfg := engine.Config{JobID: jobID, SourcePath: src, DestinationPath: dst}

	// Normal run — copies both files.
	if _, err := eng.Run(context.Background(), cfg); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// Modify both files.
	writeFile(t, filepath.Join(src, "a.txt"), "aaa-modified")
	writeFile(t, filepath.Join(src, "b.txt"), "bbb-modified")

	// Pause immediately on first file.
	pauseCh := make(chan struct{})
	close(pauseCh)
	cfg.PauseCh = pauseCh

	r1, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("paused Run: %v", err)
	}
	if !r1.Paused {
		t.Fatal("expected paused result")
	}

	// Resume — no pause channel; should pick up remaining file.
	cfg.PauseCh = nil
	r2, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("resume Run: %v", err)
	}
	if r2.Paused {
		t.Error("expected run to complete without pausing")
	}
	// Together the two runs should have copied both modified files.
	if r1.FilesCopied+r2.FilesCopied != 2 {
		t.Errorf("total FilesCopied: got %d, want 2", r1.FilesCopied+r2.FilesCopied)
	}
}

func TestEngine_RecopiesIfDestinationDeleted(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	writeFile(t, filepath.Join(src, "file.txt"), "data")
	cfg := engine.Config{JobID: jobID, SourcePath: src, DestinationPath: dst}

	// First run — copies the file.
	if _, err := eng.Run(context.Background(), cfg); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// Delete the destination file manually.
	if err := os.Remove(filepath.Join(dst, "file.txt")); err != nil {
		t.Fatal(err)
	}

	// Second run — source unchanged, but destination missing; should re-copy.
	result, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if result.FilesCopied != 1 {
		t.Errorf("FilesCopied: got %d, want 1", result.FilesCopied)
	}
	if _, err := os.Stat(filepath.Join(dst, "file.txt")); os.IsNotExist(err) {
		t.Error("destination file still missing after second run")
	}
}

// testEnvFull extends testEnv with versions and conflicts services for mode tests.
func testEnvFull(t *testing.T) (eng *engine.Engine, jobID int64, src, dst string, vSvc *versions.Service, cSvc *conflicts.Service) {
	t.Helper()

	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	if err := d.Migrate(migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	src = t.TempDir()
	dst = t.TempDir()

	job, err := d.CreateJob(context.Background(), db.CreateJobParams{
		Name:            "test",
		SourcePath:      src,
		DestinationPath: dst,
		Mode:            "two-way",
	})
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	store := manifest.New(d)
	eng = engine.New(store)
	vSvc = versions.New(d, t.TempDir(), 30)
	cSvc = conflicts.New(d)
	return eng, job.ID, src, dst, vSvc, cSvc
}

// ---------------------------------------------------------------------------
// Two-way sync tests
// ---------------------------------------------------------------------------

// TestEngine_TwoWay_PropagatesDestChange verifies that a change made on the
// destination side is propagated back to the source on the next run.
func TestEngine_TwoWay_PropagatesDestChange(t *testing.T) {
	eng, jobID, src, dst, vSvc, cSvc := testEnvFull(t)

	writeFile(t, filepath.Join(src, "file.txt"), "original")

	cfg := engine.Config{
		JobID: jobID, Mode: "two-way",
		SourcePath: src, DestinationPath: dst,
		VersionsSvc: vSvc, ConflictsSvc: cSvc,
	}

	// First run — syncs source → dest.
	if _, err := eng.Run(context.Background(), cfg); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dst, "file.txt"))
	if string(got) != "original" {
		t.Fatalf("dest content after first run: %q", got)
	}

	// Modify the destination (simulating a remote edit).
	writeFile(t, filepath.Join(dst, "file.txt"), "edited on dest")

	// Second run — should propagate dest change back to source.
	result, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("errors: %v", result.Errors)
	}
	if result.FilesCopied != 1 {
		t.Errorf("FilesCopied: got %d, want 1", result.FilesCopied)
	}

	got, _ = os.ReadFile(filepath.Join(src, "file.txt"))
	if string(got) != "edited on dest" {
		t.Errorf("source content after two-way sync: got %q, want %q", got, "edited on dest")
	}
}

// TestEngine_TwoWay_Idempotent verifies that a second run with no changes
// produces zero copies.
func TestEngine_TwoWay_Idempotent(t *testing.T) {
	eng, jobID, src, dst, vSvc, cSvc := testEnvFull(t)

	writeFile(t, filepath.Join(src, "file.txt"), "stable")

	cfg := engine.Config{
		JobID: jobID, Mode: "two-way",
		SourcePath: src, DestinationPath: dst,
		VersionsSvc: vSvc, ConflictsSvc: cSvc,
	}

	if _, err := eng.Run(context.Background(), cfg); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	second, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if second.FilesCopied != 0 {
		t.Errorf("idempotency: FilesCopied = %d on second run, want 0", second.FilesCopied)
	}
}

// TestEngine_TwoWay_ConflictAskUser verifies that when both sides change the same
// file and strategy is ask-user, the conflict is recorded but neither file is
// modified and no .conflict.<timestamp> file is created.  Resolution is deferred
// to the user via the API.
func TestEngine_TwoWay_ConflictAskUser(t *testing.T) {
	eng, jobID, src, dst, vSvc, cSvc := testEnvFull(t)

	writeFile(t, filepath.Join(src, "file.txt"), "original")

	cfg := engine.Config{
		JobID: jobID, Mode: "two-way", ConflictStrategy: "ask-user",
		SourcePath: src, DestinationPath: dst,
		VersionsSvc: vSvc, ConflictsSvc: cSvc,
	}

	// First run — establishes manifest baseline.
	if _, err := eng.Run(context.Background(), cfg); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// Modify both sides independently.
	writeFile(t, filepath.Join(src, "file.txt"), "source version")
	writeFile(t, filepath.Join(dst, "file.txt"), "dest version")

	// Second run — should detect conflict without touching the filesystem.
	result, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("conflict Run: %v", err)
	}
	if result.Conflicts != 1 {
		t.Errorf("Conflicts: got %d, want 1", result.Conflicts)
	}

	// Both files must be untouched — neither side is modified until the user resolves.
	gotSrc, _ := os.ReadFile(filepath.Join(src, "file.txt"))
	if string(gotSrc) != "source version" {
		t.Errorf("src content changed unexpectedly: got %q", gotSrc)
	}
	gotDest, _ := os.ReadFile(filepath.Join(dst, "file.txt"))
	if string(gotDest) != "dest version" {
		t.Errorf("dest content changed unexpectedly: got %q", gotDest)
	}

	// No .conflict.* files should have been created.
	entries, _ := os.ReadDir(dst)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".conflict.") {
			t.Errorf("unexpected conflict file created at detection time: %s", e.Name())
		}
	}

	// Running the engine again (simulating a second sync before resolution) must
	// record only one conflict — CreateConflict is idempotent.
	result2, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("third Run: %v", err)
	}
	if result2.Conflicts != 1 {
		t.Errorf("second conflict run Conflicts: got %d, want 1", result2.Conflicts)
	}
}

// TestEngine_TwoWay_ConflictSourceWins verifies that source-wins auto-resolves
// without creating a conflict file.
func TestEngine_TwoWay_ConflictSourceWins(t *testing.T) {
	eng, jobID, src, dst, vSvc, cSvc := testEnvFull(t)

	writeFile(t, filepath.Join(src, "file.txt"), "original")

	cfg := engine.Config{
		JobID: jobID, Mode: "two-way", ConflictStrategy: "source-wins",
		SourcePath: src, DestinationPath: dst,
		VersionsSvc: vSvc, ConflictsSvc: cSvc,
	}

	if _, err := eng.Run(context.Background(), cfg); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	writeFile(t, filepath.Join(src, "file.txt"), "source version")
	writeFile(t, filepath.Join(dst, "file.txt"), "dest version")

	result, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("conflict Run: %v", err)
	}
	if result.Conflicts != 1 {
		t.Errorf("Conflicts: got %d, want 1", result.Conflicts)
	}

	// Source should win — dest gets source content; no .conflict file created.
	got, _ := os.ReadFile(filepath.Join(dst, "file.txt"))
	if string(got) != "source version" {
		t.Errorf("dest content: got %q, want %q", got, "source version")
	}

	entries, _ := os.ReadDir(dst)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".conflict.") {
			t.Errorf("unexpected .conflict file: %s", e.Name())
		}
	}
}

// ---------------------------------------------------------------------------
// Mirror mode tests
// ---------------------------------------------------------------------------

// TestEngine_Mirror_QuarantinesDeletedFile verifies that a file deleted from
// source is moved to quarantine (not hard-deleted) in dest.
func TestEngine_Mirror_QuarantinesDeletedFile(t *testing.T) {
	eng, jobID, src, dst, vSvc, _ := testEnvFull(t)

	writeFile(t, filepath.Join(src, "keep.txt"), "keep")
	writeFile(t, filepath.Join(src, "remove.txt"), "remove")

	cfg := engine.Config{
		JobID: jobID, Mode: "one-way-mirror",
		SourcePath: src, DestinationPath: dst,
		VersionsSvc: vSvc,
	}

	// First run — syncs both files to dest.
	if _, err := eng.Run(context.Background(), cfg); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "remove.txt")); os.IsNotExist(err) {
		t.Fatal("remove.txt should exist in dest after first run")
	}

	// Delete remove.txt from source.
	if err := os.Remove(filepath.Join(src, "remove.txt")); err != nil {
		t.Fatal(err)
	}

	// Second run — remove.txt should be quarantined from dest.
	result, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if result.Quarantined != 1 {
		t.Errorf("Quarantined: got %d, want 1", result.Quarantined)
	}

	// File should be gone from dest.
	if _, err := os.Stat(filepath.Join(dst, "remove.txt")); !os.IsNotExist(err) {
		t.Error("remove.txt should have been quarantined from dest but still exists")
	}

	// keep.txt must remain untouched.
	if _, err := os.Stat(filepath.Join(dst, "keep.txt")); os.IsNotExist(err) {
		t.Error("keep.txt should still exist in dest")
	}
}

// TestEngine_Mirror_PrunesEmptyDirAfterQuarantine verifies that when all files
// in a source subdirectory are deleted, the corresponding empty directory is
// removed from the destination after the mirror sync quarantines its contents.
func TestEngine_Mirror_PrunesEmptyDirAfterQuarantine(t *testing.T) {
	eng, jobID, src, dst, vSvc, _ := testEnvFull(t)

	// Create a dedicated subdirectory with two files.
	writeFile(t, filepath.Join(src, "docs", "a.txt"), "a")
	writeFile(t, filepath.Join(src, "docs", "b.txt"), "b")
	writeFile(t, filepath.Join(src, "root.txt"), "root") // file at dest root — must survive

	cfg := engine.Config{
		JobID: jobID, Mode: "one-way-mirror",
		SourcePath: src, DestinationPath: dst,
		VersionsSvc: vSvc,
	}

	// First run: sync everything to dest.
	if _, err := eng.Run(context.Background(), cfg); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "docs")); err != nil {
		t.Fatal("docs/ should exist in dest after first run")
	}

	// Remove the entire docs/ directory from source.
	if err := os.RemoveAll(filepath.Join(src, "docs")); err != nil {
		t.Fatal(err)
	}

	// Second run: both files should be quarantined.
	result, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if result.Quarantined != 2 {
		t.Errorf("expected 2 quarantined files, got %d", result.Quarantined)
	}

	// docs/ should have been pruned from dest — it is now empty.
	if _, err := os.Stat(filepath.Join(dst, "docs")); err == nil {
		t.Error("expected empty docs/ to be removed from dest after quarantine")
	}
	// Root-level file and dest root itself must be untouched.
	if _, err := os.Stat(filepath.Join(dst, "root.txt")); err != nil {
		t.Errorf("root.txt should still be in dest: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Errorf("destination root must not be removed: %v", err)
	}
}

// TestEngine_Mirror_RetainsDirWhenSiblingFileRemains verifies that a
// destination directory is NOT pruned when only some of its files are removed
// from the source — a sibling file keeps it alive.
func TestEngine_Mirror_RetainsDirWhenSiblingFileRemains(t *testing.T) {
	eng, jobID, src, dst, vSvc, _ := testEnvFull(t)

	writeFile(t, filepath.Join(src, "docs", "keep.txt"), "keep")
	writeFile(t, filepath.Join(src, "docs", "remove.txt"), "remove")

	cfg := engine.Config{
		JobID: jobID, Mode: "one-way-mirror",
		SourcePath: src, DestinationPath: dst,
		VersionsSvc: vSvc,
	}

	if _, err := eng.Run(context.Background(), cfg); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// Remove only one of the two files.
	if err := os.Remove(filepath.Join(src, "docs", "remove.txt")); err != nil {
		t.Fatal(err)
	}

	result, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if result.Quarantined != 1 {
		t.Errorf("expected 1 quarantined file, got %d", result.Quarantined)
	}

	// docs/ should still exist — keep.txt is still there.
	if _, err := os.Stat(filepath.Join(dst, "docs")); err != nil {
		t.Errorf("docs/ should still exist while keep.txt remains: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "docs", "keep.txt")); err != nil {
		t.Errorf("keep.txt should still be in dest: %v", err)
	}
}

// TestEngine_Mirror_PrunesNestedEmptyDirs verifies bottom-up pruning across
// multiple levels of nesting when an entire subtree is removed from source.
func TestEngine_Mirror_PrunesNestedEmptyDirs(t *testing.T) {
	eng, jobID, src, dst, vSvc, _ := testEnvFull(t)

	writeFile(t, filepath.Join(src, "a", "b", "c", "deep.txt"), "deep")

	cfg := engine.Config{
		JobID: jobID, Mode: "one-way-mirror",
		SourcePath: src, DestinationPath: dst,
		VersionsSvc: vSvc,
	}

	if _, err := eng.Run(context.Background(), cfg); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	if err := os.RemoveAll(filepath.Join(src, "a")); err != nil {
		t.Fatal(err)
	}

	if _, err := eng.Run(context.Background(), cfg); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	// All three ancestor directories should be pruned.
	for _, dir := range []string{
		filepath.Join(dst, "a", "b", "c"),
		filepath.Join(dst, "a", "b"),
		filepath.Join(dst, "a"),
	} {
		if _, err := os.Stat(dir); err == nil {
			t.Errorf("expected empty dir to be removed from dest: %s", dir)
		}
	}
	// Destination root must survive.
	if _, err := os.Stat(dst); err != nil {
		t.Errorf("destination root must not be removed: %v", err)
	}
}

// TestEngine_TwoWay_PrunesEmptyDirAfterQuarantine verifies the same pruning
// behaviour in two-way mode when the source side of a file is deleted.
func TestEngine_TwoWay_PrunesEmptyDirAfterQuarantine(t *testing.T) {
	eng, jobID, src, dst, vSvc, cSvc := testEnvFull(t)

	writeFile(t, filepath.Join(src, "shared", "file.txt"), "data")

	cfg := engine.Config{
		JobID: jobID, Mode: "two-way",
		SourcePath: src, DestinationPath: dst,
		VersionsSvc: vSvc, ConflictsSvc: cSvc,
		ConflictStrategy: "ask-user",
	}

	// First run: establish baseline.
	if _, err := eng.Run(context.Background(), cfg); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// Delete the source copy — dest still has it, so the engine should
	// quarantine the dest copy in two-way mode.
	if err := os.RemoveAll(filepath.Join(src, "shared")); err != nil {
		t.Fatal(err)
	}

	if _, err := eng.Run(context.Background(), cfg); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	// shared/ should be pruned from dest after the file was quarantined.
	if _, err := os.Stat(filepath.Join(dst, "shared")); err == nil {
		t.Error("expected empty shared/ to be removed from dest after quarantine")
	}
	if _, err := os.Stat(dst); err != nil {
		t.Errorf("destination root must not be removed: %v", err)
	}
}

// TestEngine_Mirror_Idempotent verifies that a second mirror run with no
// source changes produces zero copies and zero quarantine events.
func TestEngine_Mirror_Idempotent(t *testing.T) {
	eng, jobID, src, dst, vSvc, _ := testEnvFull(t)

	writeFile(t, filepath.Join(src, "file.txt"), "data")

	cfg := engine.Config{
		JobID: jobID, Mode: "one-way-mirror",
		SourcePath: src, DestinationPath: dst,
		VersionsSvc: vSvc,
	}

	if _, err := eng.Run(context.Background(), cfg); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	second, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if second.FilesCopied != 0 {
		t.Errorf("idempotency: FilesCopied = %d, want 0", second.FilesCopied)
	}
	if second.Quarantined != 0 {
		t.Errorf("idempotency: Quarantined = %d, want 0", second.Quarantined)
	}
}

// TestEngine_BandwidthLimit verifies that a non-zero bandwidth limit slows
// the transfer to at most 2× the configured rate (generous bound to avoid
// flakiness, while still catching an unthrottled transfer).
func TestEngine_BandwidthLimit(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	// 512 KB file, limited to 256 KB/s → expect at least ~1s transfer.
	size := 512 * 1024
	data := make([]byte, size)
	writeFile(t, filepath.Join(src, "big.bin"), string(data))

	start := time.Now()
	result, err := eng.Run(context.Background(), engine.Config{
		JobID:            jobID,
		SourcePath:       src,
		DestinationPath:  dst,
		BandwidthLimitKB: 256,
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("errors: %v", result.Errors)
	}
	// At 256 KB/s, 512 KB should take ≥ 1 second.
	if elapsed < time.Second {
		t.Errorf("transfer completed in %v — bandwidth limit does not appear to be enforced", elapsed)
	}
}

func TestEngine_ChecksumVerification(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	writeFile(t, filepath.Join(src, "file.txt"), "source content")

	// Pre-place a corrupt file at the destination that will be overwritten correctly.
	writeFile(t, filepath.Join(dst, "file.txt"), "corrupt")

	result, err := eng.Run(context.Background(), engine.Config{
		JobID: jobID, SourcePath: src, DestinationPath: dst,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	got, err := os.ReadFile(filepath.Join(dst, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "source content" {
		t.Errorf("content: got %q, want %q", got, "source content")
	}
}
