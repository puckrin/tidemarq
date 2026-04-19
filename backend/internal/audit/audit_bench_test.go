package audit_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tidemarq/tidemarq/internal/audit"
	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/migrations"
)

func newAuditBenchDB(b testing.TB) (*db.DB, int64) {
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
		Mode:            "one-way-backup",
	})
	if err != nil {
		b.Fatalf("CreateJob: %v", err)
	}
	return d, j.ID
}

func seedAuditLog(b testing.TB, d *db.DB, jobID int64, n int) {
	b.Helper()
	events := []string{"sync.start", "sync.complete", "file.copied", "file.conflict", "job.error"}
	tx, err := d.Begin()
	if err != nil {
		b.Fatalf("begin: %v", err)
	}
	stmt, err := tx.PrepareContext(context.Background(),
		`INSERT INTO audit_log (job_id, job_name, actor, event, message, detail)
		 VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		b.Fatalf("prepare: %v", err)
	}
	defer stmt.Close()
	for i := 0; i < n; i++ {
		event := events[i%len(events)]
		if _, err := stmt.ExecContext(context.Background(),
			jobID, "bench-job", "system", event,
			fmt.Sprintf("message %d", i), fmt.Sprintf("detail %d", i),
		); err != nil {
			tx.Rollback()
			b.Fatalf("insert row %d: %v", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		b.Fatalf("commit: %v", err)
	}
}

// ── Query benchmarks ──────────────────────────────────────────────────────────

// BenchmarkAuditList measures p99-representative query cost for the four filter
// combinations used by the API, against a 100 000-row audit log.
func BenchmarkAuditList(b *testing.B) {
	d, jobID := newAuditBenchDB(b)
	seedAuditLog(b, d, jobID, 100_000)
	svc := audit.New(d)
	ctx := context.Background()

	b.Run("no_filter", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := svc.Query(ctx, db.AuditFilter{}); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("by_job", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := svc.Query(ctx, db.AuditFilter{JobID: &jobID}); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("by_event", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := svc.Query(ctx, db.AuditFilter{Event: "file.copied"}); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("by_job_and_date_range", func(b *testing.B) {
		since := time.Now().Add(-24 * time.Hour)
		until := time.Now().Add(time.Hour)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := svc.Query(ctx, db.AuditFilter{
				JobID: &jobID, Since: &since, Until: &until,
			}); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// ── EXPLAIN QUERY PLAN ────────────────────────────────────────────────────────

// TestAuditList_ExplainQueryPlan prints the SQLite query plan for each filter
// combination and fails if a full table scan is detected on a frequently-queried
// path where an index already exists.
func TestAuditList_ExplainQueryPlan(t *testing.T) {
	d, jobID := newAuditBenchDB(t)

	cases := []struct {
		name  string
		query string
		args  []any
	}{
		{
			name:  "by_job",
			query: `SELECT id FROM audit_log WHERE job_id = ? ORDER BY created_at DESC, id DESC LIMIT 500`,
			args:  []any{jobID},
		},
		{
			name:  "by_event",
			query: `SELECT id FROM audit_log WHERE event = ? ORDER BY created_at DESC, id DESC LIMIT 500`,
			args:  []any{"sync.complete"},
		},
		{
			name:  "by_job_and_event",
			query: `SELECT id FROM audit_log WHERE job_id = ? AND event = ? ORDER BY created_at DESC, id DESC LIMIT 500`,
			args:  []any{jobID, "sync.complete"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			rows, err := d.QueryContext(context.Background(),
				"EXPLAIN QUERY PLAN "+tc.query, tc.args...)
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
				// SCAN without USING INDEX means a full heap scan — bad for 100k rows.
				if strings.Contains(detail, "SCAN") && !strings.Contains(detail, "USING INDEX") {
					t.Errorf("full table scan on audit_log for filter %q — check indexes", tc.name)
				}
			}
		})
	}
}
