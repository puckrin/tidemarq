package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tidemarq/tidemarq/internal/api"
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

// newTestServer spins up a Server backed by a real SQLite database in a
// temporary directory. Returns the server and a seeded admin token.
func newTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := database.Migrate(migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret: "test-secret",
			JWTTTL:    time.Hour,
		},
		Admin: config.AdminConfig{
			Username: "admin",
			Password: "adminpass",
		},
	}

	// Seed admin user.
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
	jobsSvc := jobs.New(database, syncEngine, hub, watcher, nil, nil, nil, nil)
	srv := api.NewServer(cfg, database, authSvc, jobsSvc, hub, nil, nil, nil, nil)
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)

	// Obtain admin token via login.
	token := mustLogin(t, ts, "admin", "adminpass")
	return ts, token
}

func mustLogin(t *testing.T, ts *httptest.Server, username, password string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"username": username, "password": password})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/auth/login", nil, bytes.NewReader(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: expected 200, got %d", resp.StatusCode)
	}
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck
	return result["token"]
}

func doRequest(t *testing.T, ts *httptest.Server, method, path string, token *string, body io.Reader) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, body)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != nil {
		req.Header.Set("Authorization", "Bearer "+*token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	return resp
}

// --- /health ---

func TestHealth(t *testing.T) {
	ts, _ := newTestServer(t)

	resp := doRequest(t, ts, http.MethodGet, "/health", nil, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["version"] == nil {
		t.Error("missing version field")
	}
	if result["database"] != "ok" {
		t.Errorf("database: got %q, want ok", result["database"])
	}
	if result["uptime"] == nil {
		t.Error("missing uptime field")
	}
}

// --- /api/v1/auth/login ---

func TestLogin_ValidCredentials(t *testing.T) {
	ts, _ := newTestServer(t)

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "adminpass"})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/auth/login", nil, bytes.NewReader(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck
	if result["token"] == "" {
		t.Error("expected non-empty token")
	}
}

func TestLogin_InvalidPassword(t *testing.T) {
	ts, _ := newTestServer(t)

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "wrong"})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/auth/login", nil, bytes.NewReader(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLogin_UnknownUser(t *testing.T) {
	ts, _ := newTestServer(t)

	body, _ := json.Marshal(map[string]string{"username": "ghost", "password": "pass"})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/auth/login", nil, bytes.NewReader(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// --- JWT middleware ---

func TestProtectedEndpoint_NoToken(t *testing.T) {
	ts, _ := newTestServer(t)

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/users", nil, nil)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestProtectedEndpoint_TamperedToken(t *testing.T) {
	ts, _ := newTestServer(t)

	tampered := "Bearer eyJhbGciOiJIUzI1NiJ9.tampered.signature"
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/users", nil)
	req.Header.Set("Authorization", tampered)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// --- Role enforcement ---

func TestAdminEndpoint_OperatorToken(t *testing.T) {
	ts, adminToken := newTestServer(t)

	// Create an operator user.
	body, _ := json.Marshal(map[string]string{"username": "op1", "password": "oppass", "role": "operator"})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/users", &adminToken, bytes.NewReader(body))
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create operator: expected 201, got %d", resp.StatusCode)
	}

	opToken := mustLogin(t, ts, "op1", "oppass")

	// Operator should be forbidden from listing users.
	resp = doRequest(t, ts, http.MethodGet, "/api/v1/users", &opToken, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestAdminEndpoint_ViewerToken(t *testing.T) {
	ts, adminToken := newTestServer(t)

	body, _ := json.Marshal(map[string]string{"username": "viewer1", "password": "viewpass", "role": "viewer"})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/users", &adminToken, bytes.NewReader(body))
	resp.Body.Close()

	viewerToken := mustLogin(t, ts, "viewer1", "viewpass")

	resp = doRequest(t, ts, http.MethodGet, "/api/v1/users", &viewerToken, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

// --- User CRUD ---

func TestUserCRUD(t *testing.T) {
	ts, adminToken := newTestServer(t)

	// Create.
	body, _ := json.Marshal(map[string]string{"username": "bob", "password": "bobpass", "role": "operator"})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/users", &adminToken, bytes.NewReader(body))
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", resp.StatusCode)
	}

	// List — should contain admin + bob.
	resp = doRequest(t, ts, http.MethodGet, "/api/v1/users", &adminToken, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", resp.StatusCode)
	}
	var users []map[string]any
	json.NewDecoder(resp.Body).Decode(&users) //nolint:errcheck
	if len(users) != 2 {
		t.Fatalf("list: expected 2 users, got %d", len(users))
	}

	// Get bob by ID.
	var bobID float64
	for _, u := range users {
		if u["username"] == "bob" {
			bobID = u["id"].(float64)
		}
	}
	if bobID == 0 {
		t.Fatal("bob not found in user list")
	}

	// Ensure password_hash is not exposed.
	for _, u := range users {
		if _, ok := u["password_hash"]; ok {
			t.Error("password_hash must not be present in user response")
		}
	}

	// Update bob's role.
	updateBody, _ := json.Marshal(map[string]string{"role": "viewer"})
	path := "/api/v1/users/" + idStr(bobID)
	resp2 := doRequest(t, ts, http.MethodPut, path, &adminToken, bytes.NewReader(updateBody))
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("update: expected 200, got %d", resp2.StatusCode)
	}
	var updated map[string]any
	json.NewDecoder(resp2.Body).Decode(&updated) //nolint:errcheck
	if updated["role"] != "viewer" {
		t.Errorf("update: expected role viewer, got %v", updated["role"])
	}

	// Delete bob.
	resp3 := doRequest(t, ts, http.MethodDelete, path, &adminToken, nil)
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", resp3.StatusCode)
	}

	// Verify bob is gone.
	resp4 := doRequest(t, ts, http.MethodGet, path, &adminToken, nil)
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusNotFound {
		t.Fatalf("get deleted: expected 404, got %d", resp4.StatusCode)
	}
}

func TestCreateUser_DuplicateUsername(t *testing.T) {
	ts, adminToken := newTestServer(t)

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "pass", "role": "viewer"})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/users", &adminToken, bytes.NewReader(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}
}

