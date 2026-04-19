package versions_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/versions"
	"github.com/tidemarq/tidemarq/migrations"
)

func newVersionsBenchDB(b testing.TB) (*versions.Service, *db.DB, int64) {
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
	svc := versions.New(d, b.TempDir())
	return svc, d, j.ID
}

// seedFileVersions inserts nFiles × versionsPerFile rows in a single transaction.
// Each file gets versionsPerFile version entries so the table has nFiles*versionsPerFile rows total.
func seedFileVersions(b testing.TB, d *db.DB, jobID int64, nFiles, versionsPerFile int) {
	b.Helper()
	tx, err := d.Begin()
	if err != nil {
		b.Fatalf("begin: %v", err)
	}
	stmt, err := tx.PrepareContext(context.Background(),
		`INSERT INTO file_versions (job_id, rel_path, version_num, stored_path, sha256, hash_algo, size_bytes, mod_time)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		b.Fatalf("prepare: %v", err)
	}
	defer stmt.Close()
	now := time.Now().UTC()
	for f := 0; f < nFiles; f++ {
		relPath := fmt.Sprintf("dir%04d/file%06d.dat", f/100, f)
		for v := 1; v <= versionsPerFile; v++ {
			storedPath := fmt.Sprintf("/versions/%d/%s/%d", jobID, relPath, v)
			hash := fmt.Sprintf("hash-f%d-v%d", f, v)
			if _, err := stmt.ExecContext(context.Background(),
				jobID, relPath, v, storedPath, hash, "blake3", int64(f*1000+v), now,
			); err != nil {
				tx.Rollback()
				b.Fatalf("insert file=%d ver=%d: %v", f, v, err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatalf("commit: %v", err)
	}
}

// seedQuarantineExpired inserts n active quarantine entries all with expires_at
// in the past so the retention sweep picks them all up.
func seedQuarantineExpired(b testing.TB, d *db.DB, jobID int64, n int) {
	b.Helper()
	tx, err := d.Begin()
	if err != nil {
		b.Fatalf("begin: %v", err)
	}
	stmt, err := tx.PrepareContext(context.Background(),
		`INSERT INTO quarantine_entries
		 (job_id, rel_path, quarantine_path, sha256, hash_algo, size_bytes, deleted_at, expires_at, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'active')`)
	if err != nil {
		tx.Rollback()
		b.Fatalf("prepare: %v", err)
	}
	defer stmt.Close()
	past := time.Now().UTC().Add(-24 * time.Hour)
	for i := 0; i < n; i++ {
		relPath := fmt.Sprintf("dir%04d/file%06d.dat", i/100, i)
		qPath := fmt.Sprintf("/quarantine/%d/%s", jobID, relPath)
		hash := fmt.Sprintf("hash%06d", i)
		if _, err := stmt.ExecContext(context.Background(),
			jobID, relPath, qPath, hash, "blake3", int64(i*100+1), past, past,
		); err != nil {
			tx.Rollback()
			b.Fatalf("insert row %d: %v", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatalf("commit: %v", err)
	}
}

// ── B7: file_versions query performance ──────────────────────────────────────

// BenchmarkListFileVersions measures ListFileVersions point-lookup cost at
// varying table sizes. Without an index on (job_id, rel_path) this degrades
// as a full table scan; with an index it stays O(log n).
func BenchmarkListFileVersions(b *testing.B) {
	for _, nFiles := range []int{500, 5_000, 50_000} {
		nFiles := nFiles
		b.Run(fmt.Sprintf("files=%d", nFiles), func(b *testing.B) {
			_, d, jobID := newVersionsBenchDB(b)
			seedFileVersions(b, d, jobID, nFiles, 10)
			// Look up the median file's version history.
			mid := nFiles / 2
			target := fmt.Sprintf("dir%04d/file%06d.dat", mid/100, mid)
			ctx := context.Background()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				rows, err := d.ListFileVersions(ctx, jobID, target)
				if err != nil {
					b.Fatal(err)
				}
				if len(rows) != 10 {
					b.Fatalf("got %d versions, want 10", len(rows))
				}
			}
		})
	}
}

// TestListFileVersions_ExplainQueryPlan prints the SQLite query plan and flags
// a full table scan. file_versions has no secondary index on (job_id, rel_path);
// this test will fail until one is added via migration.
func TestListFileVersions_ExplainQueryPlan(t *testing.T) {
	_, d, jobID := newVersionsBenchDB(t)

	query := `SELECT id, job_id, rel_path, version_num, stored_path, sha256, hash_algo, size_bytes, mod_time, created_at
	          FROM file_versions WHERE job_id = ? AND rel_path = ? ORDER BY version_num DESC`

	rows, err := d.QueryContext(context.Background(),
		"EXPLAIN QUERY PLAN "+query, jobID, "some/path.dat")
	if err != nil {
		t.Fatalf("EXPLAIN: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatal(err)
		}
		t.Logf("plan: %s", detail)
		if strings.Contains(detail, "SCAN") && !strings.Contains(detail, "USING INDEX") {
			t.Errorf("full table scan on file_versions — add CREATE INDEX idx_file_versions_job_path ON file_versions (job_id, rel_path)")
		}
	}
}

// ── B8: quarantine retention sweep ───────────────────────────────────────────

// BenchmarkExpireQuarantine measures the time for the retention sweep over
// 10 000 expired entries. It uses a single SQL DELETE (not a row-by-row loop).
func BenchmarkExpireQuarantine(b *testing.B) {
	const n = 10_000

	for i := 0; i < b.N; i++ {
		// Re-seed before each iteration so the sweep always has rows to delete.
		// Setup is excluded from timing via b.StopTimer/b.StartTimer.
		b.StopTimer()
		svc, d, jobID := newVersionsBenchDB(b)
		seedQuarantineExpired(b, d, jobID, n)
		b.StartTimer()

		if err := svc.ExpireQuarantine(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
}

// TestExpireQuarantine_Correctness seeds 10 000 expired + 100 still-active entries
// and confirms that only expired rows are removed, under 1 second.
func TestExpireQuarantine_Correctness(t *testing.T) {
	svc, d, jobID := newVersionsBenchDB(t)
	ctx := context.Background()

	const nExpired = 10_000
	const nActive = 100

	// Insert expired entries.
	seedQuarantineExpired(t, d, jobID, nExpired)

	// Insert 100 entries that expire in the future (should be kept).
	tx, err := d.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO quarantine_entries
		 (job_id, rel_path, quarantine_path, sha256, hash_algo, size_bytes, deleted_at, expires_at, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'active')`)
	if err != nil {
		tx.Rollback()
		t.Fatalf("prepare: %v", err)
	}
	future := time.Now().UTC().Add(30 * 24 * time.Hour)
	now := time.Now().UTC()
	for i := 0; i < nActive; i++ {
		relPath := fmt.Sprintf("active/file%04d.dat", i)
		if _, err := stmt.ExecContext(ctx,
			jobID, relPath, "/quarantine/active/"+relPath, "hash", "blake3", 1024, now, future,
		); err != nil {
			stmt.Close()
			tx.Rollback()
			t.Fatalf("insert active row %d: %v", i, err)
		}
	}
	stmt.Close()
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}

	start := time.Now()
	if err := svc.ExpireQuarantine(ctx); err != nil {
		t.Fatalf("ExpireQuarantine: %v", err)
	}
	elapsed := time.Since(start)
	t.Logf("ExpireQuarantine(10k expired): %v", elapsed)

	limit := time.Second
	if raceEnabled {
		limit = 10 * time.Second // race detector adds ~5-10x overhead
	}
	if elapsed > limit {
		t.Errorf("sweep took %v, want < %v", elapsed, limit)
	}

	// Only the nActive entries should remain.
	remaining, err := d.ListQuarantineEntries(ctx, jobID)
	if err != nil {
		t.Fatalf("ListQuarantineEntries: %v", err)
	}
	if len(remaining) != nActive {
		t.Errorf("remaining entries = %d, want %d", len(remaining), nActive)
	}
}
