package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tidemarq/tidemarq/internal/api"
	"github.com/tidemarq/tidemarq/internal/audit"
	"github.com/tidemarq/tidemarq/internal/auth"
	"github.com/tidemarq/tidemarq/internal/config"
	"github.com/tidemarq/tidemarq/internal/conflicts"
	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/engine"
	"github.com/tidemarq/tidemarq/internal/jobs"
	"github.com/tidemarq/tidemarq/internal/manifest"
	"github.com/tidemarq/tidemarq/internal/versions"
	"github.com/tidemarq/tidemarq/internal/watch"
	"github.com/tidemarq/tidemarq/internal/ws"
	"github.com/tidemarq/tidemarq/migrations"
)

// newAuditTestServer wires up all services including audit.
// Returns the server, admin token, source dir, and destination dir.
func newAuditTestServer(t *testing.T) (*httptest.Server, string, string, string) {
	t.Helper()

	tmp := t.TempDir()
	database, err := db.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := database.Migrate(migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	src, dst := t.TempDir(), t.TempDir()
	cfg := &config.Config{
		Auth:  config.AuthConfig{JWTSecret: "test-secret", JWTTTL: time.Hour},
		Admin: config.AdminConfig{Username: "admin", Password: "adminpass"},
	}

	hash, err := auth.HashPassword(cfg.Admin.Password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if _, err := database.CreateUser(context.Background(), "admin", hash, "admin"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	authSvc := auth.NewService(cfg.Auth.JWTSecret, cfg.Auth.JWTTTL)
	manifestStore := manifest.New(database)
	syncEngine := engine.New(manifestStore)
	hub := ws.New()
	watcher, err := watch.New()
	if err != nil {
		t.Fatalf("watch.New: %v", err)
	}
	t.Cleanup(watcher.Close)

	vSvc := versions.New(database, filepath.Join(tmp, "versions"))
	cSvc := conflicts.New(database)
	auditSvc := audit.New(database)

	jobsSvc := jobs.New(database, syncEngine, hub, watcher, vSvc, cSvc, nil, auditSvc)
	if err := jobsSvc.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(jobsSvc.Stop)

	srv := api.NewServer(cfg, database, authSvc, jobsSvc, hub, cSvc, vSvc, nil, auditSvc)
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)

	token := mustLogin(t, ts, "admin", "adminpass")
	return ts, token, src, dst
}

// ── Settings ─────────────────────────────────────────────────────────────────

func TestSettings_Get(t *testing.T) {
	ts, token := newTestServer(t)

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/settings", &token, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["versions_to_keep"] == nil {
		t.Error("missing versions_to_keep field")
	}
	if result["quarantine_retention_days"] == nil {
		t.Error("missing quarantine_retention_days field")
	}
}

func TestSettings_Get_RequiresAuth(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := doRequest(t, ts, http.MethodGet, "/api/v1/settings", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestSettings_Update(t *testing.T) {
	ts, token := newTestServer(t)

	body, _ := json.Marshal(map[string]any{
		"versions_to_keep":          5,
		"quarantine_retention_days": 14,
	})
	resp := doRequest(t, ts, http.MethodPut, "/api/v1/settings", &token, bytes.NewReader(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["versions_to_keep"] != float64(5) {
		t.Errorf("versions_to_keep = %v, want 5", result["versions_to_keep"])
	}
	if result["quarantine_retention_days"] != float64(14) {
		t.Errorf("quarantine_retention_days = %v, want 14", result["quarantine_retention_days"])
	}
}

func TestSettings_Update_InvalidBody(t *testing.T) {
	ts, token := newTestServer(t)

	// quarantine_retention_days = 0 is invalid (must be >= 1).
	body, _ := json.Marshal(map[string]any{
		"versions_to_keep":          3,
		"quarantine_retention_days": 0,
	})
	resp := doRequest(t, ts, http.MethodPut, "/api/v1/settings", &token, bytes.NewReader(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSettings_Update_RequiresAuth(t *testing.T) {
	ts, _ := newTestServer(t)
	body, _ := json.Marshal(map[string]any{"versions_to_keep": 3, "quarantine_retention_days": 7})
	resp := doRequest(t, ts, http.MethodPut, "/api/v1/settings", nil, bytes.NewReader(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// ── Audit log ─────────────────────────────────────────────────────────────────

func TestAuditLog_List(t *testing.T) {
	ts, token, _, _ := newAuditTestServer(t)

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/audit", &token, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result []any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func TestAuditLog_List_InvalidJobID(t *testing.T) {
	ts, token, _, _ := newAuditTestServer(t)

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/audit?job_id=notanumber", &token, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAuditLog_List_InvalidSince(t *testing.T) {
	ts, token, _, _ := newAuditTestServer(t)

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/audit?since=notadate", &token, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAuditLog_List_RequiresAuth(t *testing.T) {
	ts, _, _, _ := newAuditTestServer(t)
	resp := doRequest(t, ts, http.MethodGet, "/api/v1/audit", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuditLog_Export_JSON(t *testing.T) {
	ts, token, _, _ := newAuditTestServer(t)

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/audit/export?format=json", &token, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "audit_log.json") {
		t.Errorf("Content-Disposition = %q, want attachment with audit_log.json", cd)
	}
}

func TestAuditLog_Export_CSV(t *testing.T) {
	ts, token, _, _ := newAuditTestServer(t)

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/audit/export?format=csv", &token, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/csv" {
		t.Errorf("Content-Type = %q, want text/csv", ct)
	}
}

func TestAuditLog_Export_InvalidFormat(t *testing.T) {
	ts, token, _, _ := newAuditTestServer(t)
	resp := doRequest(t, ts, http.MethodGet, "/api/v1/audit/export?format=xml", &token, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAuditLog_Export_RequiresAuth(t *testing.T) {
	ts, _, _, _ := newAuditTestServer(t)
	resp := doRequest(t, ts, http.MethodGet, "/api/v1/audit/export", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// ── Conflicts clear-resolved ──────────────────────────────────────────────────

func TestConflicts_ClearResolved(t *testing.T) {
	ts, token, _, _ := newFullTestServer(t)

	resp := doRequest(t, ts, http.MethodPost, "/api/v1/conflicts/clear-resolved", &token, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestConflicts_ClearResolved_RequiresAuth(t *testing.T) {
	ts, _, _, _ := newFullTestServer(t)
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/conflicts/clear-resolved", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// ── Quarantine: removed list, delete, clear-removed ──────────────────────────

func TestQuarantine_ListRemoved(t *testing.T) {
	ts, token, _, _ := newFullTestServer(t)

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/quarantine/removed", &token, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result []any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func TestQuarantine_ListRemoved_RequiresAuth(t *testing.T) {
	ts, _, _, _ := newFullTestServer(t)
	resp := doRequest(t, ts, http.MethodGet, "/api/v1/quarantine/removed", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestQuarantine_Delete_NotFound(t *testing.T) {
	ts, token, _, _ := newFullTestServer(t)

	resp := doRequest(t, ts, http.MethodDelete, "/api/v1/quarantine/9999", &token, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestQuarantine_Delete_InvalidID(t *testing.T) {
	ts, token, _, _ := newFullTestServer(t)

	resp := doRequest(t, ts, http.MethodDelete, "/api/v1/quarantine/notanid", &token, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestQuarantine_Delete_RequiresAuth(t *testing.T) {
	ts, _, _, _ := newFullTestServer(t)
	resp := doRequest(t, ts, http.MethodDelete, "/api/v1/quarantine/1", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestQuarantine_ClearRemoved(t *testing.T) {
	ts, token, _, _ := newFullTestServer(t)

	resp := doRequest(t, ts, http.MethodPost, "/api/v1/quarantine/clear-removed", &token, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

func TestQuarantine_ClearRemoved_RequiresAuth(t *testing.T) {
	ts, _, _, _ := newFullTestServer(t)
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/quarantine/clear-removed", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// ── Quarantine delete: end-to-end with a real entry ──────────────────────────

// TestQuarantine_Delete_EndToEnd runs a mirror job, deletes the source file
// (triggering quarantine), then calls DELETE /api/v1/quarantine/{id} and
// confirms the response is 204 and the entry no longer appears in the active list.
func TestQuarantine_Delete_EndToEnd(t *testing.T) {
	ts, token, src, dst := newFullTestServer(t)

	// Create a file and run a mirror job to establish baseline.
	if err := writeFile(t, src, "todelete.txt", "content"); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]any{
		"name": "mirror-job", "source_path": src, "destination_path": dst, "mode": "one-way-mirror",
	})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/jobs", &token, bytes.NewReader(body))
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created) //nolint:errcheck
	resp.Body.Close()
	jobID := idStr(created["id"].(float64))

	doRequest(t, ts, http.MethodPost, "/api/v1/jobs/"+jobID+"/run", &token, nil).Body.Close()
	waitJobStatus(t, ts, token, jobID, "idle", 10*time.Second)

	// Remove the source file so the next run quarantines the destination copy.
	if err := removeFile(src, "todelete.txt"); err != nil {
		t.Fatal(err)
	}
	doRequest(t, ts, http.MethodPost, "/api/v1/jobs/"+jobID+"/run", &token, nil).Body.Close()
	waitJobStatus(t, ts, token, jobID, "idle", 10*time.Second)

	// Confirm an active quarantine entry exists.
	qResp := doRequest(t, ts, http.MethodGet, "/api/v1/quarantine?job_id="+jobID, &token, nil)
	var qList []map[string]any
	json.NewDecoder(qResp.Body).Decode(&qList) //nolint:errcheck
	qResp.Body.Close()
	if len(qList) == 0 {
		t.Fatal("expected at least one quarantine entry after mirror deletion")
	}
	entryID := fmt.Sprintf("%.0f", qList[0]["id"].(float64))

	// DELETE the quarantine entry.
	delResp := doRequest(t, ts, http.MethodDelete, "/api/v1/quarantine/"+entryID, &token, nil)
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE quarantine: expected 204, got %d", delResp.StatusCode)
	}

	// Active list must now be empty.
	qResp2 := doRequest(t, ts, http.MethodGet, "/api/v1/quarantine?job_id="+jobID, &token, nil)
	var qList2 []map[string]any
	json.NewDecoder(qResp2.Body).Decode(&qList2) //nolint:errcheck
	qResp2.Body.Close()
	if len(qList2) != 0 {
		t.Errorf("expected empty active quarantine list after delete, got %d entries", len(qList2))
	}

	// Removed list must contain the deleted entry.
	rResp := doRequest(t, ts, http.MethodGet, "/api/v1/quarantine/removed?job_id="+jobID, &token, nil)
	var rList []map[string]any
	json.NewDecoder(rResp.Body).Decode(&rList) //nolint:errcheck
	rResp.Body.Close()
	if len(rList) == 0 {
		t.Error("expected deleted entry in removed quarantine list")
	}
}

// helpers used only in this file

func writeFile(t *testing.T, dir, name, content string) error {
	t.Helper()
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}

func removeFile(dir, name string) error {
	return os.Remove(filepath.Join(dir, name))
}