func TestCreateUser_InvalidRole(t *testing.T) {
	ts, adminToken := newTestServer(t)

	body, _ := json.Marshal(map[string]string{"username": "newuser", "password": "pass", "role": "superuser"})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/users", &adminToken, bytes.NewReader(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func idStr(id float64) string {
	return fmt.Sprintf("%d", int(id))
}

// newFullTestServer wires up versions and conflicts services so job/conflict/
// version/quarantine endpoints work end-to-end. Returns the server, admin token,
// a source directory, and a destination directory.
func newFullTestServer(t *testing.T) (*httptest.Server, string, string, string) {
	t.Helper()

	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := database.Migrate(migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	src := t.TempDir()
	dst := t.TempDir()

	cfg := &config.Config{
		Auth: config.AuthConfig{JWTSecret: "test-secret", JWTTTL: time.Hour},
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

	versionsDir := filepath.Join(tmp, "versions")
	vSvc := versions.New(database, versionsDir)
	cSvc := conflicts.New(database)

	jobsSvc := jobs.New(database, syncEngine, hub, watcher, vSvc, cSvc, nil, nil)
	if err := jobsSvc.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(jobsSvc.Stop)

	srv := api.NewServer(cfg, database, authSvc, jobsSvc, hub, cSvc, vSvc, nil, nil)
	ts := httptest.NewServer(srv.Routes())
	t.Cleanup(ts.Close)

	token := mustLogin(t, ts, "admin", "adminpass")
	return ts, token, src, dst
}

// waitJobStatus polls until the job reaches wantStatus or the timeout expires.
func waitJobStatus(t *testing.T, ts *httptest.Server, token, jobID, wantStatus string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp := doRequest(t, ts, http.MethodGet, "/api/v1/jobs/"+jobID, &token, nil)
		var j map[string]any
		json.NewDecoder(resp.Body).Decode(&j) //nolint:errcheck
		resp.Body.Close()
		if j["status"] == wantStatus {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Errorf("timeout waiting for job %s to reach status %q", jobID, wantStatus)
}

// -------------------------------------------------------------------------
// Expired JWT
// -------------------------------------------------------------------------

func TestExpiredToken_Returns401(t *testing.T) {
	ts, _ := newTestServer(t)

	// Issue a token with 1ms TTL so it expires immediately.
	authSvc := auth.NewService("test-secret", time.Hour)
	expiredToken, err := authSvc.IssueTokenTTL(1, "admin", "admin", time.Millisecond)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	time.Sleep(5 * time.Millisecond)

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/users", &expiredToken, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token, got %d", resp.StatusCode)
	}
}

// -------------------------------------------------------------------------
// Job CRUD via HTTP API
// -------------------------------------------------------------------------

func TestJobAPI_CRUD(t *testing.T) {
	ts, token, src, dst := newFullTestServer(t)

	// Create.
	body, _ := json.Marshal(map[string]any{
		"name": "api-job", "source_path": src, "destination_path": dst,
		"mode": "one-way-backup",
	})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/jobs", &token, bytes.NewReader(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", resp.StatusCode)
	}
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created) //nolint:errcheck
	jobID := idStr(created["id"].(float64))

	// Get.
	resp2 := doRequest(t, ts, http.MethodGet, "/api/v1/jobs/"+jobID, &token, nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", resp2.StatusCode)
	}

	// List.
	resp3 := doRequest(t, ts, http.MethodGet, "/api/v1/jobs", &token, nil)
	defer resp3.Body.Close()
	var list []any
	json.NewDecoder(resp3.Body).Decode(&list) //nolint:errcheck
	if len(list) != 1 {
		t.Fatalf("list: expected 1 job, got %d", len(list))
	}

	// Update.
	upd, _ := json.Marshal(map[string]any{
		"name": "renamed-job", "source_path": src, "destination_path": dst,
		"mode": "one-way-backup",
	})
	resp4 := doRequest(t, ts, http.MethodPut, "/api/v1/jobs/"+jobID, &token, bytes.NewReader(upd))
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("update: expected 200, got %d", resp4.StatusCode)
	}
	var updated map[string]any
	json.NewDecoder(resp4.Body).Decode(&updated) //nolint:errcheck
	if updated["name"] != "renamed-job" {
		t.Errorf("name after update: got %v", updated["name"])
	}

	// Delete.
	resp5 := doRequest(t, ts, http.MethodDelete, "/api/v1/jobs/"+jobID, &token, nil)
	defer resp5.Body.Close()
	if resp5.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", resp5.StatusCode)
	}

	// Get deleted → 404.
	resp6 := doRequest(t, ts, http.MethodGet, "/api/v1/jobs/"+jobID, &token, nil)
	defer resp6.Body.Close()
	if resp6.StatusCode != http.StatusNotFound {
		t.Fatalf("get after delete: expected 404, got %d", resp6.StatusCode)
	}
}

