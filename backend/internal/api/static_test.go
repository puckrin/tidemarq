package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// staticWriteFile is a test helper that writes content to path and registers cleanup.
func staticWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile %s: %v", path, err)
	}
}

func TestStaticHandler_HoldingPage_WhenDistAbsent(t *testing.T) {
	h := staticHandler(filepath.Join(t.TempDir(), "nonexistent-dist"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type: got %q, want text/html; charset=utf-8", ct)
	}
	if !strings.Contains(rec.Body.String(), "tidemarq") {
		t.Error("holding page missing tidemarq branding")
	}
}

func TestStaticHandler_ServesIndexHTML(t *testing.T) {
	dir := t.TempDir()
	staticWriteFile(t, filepath.Join(dir, "index.html"), "<html>app</html>")

	h := staticHandler(dir)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /: expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "app") {
		t.Error("index.html content not served at /")
	}
}

func TestStaticHandler_ServesNestedAsset(t *testing.T) {
	dir := t.TempDir()
	staticWriteFile(t, filepath.Join(dir, "index.html"), "<html>app</html>")
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	staticWriteFile(t, filepath.Join(dir, "assets", "app.js"), "console.log('hi')")

	h := staticHandler(dir)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /assets/app.js: expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "console.log") {
		t.Error("asset content not served")
	}
}

func TestStaticHandler_SPAFallback_UnknownPath(t *testing.T) {
	dir := t.TempDir()
	staticWriteFile(t, filepath.Join(dir, "index.html"), "<html>spa-root</html>")

	h := staticHandler(dir)

	spaPaths := []string{
		"/jobs/123",
		"/settings/general",
		"/conflicts",
		"/nonexistent-page",
	}
	for _, p := range spaPaths {
		t.Run(p, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), "spa-root") {
				t.Error("SPA fallback: index.html content not served")
			}
		})
	}
}

// TestStaticHandler_TraversalCannotEscapeRoot verifies that path traversal
// attempts cannot read files outside the dist root. http.FS(os.DirFS(dir))
// enforces fs.ValidPath on every Open call, which rejects any path element
// containing "..". A successful traversal would return "secret-content".
func TestStaticHandler_TraversalCannotEscapeRoot(t *testing.T) {
	outer := t.TempDir()
	distDir := filepath.Join(outer, "dist")
	if err := os.Mkdir(distDir, 0755); err != nil {
		t.Fatal(err)
	}
	staticWriteFile(t, filepath.Join(distDir, "index.html"), "<html>app</html>")
	// A sensitive file lives one level above the dist root.
	staticWriteFile(t, filepath.Join(outer, "secret.txt"), "secret-content")

	h := staticHandler(distDir)

	traversalPaths := []string{
		"/../secret.txt",
		"/../../secret.txt",
		"/../../../etc/passwd",
	}
	for _, p := range traversalPaths {
		t.Run(p, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
			body := rec.Body.String()
			if strings.Contains(body, "secret-content") {
				t.Errorf("path %q: traversal succeeded — secret content returned", p)
			}
		})
	}
}

// TestStaticHandler_TraversalViaURLEncoding verifies that percent-encoded
// traversal sequences are also handled safely.
func TestStaticHandler_TraversalViaURLEncoding(t *testing.T) {
	outer := t.TempDir()
	distDir := filepath.Join(outer, "dist")
	if err := os.Mkdir(distDir, 0755); err != nil {
		t.Fatal(err)
	}
	staticWriteFile(t, filepath.Join(distDir, "index.html"), "<html>app</html>")
	staticWriteFile(t, filepath.Join(outer, "secret.txt"), "secret-content")

	h := staticHandler(distDir)

	// %2F is '/', %2E is '.'. Go's net/http URL parsing decodes percent-encoding
	// before the handler sees r.URL.Path, so these are equivalent to the
	// unencoded forms and exercised the same code path.
	encodedPaths := []string{
		"/%2e%2e/secret.txt",
		"/%2e%2e%2fsecret.txt",
	}
	for _, p := range encodedPaths {
		t.Run(p, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
			if strings.Contains(rec.Body.String(), "secret-content") {
				t.Errorf("path %q: traversal succeeded via URL encoding", p)
			}
		})
	}
}

// TestSpaFS_InvalidPathFallsBackToIndex verifies the spaFS.Open behaviour
// directly: an ErrInvalid from fs.ValidPath (e.g. ".." component) triggers
// the index.html fallback, not an error response.
func TestSpaFS_InvalidPathFallsBackToIndex(t *testing.T) {
	dir := t.TempDir()
	staticWriteFile(t, filepath.Join(dir, "index.html"), "<html>fallback</html>")

	fsys := spaFS{base: http.FS(os.DirFS(dir))}

	// A path with ".." is rejected by fs.ValidPath with ErrInvalid.
	f, err := fsys.Open("/../secret")
	if err != nil {
		t.Fatalf("spaFS.Open: expected fallback to index.html, got error: %v", err)
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if !strings.Contains(string(buf[:n]), "fallback") {
		t.Errorf("expected index.html content, got: %q", string(buf[:n]))
	}
}

// TestSpaFS_MissingFileFallsBackToIndex verifies the ErrNotExist fallback.
func TestSpaFS_MissingFileFallsBackToIndex(t *testing.T) {
	dir := t.TempDir()
	staticWriteFile(t, filepath.Join(dir, "index.html"), "<html>fallback</html>")

	fsys := spaFS{base: http.FS(os.DirFS(dir))}

	f, err := fsys.Open("/does-not-exist.js")
	if err != nil {
		t.Fatalf("spaFS.Open: expected fallback, got error: %v", err)
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if !strings.Contains(string(buf[:n]), "fallback") {
		t.Errorf("expected index.html content, got: %q", string(buf[:n]))
	}
}
