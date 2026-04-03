package jobs_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/engine"
	"github.com/tidemarq/tidemarq/internal/jobs"
	"github.com/tidemarq/tidemarq/internal/manifest"
	"github.com/tidemarq/tidemarq/internal/watch"
	"github.com/tidemarq/tidemarq/internal/ws"
	"github.com/tidemarq/tidemarq/migrations"
)

func newTestService(t *testing.T) (*jobs.Service, string, string) {
	t.Helper()

	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	if err := d.Migrate(migrations.FS); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	src := t.TempDir()
	dst := t.TempDir()

	hub := ws.New()
	watcher, err := watch.New()
	if err != nil {
		t.Fatalf("watch.New: %v", err)
	}
	t.Cleanup(watcher.Close)

	store := manifest.New(d)
	eng := engine.New(store)
	svc := jobs.New(d, eng, hub, watcher, nil, nil)
	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(svc.Stop)

	return svc, src, dst
}

// waitStatus polls until the job reaches wantStatus or timeout expires.
func waitStatus(t *testing.T, svc *jobs.Service, id int64, wantStatus string, timeout time.Duration) *db.Job {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		got, _ := svc.Get(context.Background(), id)
		if got != nil && got.Status == wantStatus {
			return got
		}
		time.Sleep(50 * time.Millisecond)
	}
	got, _ := svc.Get(context.Background(), id)
	status := "unknown"
	if got != nil {
		status = got.Status
	}
	t.Errorf("timeout waiting for status %q: current status %q", wantStatus, status)
	return got
}

// waitRunComplete polls until the job has last_run_at set (run finished) and
// status is idle or error. For fast jobs that complete before "running" is observable.
func waitRunComplete(t *testing.T, svc *jobs.Service, id int64, timeout time.Duration) *db.Job {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		got, _ := svc.Get(context.Background(), id)
		if got != nil && got.LastRunAt != nil && got.Status != "running" {
			return got
		}
		time.Sleep(50 * time.Millisecond)
	}
	got, _ := svc.Get(context.Background(), id)
	t.Errorf("timeout waiting for run to complete")
	return got
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