func TestJobAPI_InvalidMode(t *testing.T) {
	ts, token, src, dst := newFullTestServer(t)
	body, _ := json.Marshal(map[string]any{
		"name": "bad", "source_path": src, "destination_path": dst, "mode": "teleport",
	})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/jobs", &token, bytes.NewReader(body))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// -------------------------------------------------------------------------
// Run / Pause / Resume via HTTP API
// -------------------------------------------------------------------------

func TestJobAPI_Run_CompletesSuccessfully(t *testing.T) {
	ts, token, src, dst := newFullTestServer(t)

	if err := os.WriteFile(filepath.Join(src, "hello.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{
		"name": "run-job", "source_path": src, "destination_path": dst, "mode": "one-way-backup",
	})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/jobs", &token, bytes.NewReader(body))
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created) //nolint:errcheck
	resp.Body.Close()
	jobID := idStr(created["id"].(float64))

	runResp := doRequest(t, ts, http.MethodPost, "/api/v1/jobs/"+jobID+"/run", &token, nil)
	runResp.Body.Close()
	if runResp.StatusCode != http.StatusAccepted {
		t.Fatalf("run: expected 202, got %d", runResp.StatusCode)
	}

	waitJobStatus(t, ts, token, jobID, "idle", 10*time.Second)

	if _, err := os.Stat(filepath.Join(dst, "hello.txt")); os.IsNotExist(err) {
		t.Error("hello.txt not found in destination after run")
	}
}

func TestJobAPI_Run_AlreadyRunning(t *testing.T) {
	ts, token, src, dst := newFullTestServer(t)

	// Large file to keep the job running.
	data := make([]byte, 512*1024)
	if err := os.WriteFile(filepath.Join(src, "big.bin"), data, 0644); err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(map[string]any{
		"name": "dup-run", "source_path": src, "destination_path": dst, "mode": "one-way-backup",
	})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/jobs", &token, bytes.NewReader(body))
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created) //nolint:errcheck
	resp.Body.Close()
	jobID := idStr(created["id"].(float64))

	doRequest(t, ts, http.MethodPost, "/api/v1/jobs/"+jobID+"/run", &token, nil).Body.Close()

	resp2 := doRequest(t, ts, http.MethodPost, "/api/v1/jobs/"+jobID+"/run", &token, nil)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp2.StatusCode)
	}

	waitJobStatus(t, ts, token, jobID, "idle", 10*time.Second)
}

