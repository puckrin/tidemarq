package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/tidemarq/tidemarq/internal/db"
)

// TestCreateAndLatestJobRun verifies that CreateJobRun persists all fields and
// LatestJobRun returns the most recent successful run, skipping paused/errored ones.
func TestCreateAndLatestJobRun(t *testing.T) {
	d := newJobTestDB(t)
	ctx := context.Background()

	job, err := d.CreateJob(ctx, db.CreateJobParams{
		Name:            "history-job",
		SourcePath:      t.TempDir(),
		DestinationPath: t.TempDir(),
		Mode:            "one-way-backup",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	now := time.Now()
	// Insert: older completed → errored → newer completed → paused.
	// LatestJobRun should return the newer completed run.
	older := mustCreateRun(t, d, ctx, db.CreateJobRunParams{
		JobID:         job.ID,
		StartedAt:     now.Add(-3 * time.Hour),
		EndedAt:       now.Add(-3*time.Hour + 10*time.Minute),
		Outcome:       "completed",
		FilesCopied:   5,
		BytesCopied:   1024,
		BytesReviewed: 2048,
	})
	mustCreateRun(t, d, ctx, db.CreateJobRunParams{
		JobID:     job.ID,
		StartedAt: now.Add(-2 * time.Hour),
		EndedAt:   now.Add(-2*time.Hour + 1*time.Minute),
		Outcome:   "error",
	})
	newer := mustCreateRun(t, d, ctx, db.CreateJobRunParams{
		JobID:         job.ID,
		StartedAt:     now.Add(-1 * time.Hour),
		EndedAt:       now.Add(-1*time.Hour + 8*time.Minute),
		Outcome:       "completed",
		FilesCopied:   3,
		FilesSkipped:  10,
		BytesCopied:   512,
		BytesReviewed: 5000,
	})
	mustCreateRun(t, d, ctx, db.CreateJobRunParams{
		JobID:     job.ID,
		StartedAt: now.Add(-30 * time.Minute),
		EndedAt:   now.Add(-29 * time.Minute),
		Outcome:   "paused",
	})

	latest, err := d.LatestJobRun(ctx, job.ID)
	if err != nil {
		t.Fatalf("LatestJobRun: %v", err)
	}
	if latest == nil {
		t.Fatal("LatestJobRun: got nil, want the newer completed run")
	}
	if latest.ID != newer.ID {
		t.Errorf("LatestJobRun: got run %d, want %d (newer completed run)", latest.ID, newer.ID)
	}
	if latest.BytesReviewed != 5000 || latest.BytesCopied != 512 {
		t.Errorf("LatestJobRun fields: BytesReviewed=%d, BytesCopied=%d", latest.BytesReviewed, latest.BytesCopied)
	}

	// ListJobRuns: should return all four, newest first.
	runs, err := d.ListJobRuns(ctx, job.ID, 0)
	if err != nil {
		t.Fatalf("ListJobRuns: %v", err)
	}
	if len(runs) != 4 {
		t.Fatalf("ListJobRuns: got %d runs, want 4", len(runs))
	}
	if runs[len(runs)-1].ID != older.ID {
		t.Errorf("ListJobRuns ordering: oldest at position %d should be run %d, got %d",
			len(runs)-1, older.ID, runs[len(runs)-1].ID)
	}
}

// TestLatestJobRun_NoRuns verifies that LatestJobRun returns (nil, nil) when a
// job has no history yet — Step 4 ETA code relies on this nil-not-error contract.
func TestLatestJobRun_NoRuns(t *testing.T) {
	d := newJobTestDB(t)
	ctx := context.Background()

	job, err := d.CreateJob(ctx, db.CreateJobParams{
		Name:            "fresh-job",
		SourcePath:      t.TempDir(),
		DestinationPath: t.TempDir(),
		Mode:            "one-way-backup",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	latest, err := d.LatestJobRun(ctx, job.ID)
	if err != nil {
		t.Fatalf("LatestJobRun: %v", err)
	}
	if latest != nil {
		t.Errorf("LatestJobRun: got %+v, want nil for job with no history", latest)
	}
}

func mustCreateRun(t *testing.T, d *db.DB, ctx context.Context, p db.CreateJobRunParams) *db.JobRun {
	t.Helper()
	run, err := d.CreateJobRun(ctx, p)
	if err != nil {
		t.Fatalf("CreateJobRun: %v", err)
	}
	return run
}
