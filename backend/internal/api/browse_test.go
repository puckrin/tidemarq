package api_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestBrowse_RequiresAuth(t *testing.T) {
	ts, _ := newTestServer(t)

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/browse", nil, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

// TestBrowse_LocalRelativePathTraversalRejected verifies that relative paths
// are rejected with 400. filepath.Clean leaves relative paths relative, so
// "../../etc" would resolve against the process CWD without this guard.
func TestBrowse_LocalRelativePathTraversalRejected(t *testing.T) {
	ts, token := newTestServer(t)

	cases := []string{
		"../../etc",
		"../etc/passwd",
		"foo/../../bar",
		"..",
		"../",
		"subdir/../..",
	}

	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			resp := doRequest(t, ts, http.MethodGet, "/api/v1/browse?path="+p, &token, nil)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("path %q: expected 400, got %d", p, resp.StatusCode)
			}
		})
	}
}

func TestBrowse_LocalAbsolutePath(t *testing.T) {
	ts, token := newTestServer(t)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/browse?path="+filepath.ToSlash(dir), &token, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		Path    string `json:"path"`
		Entries []struct {
			Name  string `json:"name"`
			IsDir bool   `json:"is_dir"`
		} `json:"entries"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	names := make(map[string]bool)
	for _, e := range result.Entries {
		names[e.Name] = true
	}
	if !names["hello.txt"] {
		t.Error("hello.txt missing from entries")
	}
	if !names["subdir"] {
		t.Error("subdir missing from entries")
	}
}

func TestBrowse_LocalNotFound(t *testing.T) {
	ts, token := newTestServer(t)

	// Construct an absolute path that cannot exist, using the OS path convention
	// so that filepath.IsAbs passes (on Windows a bare "/..." is not absolute).
	dir := t.TempDir()
	absPath := filepath.ToSlash(filepath.VolumeName(dir)) + "/nonexistent-tidemarq-test-xyz-99999"

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/browse?path="+absPath, &token, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestBrowse_LocalEmptyPath(t *testing.T) {
	ts, token := newTestServer(t)

	resp := doRequest(t, ts, http.MethodGet, "/api/v1/browse", &token, nil)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// TestBrowse_MountTraversalRejected verifies that traversal paths on a mount
// are rejected with 400 before the mount is opened. Path validation now runs
// before Open(), so we get 400 even when the mount ID does not exist.
func TestBrowse_MountTraversalRejected(t *testing.T) {
	ts, token, _, _ := newFullTestServer(t)

	cases := []string{
		"../../etc",
		"../secret",
		"foo/../../bar",
		"..",
		"../",
	}

	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			// mount_id=9999 does not exist, but the path check fires first.
			resp := doRequest(t, ts, http.MethodGet, "/api/v1/browse?mount_id=9999&path="+p, &token, nil)
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("path %q: expected 400, got %d", p, resp.StatusCode)
			}
		})
	}
}

// TestBrowse_MountValidRelativePath verifies that a valid relative path passes
// the traversal guard and does not produce a 400. The mounts service is nil in
// test servers so the request ends with a non-200 further down the stack, but
// the key invariant is that the traversal guard itself does not fire.
func TestBrowse_MountValidRelativePath(t *testing.T) {
	ts, token := newTestServer(t)

	// "docs/reports" is a normal relative path — no traversal.
	resp := doRequest(t, ts, http.MethodGet, "/api/v1/browse?mount_id=9999&path=docs/reports", &token, nil)
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusBadRequest {
		t.Fatal("valid relative path should not be rejected by traversal guard")
	}
}
