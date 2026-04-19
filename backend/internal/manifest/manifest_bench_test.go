package manifest_test

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/manifest"
	"github.com/tidemarq/tidemarq/migrations"
)

// newBenchDB opens a fresh SQLite database for benchmarks and registers cleanup.
func newBenchDB(b *testing.B) (*db.DB, int64) {
	b.Helper()
	d, err := db.Open(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatalf("open db: %v", err)
	}
	b.Cleanup(func() { d.Close() })
	if err := d.Migrate(migrations.FS); err != nil {
		b.Fatalf("migrate: %v", err)
	}
	job, err := d.CreateJob(context.Background(), db.CreateJobParams{
		Name:            "bench-job",
		SourcePath:      "/src",
		DestinationPath: "/dst",
		Mode:            "one-way-backup",
	})
	if err != nil {
		b.Fatalf("create job: %v", err)
	}
	return d, job.ID
}

// seedEntries bulk-inserts n manifest rows for jobID in a single transaction.
// Each row uses a predictable path and hash so benchmarks can target specific rows.
func seedEntries(b *testing.B, d *db.DB, jobID int64, n int) {
	b.Helper()
	tx, err := d.Begin()
	if err != nil {
		b.Fatalf("begin tx: %v", err)
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

// ── List ─────────────────────────────────────────────────────────────────────

// BenchmarkStore_List measures how long it takes to load the full manifest for
// a job. This is called once at the start of every sync run, so its cost scales
// directly with job size.
func BenchmarkStore_List(b *testing.B) {
	for _, n := range []int{1_000, 10_000, 100_000} {
		n := n
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			d, jobID := newBenchDB(b)
			seedEntries(b, d, jobID, n)
			store := manifest.New(d)
			ctx := context.Background()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				entries, err := store.List(ctx, jobID)
				if err != nil {
					b.Fatal(err)
				}
				if len(entries) != n {
					b.Fatalf("got %d entries, want %d", len(entries), n)
				}
			}
		})
	}
}

// ── Get ──────────────────────────────────────────────────────────────────────

// BenchmarkStore_Get measures point-lookup latency at different table sizes.
// The UNIQUE(job_id, rel_path) index should keep this O(log n).
func BenchmarkStore_Get(b *testing.B) {
	for _, n := range []int{1_000, 10_000, 100_000} {
		n := n
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			d, jobID := newBenchDB(b)
			seedEntries(b, d, jobID, n)
			store := manifest.New(d)
			// Look up the median entry to avoid best-case tree positioning.
			mid := n / 2
			target := fmt.Sprintf("dir%04d/file%06d.dat", mid/100, mid)
			ctx := context.Background()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := store.Get(ctx, jobID, target); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// ── Put ──────────────────────────────────────────────────────────────────────

// BenchmarkStore_Put measures the per-write cost (upsert) against a populated
// table. Each engine file transfer calls Put() once, so this is the per-file
// manifest overhead during a sync run.
func BenchmarkStore_Put(b *testing.B) {
	d, jobID := newBenchDB(b)
	seedEntries(b, d, jobID, 10_000)
	store := manifest.New(d)
	now := time.Now().UTC()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := store.Put(ctx, &manifest.Entry{
			JobID:       jobID,
			RelPath:     "bench/target.dat",
			ContentHash: fmt.Sprintf("hash%d", i),
			HashAlgo:    "blake3",
			SizeBytes:   1024,
			ModTime:     now,
			Permissions: fs.FileMode(0o644),
			SyncedAt:    now,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}
