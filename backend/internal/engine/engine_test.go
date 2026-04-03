package engine_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/engine"
	"github.com/tidemarq/tidemarq/internal/manifest"
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
