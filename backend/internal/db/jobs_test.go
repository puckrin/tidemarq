package db_test

import (
	"context"
	"testing"

	"github.com/tidemarq/tidemarq/internal/db"
)

func newJobTestDB(t *testing.T) *db.DB {
	t.Helper()
	// Reuse the helper already defined in audit_test.go (same package).
	return newAuditTestDB(t)
}

// TestCreateJob_DeltaFieldsRoundtrip verifies that use_delta, delta_block_size, and
// delta_min_bytes survive a create → get roundtrip through SQLite.
func TestCreateJob_DeltaFieldsRoundtrip(t *testing.T) {
	d := newJobTestDB(t)
	ctx := context.Background()

	j, err := d.CreateJob(ctx, db.CreateJobParams{
		Name:            "delta-job",
		SourcePath:      t.TempDir(),
		DestinationPath: t.TempDir(),
		Mode:            "one-way-backup",
		UseDelta:        true,
		DeltaBlockSize:  4096,
		DeltaMinBytes:   131072,
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	got, err := d.GetJobByID(ctx, j.ID)
	if err != nil {
		t.Fatalf("GetJobByID: %v", err)
	}
	if !got.UseDelta {
		t.Error("UseDelta: got false, want true")
	}
	if got.DeltaBlockSize != 4096 {
		t.Errorf("DeltaBlockSize: got %d, want 4096", got.DeltaBlockSize)
	}
	if got.DeltaMinBytes != 131072 {
		t.Errorf("DeltaMinBytes: got %d, want 131072", got.DeltaMinBytes)
	}
}

// TestCreateJob_DeltaFieldsDefaultToZero verifies that omitting delta fields
// results in zero values, not an error.
func TestCreateJob_DeltaFieldsDefaultToZero(t *testing.T) {
	d := newJobTestDB(t)
	ctx := context.Background()

	j, err := d.CreateJob(ctx, db.CreateJobParams{
		Name:            "plain-job",
		SourcePath:      t.TempDir(),
		DestinationPath: t.TempDir(),
		Mode:            "one-way-backup",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	got, err := d.GetJobByID(ctx, j.ID)
	if err != nil {
		t.Fatalf("GetJobByID: %v", err)
	}
	if got.UseDelta {
		t.Error("UseDelta: got true, want false (default)")
	}
	if got.DeltaBlockSize != 0 {
		t.Errorf("DeltaBlockSize: got %d, want 0 (default)", got.DeltaBlockSize)
	}
	if got.DeltaMinBytes != 0 {
		t.Errorf("DeltaMinBytes: got %d, want 0 (default)", got.DeltaMinBytes)
	}
}

// TestUpdateJob_DeltaFields verifies that enabling delta settings via UpdateJob
// persists correctly.
func TestUpdateJob_DeltaFields(t *testing.T) {
	d := newJobTestDB(t)
	ctx := context.Background()

	j, err := d.CreateJob(ctx, db.CreateJobParams{
		Name:            "update-job",
		SourcePath:      t.TempDir(),
		DestinationPath: t.TempDir(),
		Mode:            "one-way-backup",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	updated, err := d.UpdateJob(ctx, j.ID, db.UpdateJobParams{
		Name:            j.Name,
		SourcePath:      j.SourcePath,
		DestinationPath: j.DestinationPath,
		Mode:            j.Mode,
		UseDelta:        true,
		DeltaBlockSize:  8192,
		DeltaMinBytes:   262144,
	})
	if err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}
	if !updated.UseDelta {
		t.Error("UseDelta: got false, want true after update")
	}
	if updated.DeltaBlockSize != 8192 {
		t.Errorf("DeltaBlockSize: got %d, want 8192", updated.DeltaBlockSize)
	}
	if updated.DeltaMinBytes != 262144 {
		t.Errorf("DeltaMinBytes: got %d, want 262144", updated.DeltaMinBytes)
	}
}

// TestUpdateJob_ClearDeltaFields verifies that delta settings can be disabled
// by setting them back to zero values via UpdateJob.
func TestUpdateJob_ClearDeltaFields(t *testing.T) {
	d := newJobTestDB(t)
	ctx := context.Background()

	j, err := d.CreateJob(ctx, db.CreateJobParams{
		Name:            "clear-delta-job",
		SourcePath:      t.TempDir(),
		DestinationPath: t.TempDir(),
		Mode:            "one-way-backup",
		UseDelta:        true,
		DeltaBlockSize:  4096,
		DeltaMinBytes:   65536,
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	updated, err := d.UpdateJob(ctx, j.ID, db.UpdateJobParams{
		Name:            j.Name,
		SourcePath:      j.SourcePath,
		DestinationPath: j.DestinationPath,
		Mode:            j.Mode,
		UseDelta:        false,
		DeltaBlockSize:  0,
		DeltaMinBytes:   0,
	})
	if err != nil {
		t.Fatalf("UpdateJob: %v", err)
	}
	if updated.UseDelta {
		t.Error("UseDelta: got true, want false after clearing")
	}
	if updated.DeltaBlockSize != 0 {
		t.Errorf("DeltaBlockSize: got %d, want 0 after clearing", updated.DeltaBlockSize)
	}
}

// TestListJobs_IncludesDeltaFields verifies that ListJobs returns the delta
// configuration columns for each job.
func TestListJobs_IncludesDeltaFields(t *testing.T) {
	d := newJobTestDB(t)
	ctx := context.Background()

	_, err := d.CreateJob(ctx, db.CreateJobParams{
		Name:            "list-delta-job",
		SourcePath:      t.TempDir(),
		DestinationPath: t.TempDir(),
		Mode:            "one-way-backup",
		UseDelta:        true,
		DeltaBlockSize:  2048,
		DeltaMinBytes:   65536,
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}

	list, err := d.ListJobs(ctx)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListJobs: got %d jobs, want 1", len(list))
	}
	if !list[0].UseDelta {
		t.Error("ListJobs: UseDelta = false, want true")
	}
	if list[0].DeltaBlockSize != 2048 {
		t.Errorf("ListJobs: DeltaBlockSize = %d, want 2048", list[0].DeltaBlockSize)
	}
	if list[0].DeltaMinBytes != 65536 {
		t.Errorf("ListJobs: DeltaMinBytes = %d, want 65536", list[0].DeltaMinBytes)
	}
}