func TestJobAPI_Pause_AndResume(t *testing.T) {
	ts, token, src, dst := newFullTestServer(t)

	// Enough files at a bandwidth limit to keep the job running.
	data := make([]byte, 32*1024)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("f%02d.bin", i)
		if err := os.WriteFile(filepath.Join(src, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	body, _ := json.Marshal(map[string]any{
		"name": "pause-job", "source_path": src, "destination_path": dst,
		"mode": "one-way-backup", "bandwidth_limit_kb": 64,
	})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/jobs", &token, bytes.NewReader(body))
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created) //nolint:errcheck
	resp.Body.Close()
	jobID := idStr(created["id"].(float64))

	doRequest(t, ts, http.MethodPost, "/api/v1/jobs/"+jobID+"/run", &token, nil).Body.Close()
	waitJobStatus(t, ts, token, jobID, "running", 5*time.Second)

	pauseResp := doRequest(t, ts, http.MethodPost, "/api/v1/jobs/"+jobID+"/pause", &token, nil)
	pauseResp.Body.Close()
	if pauseResp.StatusCode != http.StatusAccepted {
		t.Fatalf("pause: expected 202, got %d", pauseResp.StatusCode)
	}

	// Wait for paused or idle (if it finished before pause landed).
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp := doRequest(t, ts, http.MethodGet, "/api/v1/jobs/"+jobID, &token, nil)
		var j map[string]any
		json.NewDecoder(resp.Body).Decode(&j) //nolint:errcheck
		resp.Body.Close()
		s := j["status"].(string)
		if s == "paused" || s == "idle" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	resumeResp := doRequest(t, ts, http.MethodPost, "/api/v1/jobs/"+jobID+"/resume", &token, nil)
	resumeResp.Body.Close()
	if resumeResp.StatusCode != http.StatusAccepted {
		t.Fatalf("resume: expected 202, got %d", resumeResp.StatusCode)
	}

	waitJobStatus(t, ts, token, jobID, "idle", 15*time.Second)

	// All files must be present in destination.
	for i := 0; i < 10; i++ {
		p := filepath.Join(dst, fmt.Sprintf("f%02d.bin", i))
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("missing in destination: f%02d.bin", i)
		}
	}
}

// -------------------------------------------------------------------------
// Conflict API
// -------------------------------------------------------------------------

func TestConflictAPI_ListAndResolve(t *testing.T) {
	ts, token, src, dst := newFullTestServer(t)

	// Create a two-way job.
	body, _ := json.Marshal(map[string]any{
		"name": "conflict-job", "source_path": src, "destination_path": dst,
		"mode": "two-way", "conflict_strategy": "ask-user",
	})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/jobs", &token, bytes.NewReader(body))
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created) //nolint:errcheck
	resp.Body.Close()
	jobID := idStr(created["id"].(float64))

	// Initial sync to establish manifest baseline.
	if err := os.WriteFile(filepath.Join(src, "shared.txt"), []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}
	doRequest(t, ts, http.MethodPost, "/api/v1/jobs/"+jobID+"/run", &token, nil).Body.Close()
	waitJobStatus(t, ts, token, jobID, "idle", 10*time.Second)

	// Modify both sides to create a conflict.
	if err := os.WriteFile(filepath.Join(src, "shared.txt"), []byte("source edit"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "shared.txt"), []byte("dest edit"), 0644); err != nil {
		t.Fatal(err)
	}
	doRequest(t, ts, http.MethodPost, "/api/v1/jobs/"+jobID+"/run", &token, nil).Body.Close()
	waitJobStatus(t, ts, token, jobID, "idle", 10*time.Second)

	// List conflicts — should have one.
	listResp := doRequest(t, ts, http.MethodGet, "/api/v1/conflicts?job_id="+jobID, &token, nil)
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list conflicts: expected 200, got %d", listResp.StatusCode)
	}
	var conflictList []map[string]any
	json.NewDecoder(listResp.Body).Decode(&conflictList) //nolint:errcheck
	if len(conflictList) == 0 {
		t.Fatal("expected at least one conflict")
	}
	conflictID := idStr(conflictList[0]["id"].(float64))

	// Resolve it with keep-source.
	resolveBody, _ := json.Marshal(map[string]string{"action": "keep-source"})
	resolveResp := doRequest(t, ts, http.MethodPost, "/api/v1/conflicts/"+conflictID+"/resolve", &token, bytes.NewReader(resolveBody))
	defer resolveResp.Body.Close()
	if resolveResp.StatusCode != http.StatusNoContent {
		t.Fatalf("resolve: expected 204, got %d", resolveResp.StatusCode)
	}
}

// -------------------------------------------------------------------------
// Version API
// -------------------------------------------------------------------------

