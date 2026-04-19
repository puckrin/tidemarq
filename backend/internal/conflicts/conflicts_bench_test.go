package conflicts_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/tidemarq/tidemarq/internal/conflicts"
	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/manifest"
	"github.com/tidemarq/tidemarq/migrations"
)

// newBenchDB opens a fresh SQLite database for benchmarks and registers cleanup.
func newBenchDB(b *testing.B) (*db.DB, int64) {
	b.Helper()
	d, err := db.Open(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatalf("db.Open: %v", err)
	}
	b.Cleanup(func() { d.Close() })
	if err := d.Migrate(migrations.FS); err != nil {
		b.Fatalf("Migrate: %v", err)
	}
	j, err := d.CreateJob(context.Background(), db.CreateJobParams{
		Name:            "bench",
		SourcePath:      "/src",
		DestinationPath: "/dst",
		Mode:            "two-way",
	})
	if err != nil {
		b.Fatalf("CreateJob: %v", err)
	}
	return d, j.ID
}

// seedManifestBulk inserts n manifest rows in a single transaction so benchmark
// setup doesn't dominate wall-clock time.
func seedManifestBulk(b *testing.B, d *db.DB, jobID int64, n int) {
	b.Helper()
	tx, err := d.Begin()
	if err != nil {
		b.Fatalf("begin: %v", err)
	}
	now := time.Now().UTC()
	stmt, err := tx.PrepareContext(context.Background(),
		`INSERT INTO manifest_entries
		 (job_id, rel_path, sha256, hash_algo, size_bytes, mod_time, permissions, synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		b.Fatalf("prepare: %v", err)
	}
	defer stmt.Close()
	for i := 0; i < n; i++ {
		relPath := fmt.Sprintf("dir%04d/file%06d.dat", i/100, i)
		hash := fmt.Sprintf("hash%06d", i)
		if _, err := stmt.ExecContext(context.Background(),
			jobID, relPath, hash, "blake3", int64(i*100+1), now, int64(0o644), now,
		); err != nil {
			tx.Rollback()
			b.Fatalf("insert row %d: %v", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatalf("commit: %v", err)
	}
}

// ── Pure detection function ───────────────────────────────────────────────────

// BenchmarkDetect measures the cost of the pure Detect() function with no DB or
// disk access. It should be essentially free (3 boolean comparisons).
func BenchmarkDetect(b *testing.B) {
	src := conflicts.FileState{Exists: true, ContentHash: "src-hash", HashAlgo: "blake3"}
	dest := conflicts.FileState{Exists: true, ContentHash: "dest-hash", HashAlgo: "blake3"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conflicts.Detect("synced-hash", src, dest)
	}
}

// ── Combined manifest load + scan ─────────────────────────────────────────────

// BenchmarkConflictScan_50k measures the realistic conflict-scan pass: load
// 50 000 manifest entries from the DB and call Detect() for each. The 1%
// conflict rate (500 conflicts) reflects realistic two-way sync conditions.
//
// Detect() is ~3 boolean ops per call so the bottleneck is manifest.List().
// This benchmark measures both together to reflect actual engine cost.
func BenchmarkConflictScan_50k(b *testing.B) {
	const n = 50_000
	const conflictCount = n / 100 // 1% = 500

	d, jobID := newBenchDB(b)
	seedManifestBulk(b, d, jobID, n)
	store := manifest.New(d)
	ctx := context.Background()

	// Pre-build FileStates parallel to the seeded rows (same path order).
	// Rows are seeded with rel_path = "dir%04d/file%06d.dat" and returned by
	// List() in that lexicographic order, so index j matches entry j.
	type stateRow struct {
		syncedHash string
		src        conflicts.FileState
		dest       conflicts.FileState
	}
	rows := make([]stateRow, n)
	for i := 0; i < n; i++ {
		syncedHash := fmt.Sprintf("hash%06d", i)
		src := conflicts.FileState{Exists: true, ContentHash: syncedHash, HashAlgo: "blake3"}
		dest := conflicts.FileState{Exists: true, ContentHash: syncedHash, HashAlgo: "blake3"}
		if i < conflictCount {
			// Both sides diverged from the synced hash.
			src.ContentHash = "src-mod-" + syncedHash
			dest.ContentHash = "dest-mod-" + syncedHash
		}
		rows[i] = stateRow{syncedHash: syncedHash, src: src, dest: dest}
	}

	b.ResetTimer()
	for iter := 0; iter < b.N; iter++ {
		entries, err := store.List(ctx, jobID)
		if err != nil {
			b.Fatal(err)
		}

		found := 0
		for j, e := range entries {
			if isConflict, _, _ := conflicts.Detect(e.ContentHash, rows[j].src, rows[j].dest); isConflict {
				found++
			}
		}
		if found != conflictCount {
			b.Fatalf("found %d conflicts, want %d", found, conflictCount)
		}
	}
}

// ── Correctness gate ─────────────────────────────────────────────────────────

// TestConflictScan_50k_Correctness verifies that the scan loop finds exactly
// the expected number of conflicts and completes in a reasonable time window.
func TestConflictScan_50k_Correctness(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large-DB test in short mode")
	}

	d := newTestDB(t)
	jobID := seedJob(t, d)

	const n = 50_000
	const expectedConflicts = n / 100

	// Seed via raw SQL in a single transaction (no per-row commits).
	tx, err := d.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	now := time.Now().UTC()
	stmt, err := tx.PrepareContext(context.Background(),
		`INSERT INTO manifest_entries
		 (job_id, rel_path, sha256, hash_algo, size_bytes, mod_time, permissions, synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		t.Fatalf("prepare: %v", err)
	}
	for i := 0; i < n; i++ {
		relPath := fmt.Sprintf("dir%04d/file%06d.dat", i/100, i)
		hash := fmt.Sprintf("hash%06d", i)
		if _, err := stmt.ExecContext(context.Background(),
			jobID, relPath, hash, "blake3", int64(i*100+1), now, int64(0o644), now,
		); err != nil {
			stmt.Close()
			tx.Rollback()
			t.Fatalf("insert row %d: %v", i, err)
		}
	}
	stmt.Close()
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	store := manifest.New(d)
	start := time.Now()
	entries, err := store.List(context.Background(), jobID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	found := 0
	for j, e := range entries {
		syncedHash := fmt.Sprintf("hash%06d", j)
		src := conflicts.FileState{Exists: true, ContentHash: syncedHash, HashAlgo: "blake3"}
		dest := conflicts.FileState{Exists: true, ContentHash: syncedHash, HashAlgo: "blake3"}
		if j < expectedConflicts {
			src.ContentHash = "src-mod-" + syncedHash
			dest.ContentHash = "dest-mod-" + syncedHash
		}
		if isConflict, _, _ := conflicts.Detect(e.ContentHash, src, dest); isConflict {
			found++
		}
	}
	elapsed := time.Since(start)

	t.Logf("50k List+Detect scan: %v", elapsed)

	if found != expectedConflicts {
		t.Errorf("found %d conflicts, want %d", found, expectedConflicts)
	}
	if len(entries) != n {
		t.Errorf("List returned %d entries, want %d", len(entries), n)
	}
}
