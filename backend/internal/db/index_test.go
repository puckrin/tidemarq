package db_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/migrations"
)

func newIndexTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "idx.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if err := d.Migrate(migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return d
}

func explainPlan(t *testing.T, d *db.DB, query string, args ...any) []string {
	t.Helper()
	rows, err := d.QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query, args...)
	if err != nil {
		t.Fatalf("EXPLAIN: %v", err)
	}
	defer rows.Close()
	var plans []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			t.Fatal(err)
		}
		plans = append(plans, detail)
	}
	return plans
}

func assertNoFullScan(t *testing.T, plans []string, table string) {
	t.Helper()
	for _, p := range plans {
		t.Logf("plan: %s", p)
		if strings.Contains(p, "SCAN "+table) && !strings.Contains(p, "USING INDEX") {
			t.Errorf("full table scan on %s — missing index", table)
		}
	}
}

// TestIndex_ManifestEntries verifies idx_manifest_entries_job_path is used.
func TestIndex_ManifestEntries(t *testing.T) {
	d := newIndexTestDB(t)

	t.Run("get by job_id and rel_path", func(t *testing.T) {
		plans := explainPlan(t, d,
			`SELECT id, job_id, rel_path, sha256, size_bytes, mod_time, permissions, synced_at
			 FROM manifest_entries WHERE job_id = ? AND rel_path = ?`, 1, "some/path.dat")
		assertNoFullScan(t, plans, "manifest_entries")
	})

	t.Run("list by job_id", func(t *testing.T) {
		plans := explainPlan(t, d,
			`SELECT id, job_id, rel_path, sha256, size_bytes, mod_time, permissions, synced_at
			 FROM manifest_entries WHERE job_id = ? ORDER BY rel_path`, 1)
		assertNoFullScan(t, plans, "manifest_entries")
	})
}

// TestIndex_Conflicts verifies idx_conflicts_job_id is used for job-scoped queries.
func TestIndex_Conflicts(t *testing.T) {
	d := newIndexTestDB(t)

	t.Run("list by job_id", func(t *testing.T) {
		plans := explainPlan(t, d,
			`SELECT id, job_id, rel_path, status, resolution, conflict_path, created_at, resolved_at
			 FROM conflicts WHERE job_id = ? ORDER BY created_at DESC`, 1)
		assertNoFullScan(t, plans, "conflicts")
	})

	t.Run("pending check by job_id and rel_path", func(t *testing.T) {
		plans := explainPlan(t, d,
			`SELECT id, job_id, rel_path, status, resolution, conflict_path, created_at, resolved_at
			 FROM conflicts WHERE job_id = ? AND rel_path = ? AND status = 'pending'`, 1, "some/path.dat")
		assertNoFullScan(t, plans, "conflicts")
	})
}

// TestIndex_QuarantineEntries verifies idx_quarantine_job_status is used.
func TestIndex_QuarantineEntries(t *testing.T) {
	d := newIndexTestDB(t)

	t.Run("active by job_id", func(t *testing.T) {
		plans := explainPlan(t, d,
			`SELECT id, job_id, rel_path, quarantine_path, sha256, hash_algo, size_bytes, deleted_at, expires_at, status, removed_at
			 FROM quarantine_entries WHERE status = 'active' AND job_id = ? ORDER BY deleted_at DESC`, 1)
		assertNoFullScan(t, plans, "quarantine_entries")
	})

	t.Run("removed by job_id", func(t *testing.T) {
		plans := explainPlan(t, d,
			`SELECT id, job_id, rel_path, quarantine_path, sha256, hash_algo, size_bytes, deleted_at, expires_at, status, removed_at
			 FROM quarantine_entries WHERE status != 'active' AND job_id = ? ORDER BY removed_at DESC`, 1)
		assertNoFullScan(t, plans, "quarantine_entries")
	})
}
