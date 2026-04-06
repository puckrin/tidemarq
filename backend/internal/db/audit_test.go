package db_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/migrations"
)

func newAuditTestDB(t *testing.T) *db.DB {
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

func seedAuditJob(t *testing.T, d *db.DB, name string) int64 {
	t.Helper()
	j, err := d.CreateJob(context.Background(), db.CreateJobParams{
		Name:            name,
		SourcePath:      t.TempDir(),
		DestinationPath: t.TempDir(),
		Mode:            "one-way-backup",
	})
	if err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	return j.ID
}

func insertEntry(t *testing.T, d *db.DB, jobID *int64, event, message string) *db.AuditEntry {
	t.Helper()
	e, err := d.CreateAuditEntry(context.Background(), db.CreateAuditEntryParams{
		JobID:   jobID,
		JobName: "test-job",
		Actor:   "system",
		Event:   event,
		Message: message,
	})
	if err != nil {
		t.Fatalf("CreateAuditEntry: %v", err)
	}
	return e
}

// TestListAuditEntries_NoFilter verifies that all entries are returned with the
// highest-ID (most recently inserted) entry first. The secondary ORDER BY id DESC
// provides a stable tiebreaker when multiple entries share the same created_at
// timestamp (common when entries are inserted within the same second).
func TestListAuditEntries_NoFilter(t *testing.T) {
	d := newAuditTestDB(t)
	ctx := context.Background()

	jobID := seedAuditJob(t, d, "job-a")
	id := &jobID

	e1 := insertEntry(t, d, id, "job_started", "first")
	e2 := insertEntry(t, d, id, "job_completed", "second")
	e3 := insertEntry(t, d, id, "job_started", "third")

	entries, err := d.ListAuditEntries(ctx, db.AuditFilter{})
	if err != nil {
		t.Fatalf("ListAuditEntries: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	// Results must be ordered by (created_at DESC, id DESC).
	// IDs are guaranteed to be ascending by insertion order.
	if entries[0].ID != e3.ID {
		t.Errorf("expected entry id %d first (newest), got id %d", e3.ID, entries[0].ID)
	}
	if entries[1].ID != e2.ID {
		t.Errorf("expected entry id %d second, got id %d", e2.ID, entries[1].ID)
	}
	if entries[2].ID != e1.ID {
		t.Errorf("expected entry id %d last (oldest), got id %d", e1.ID, entries[2].ID)
	}
}

// TestListAuditEntries_FilterByJobID verifies that only entries for the given job
// are returned, ordered newest-first — the primary use of the composite index.
func TestListAuditEntries_FilterByJobID(t *testing.T) {
	d := newAuditTestDB(t)
	ctx := context.Background()

	jobA := seedAuditJob(t, d, "job-a")
	jobB := seedAuditJob(t, d, "job-b")
	idA, idB := &jobA, &jobB

	eA1 := insertEntry(t, d, idA, "job_started", "a-first")
	insertEntry(t, d, idB, "job_started", "b-first")
	eA2 := insertEntry(t, d, idA, "job_completed", "a-second")
	insertEntry(t, d, idB, "job_failed", "b-second")

	entries, err := d.ListAuditEntries(ctx, db.AuditFilter{JobID: idA})
	if err != nil {
		t.Fatalf("ListAuditEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries for job-a, got %d", len(entries))
	}
	for _, e := range entries {
		if e.JobID == nil || *e.JobID != jobA {
			t.Errorf("entry job_id %v is not job-a (%d)", e.JobID, jobA)
		}
	}
	// Highest ID first (newest inserted).
	if entries[0].ID != eA2.ID {
		t.Errorf("expected entry id %d first, got id %d", eA2.ID, entries[0].ID)
	}
	if entries[1].ID != eA1.ID {
		t.Errorf("expected entry id %d second, got id %d", eA1.ID, entries[1].ID)
	}
}

// TestListAuditEntries_FilterByEvent verifies event-type filtering.
func TestListAuditEntries_FilterByEvent(t *testing.T) {
	d := newAuditTestDB(t)
	ctx := context.Background()

	jobID := seedAuditJob(t, d, "job-a")
	id := &jobID

	insertEntry(t, d, id, "job_started", "s1")
	insertEntry(t, d, id, "job_failed", "f1")
	insertEntry(t, d, id, "job_started", "s2")
	insertEntry(t, d, id, "job_completed", "c1")

	entries, err := d.ListAuditEntries(ctx, db.AuditFilter{Event: "job_started"})
	if err != nil {
		t.Fatalf("ListAuditEntries: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 job_started entries, got %d", len(entries))
	}
	for _, e := range entries {
		if e.Event != "job_started" {
			t.Errorf("expected event job_started, got %q", e.Event)
		}
	}
}

// TestListAuditEntries_FilterBySince verifies the since (lower bound) date filter.
// SQLite CURRENT_TIMESTAMP has 1-second resolution, so we cannot reliably split
// entries inserted in the same second. Instead we test the boundary using a fixed
// time clearly before all inserts (expects all results) and clearly after (expects none).
func TestListAuditEntries_FilterBySince(t *testing.T) {
	d := newAuditTestDB(t)
	ctx := context.Background()

	jobID := seedAuditJob(t, d, "job-a")
	id := &jobID

	insertEntry(t, d, id, "job_started", "first")
	insertEntry(t, d, id, "job_completed", "second")

	// Since well before the inserts — should return both entries.
	past := time.Now().Add(-time.Hour)
	entries, err := d.ListAuditEntries(ctx, db.AuditFilter{Since: &past})
	if err != nil {
		t.Fatalf("ListAuditEntries (past since): %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries with past since, got %d", len(entries))
	}

	// Since in the future — should return nothing.
	future := time.Now().Add(time.Hour)
	entries, err = d.ListAuditEntries(ctx, db.AuditFilter{Since: &future})
	if err != nil {
		t.Fatalf("ListAuditEntries (future since): %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries with future since, got %d", len(entries))
	}
}

// TestListAuditEntries_Limit verifies that the limit parameter is respected.
func TestListAuditEntries_Limit(t *testing.T) {
	d := newAuditTestDB(t)
	ctx := context.Background()

	jobID := seedAuditJob(t, d, "job-a")
	id := &jobID

	for i := 0; i < 10; i++ {
		insertEntry(t, d, id, "job_started", "entry")
	}

	entries, err := d.ListAuditEntries(ctx, db.AuditFilter{Limit: 3})
	if err != nil {
		t.Fatalf("ListAuditEntries: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries (limit), got %d", len(entries))
	}
}

// TestListAuditEntries_JobIDAndEvent verifies that combining job_id and event
// filters works correctly — exercises the composite index path with an extra predicate.
func TestListAuditEntries_JobIDAndEvent(t *testing.T) {
	d := newAuditTestDB(t)
	ctx := context.Background()

	jobA := seedAuditJob(t, d, "job-a")
	jobB := seedAuditJob(t, d, "job-b")
	idA, idB := &jobA, &jobB

	insertEntry(t, d, idA, "job_started", "a-start")
	insertEntry(t, d, idA, "job_failed", "a-fail")
	insertEntry(t, d, idB, "job_started", "b-start")
	insertEntry(t, d, idB, "job_failed", "b-fail")

	entries, err := d.ListAuditEntries(ctx, db.AuditFilter{JobID: idA, Event: "job_failed"})
	if err != nil {
		t.Fatalf("ListAuditEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Message != "a-fail" {
		t.Errorf("expected 'a-fail', got %q", entries[0].Message)
	}
}