func TestService_CreateAndGet(t *testing.T) {
	svc, src, dst := newTestService(t)
	ctx := context.Background()

	j, err := svc.Create(ctx, jobs.CreateParams{
		Name:            "test",
		SourcePath:      src,
		DestinationPath: dst,
		Mode:            "one-way-backup",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if j.Status != "idle" {
		t.Errorf("Status: got %q, want idle", j.Status)
	}

	got, err := svc.Get(ctx, j.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != j.ID {
		t.Errorf("ID mismatch: got %d, want %d", got.ID, j.ID)
	}
}

func TestService_GetNotFound(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.Get(context.Background(), 9999)
	if !errors.Is(err, jobs.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestService_List(t *testing.T) {
	svc, src, dst := newTestService(t)
	ctx := context.Background()

	for _, name := range []string{"alpha", "beta"} {
		if _, err := svc.Create(ctx, jobs.CreateParams{
			Name: name, SourcePath: src, DestinationPath: dst, Mode: "one-way-backup",
		}); err != nil {
			t.Fatalf("Create %q: %v", name, err)
		}
	}

	list, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len: got %d, want 2", len(list))
	}
}

func TestService_Update(t *testing.T) {
	svc, src, dst := newTestService(t)
	ctx := context.Background()

	j, err := svc.Create(ctx, jobs.CreateParams{
		Name: "old-name", SourcePath: src, DestinationPath: dst, Mode: "one-way-backup",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := svc.Update(ctx, j.ID, jobs.UpdateParams{
		Name: "new-name", SourcePath: src, DestinationPath: dst, Mode: "one-way-backup",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "new-name" {
		t.Errorf("Name: got %q, want new-name", updated.Name)
	}
}

func TestService_Delete(t *testing.T) {
	svc, src, dst := newTestService(t)
	ctx := context.Background()

	j, err := svc.Create(ctx, jobs.CreateParams{
		Name: "to-delete", SourcePath: src, DestinationPath: dst, Mode: "one-way-backup",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Delete(ctx, j.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := svc.Get(ctx, j.ID); !errors.Is(err, jobs.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestService_Run_CompletesSuccessfully(t *testing.T) {
	svc, src, dst := newTestService(t)
	ctx := context.Background()

	writeFile(t, filepath.Join(src, "file.txt"), "hello")

	j, err := svc.Create(ctx, jobs.CreateParams{
		Name: "run-test", SourcePath: src, DestinationPath: dst, Mode: "one-way-backup",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Run(ctx, j.ID); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := waitRunComplete(t, svc, j.ID, 10*time.Second)
	if got != nil && got.Status != "idle" {
		t.Errorf("Status: got %q, want idle", got.Status)
	}
	if _, err := os.Stat(filepath.Join(dst, "file.txt")); os.IsNotExist(err) {
		t.Error("file.txt not found in destination")
	}
}

func TestService_Run_AlreadyRunning(t *testing.T) {
	svc, src, dst := newTestService(t)
	ctx := context.Background()

	// Create a large-ish file so the job takes a moment.
	data := make([]byte, 512*1024)
	writeFile(t, filepath.Join(src, "big.bin"), string(data))

	j, err := svc.Create(ctx, jobs.CreateParams{
		Name: "concurrent", SourcePath: src, DestinationPath: dst, Mode: "one-way-backup",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Run(ctx, j.ID); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if err := svc.Run(ctx, j.ID); !errors.Is(err, jobs.ErrAlreadyRunning) {
		t.Errorf("expected ErrAlreadyRunning, got %v", err)
	}

	// Let it finish.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := svc.Get(ctx, j.ID)
		if got.Status != "running" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestService_Pause_AndResume(t *testing.T) {
	svc, src, dst := newTestService(t)
	ctx := context.Background()

	// Use a bandwidth limit and enough files to ensure the job is still running
	// when Pause is called.
	data := string(make([]byte, 32*1024)) // 32 KB per file
	for i := 0; i < 10; i++ {
		writeFile(t, filepath.Join(src, fmt.Sprintf("file%02d.bin", i)), data)
	}

	j, err := svc.Create(ctx, jobs.CreateParams{
		Name: "pause-test", SourcePath: src, DestinationPath: dst,
		Mode: "one-way-backup", BandwidthLimitKB: 64, // slow enough to catch mid-run
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Run(ctx, j.ID); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Wait until running before pausing.
	waitStatus(t, svc, j.ID, "running", 5*time.Second)

	if err := svc.Pause(ctx, j.ID); err != nil {
		t.Fatalf("Pause: %v", err)
	}

	// Wait for paused (or idle if it finished before pause took effect).
	deadline := time.Now().Add(5 * time.Second)
	var got *db.Job
	for time.Now().Before(deadline) {
		got, _ = svc.Get(ctx, j.ID)
		if got != nil && (got.Status == "paused" || got.Status == "idle") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got == nil || (got.Status != "paused" && got.Status != "idle") {
		status := "unknown"
		if got != nil {
			status = got.Status
		}
		t.Fatalf("after pause: Status = %q, want paused or idle", status)
	}

	// Resume and wait for completion.
	if err := svc.Resume(ctx, j.ID); err != nil {
		t.Fatalf("Resume: %v", err)
	}

	waitStatus(t, svc, j.ID, "running", 5*time.Second)
	waitStatus(t, svc, j.ID, "idle", 10*time.Second)

	// All files must be in destination after resume completes.
	for i := 0; i < 10; i++ {
		p := filepath.Join(dst, fmt.Sprintf("file%02d.bin", i))
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("missing in destination: file%02d.bin", i)
		}
	}
}

func TestService_InvalidMode(t *testing.T) {
	svc, src, dst := newTestService(t)
	_, err := svc.Create(context.Background(), jobs.CreateParams{
		Name: "bad", SourcePath: src, DestinationPath: dst, Mode: "invalid",
	})
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestService_InvalidCronSchedule(t *testing.T) {
	svc, src, dst := newTestService(t)
	_, err := svc.Create(context.Background(), jobs.CreateParams{
		Name: "bad-cron", SourcePath: src, DestinationPath: dst,
		Mode: "one-way-backup", CronSchedule: "not-a-cron",
	})
	if err == nil {
		t.Error("expected error for invalid cron schedule")
	}
}

func TestService_WatchTrigger(t *testing.T) {
	svc, src, dst := newTestService(t)
	ctx := context.Background()

	j, err := svc.Create(ctx, jobs.CreateParams{
		Name:            "watch-test",
		SourcePath:      src,
		DestinationPath: dst,
		Mode:            "one-way-backup",
		WatchEnabled:    true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	_ = j

	// Write a file to trigger the watcher.
	writeFile(t, filepath.Join(src, "watched.txt"), "triggered")

	// Wait up to 10 seconds for the watch trigger + debounce + run to complete.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(dst, "watched.txt")); err == nil {
			return // success
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Error("watched.txt did not appear in destination within 10 seconds")
}