func TestVersionAPI_ListAndRestore(t *testing.T) {
	ts, token, src, dst := newFullTestServer(t)

	// Create a two-way job so overwrites trigger snapshots.
	body, _ := json.Marshal(map[string]any{
		"name": "version-job", "source_path": src, "destination_path": dst,
		"mode": "two-way", "conflict_strategy": "source-wins",
	})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/jobs", &token, bytes.NewReader(body))
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created) //nolint:errcheck
	resp.Body.Close()
	jobID := idStr(created["id"].(float64))

	// Initial sync.
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("version-1"), 0644); err != nil {
		t.Fatal(err)
	}
	doRequest(t, ts, http.MethodPost, "/api/v1/jobs/"+jobID+"/run", &token, nil).Body.Close()
	waitJobStatus(t, ts, token, jobID, "idle", 10*time.Second)

	// Overwrite on both sides to trigger a snapshot via source-wins.
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("version-2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "file.txt"), []byte("dest-edit"), 0644); err != nil {
		t.Fatal(err)
	}
	doRequest(t, ts, http.MethodPost, "/api/v1/jobs/"+jobID+"/run", &token, nil).Body.Close()
	waitJobStatus(t, ts, token, jobID, "idle", 10*time.Second)

	// List versions for file.txt.
	listResp := doRequest(t, ts, http.MethodGet, "/api/v1/versions?job_id="+jobID+"&path=file.txt", &token, nil)
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list versions: expected 200, got %d", listResp.StatusCode)
	}
	var vList []map[string]any
	json.NewDecoder(listResp.Body).Decode(&vList) //nolint:errcheck
	if len(vList) == 0 {
		t.Fatal("expected at least one version")
	}
	versionID := idStr(vList[0]["id"].(float64))

	// Restore the version.
	restoreResp := doRequest(t, ts, http.MethodPost, "/api/v1/versions/"+versionID+"/restore", &token, nil)
	defer restoreResp.Body.Close()
	if restoreResp.StatusCode != http.StatusNoContent {
		t.Fatalf("restore version: expected 204, got %d", restoreResp.StatusCode)
	}
}

// -------------------------------------------------------------------------
// Quarantine API
// -------------------------------------------------------------------------

func TestQuarantineAPI_ListAndRestore(t *testing.T) {
	ts, token, src, dst := newFullTestServer(t)

	// Create a mirror job so deleted source files are quarantined.
	body, _ := json.Marshal(map[string]any{
		"name": "quarantine-job", "source_path": src, "destination_path": dst,
		"mode": "one-way-mirror",
	})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/jobs", &token, bytes.NewReader(body))
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created) //nolint:errcheck
	resp.Body.Close()
	jobID := idStr(created["id"].(float64))

	// Initial sync with two files.
	if err := os.WriteFile(filepath.Join(src, "keep.txt"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "gone.txt"), []byte("gone"), 0644); err != nil {
		t.Fatal(err)
	}
	doRequest(t, ts, http.MethodPost, "/api/v1/jobs/"+jobID+"/run", &token, nil).Body.Close()
	waitJobStatus(t, ts, token, jobID, "idle", 10*time.Second)

	// Delete gone.txt from source and run again.
	if err := os.Remove(filepath.Join(src, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	doRequest(t, ts, http.MethodPost, "/api/v1/jobs/"+jobID+"/run", &token, nil).Body.Close()
	waitJobStatus(t, ts, token, jobID, "idle", 10*time.Second)

	// gone.txt should be absent from dest.
	if _, err := os.Stat(filepath.Join(dst, "gone.txt")); !os.IsNotExist(err) {
		t.Error("gone.txt should have been quarantined but still exists in dest")
	}

	// List quarantine — should have one entry.
	listResp := doRequest(t, ts, http.MethodGet, "/api/v1/quarantine?job_id="+jobID, &token, nil)
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list quarantine: expected 200, got %d", listResp.StatusCode)
	}
	var qList []map[string]any
	json.NewDecoder(listResp.Body).Decode(&qList) //nolint:errcheck
	if len(qList) == 0 {
		t.Fatal("expected at least one quarantine entry")
	}
	qID := idStr(qList[0]["id"].(float64))

	// Restore it.
	restoreResp := doRequest(t, ts, http.MethodPost, "/api/v1/quarantine/"+qID+"/restore", &token, nil)
	defer restoreResp.Body.Close()
	if restoreResp.StatusCode != http.StatusNoContent {
		t.Fatalf("restore quarantine: expected 204, got %d", restoreResp.StatusCode)
	}

	// gone.txt should be back in dest.
	if _, err := os.Stat(filepath.Join(dst, "gone.txt")); os.IsNotExist(err) {
		t.Error("gone.txt should be restored in dest but is missing")
	}
}
