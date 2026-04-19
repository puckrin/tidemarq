package api

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
)

// spaFS wraps an http.FileSystem to serve index.html for any path that is
// missing or structurally invalid (e.g. rejected by fs.ValidPath because it
// contains a ".." component). This lets the React router handle client-side
// navigation without requiring matching server-side routes.
type spaFS struct {
	base http.FileSystem
}

func (s spaFS) Open(name string) (http.File, error) {
	f, err := s.base.Open(name)
	if err == nil {
		return f, nil
	}
	// ErrNotExist  → legitimate missing file (SPA client-side route).
	// ErrInvalid   → path rejected by fs.ValidPath (e.g. contains "..").
	// Both fall back to index.html; all other errors propagate.
	if os.IsNotExist(err) || errors.Is(err, fs.ErrInvalid) {
		return s.base.Open("/index.html")
	}
	return nil, err
}

// staticHandler serves the pre-built frontend from dir. Unknown paths fall back
// to index.html for SPA client-side routing. If the dist directory is absent a
// holding page is returned.
//
// Path safety: http.FS(os.DirFS(dir)) passes every request path through
// fs.ValidPath, which rejects any element that is or contains "..". No manual
// filepath.Join / filepath.Clean is required or performed.
func staticHandler(dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(holdingPage)) //nolint:errcheck
			return
		}
		// os.DirFS roots all operations at dir; http.FS enforces fs.ValidPath
		// on every Open call — traversal via ".." is rejected at the FS layer.
		http.FileServer(spaFS{base: http.FS(os.DirFS(dir))}).ServeHTTP(w, r)
	})
}

const holdingPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>tidemarq</title>
  <style>
    body { font-family: system-ui, sans-serif; background: #0f1117; color: #e2e8f0;
           display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
    .card { text-align: center; }
    h1 { font-size: 1.5rem; margin: 0 0 .5rem; }
    p  { color: #94a3b8; margin: 0; font-size: .9rem; }
    code { font-family: "Courier New", monospace; background: #1e2432;
           padding: .15em .4em; border-radius: 4px; font-size: .85rem; }
  </style>
</head>
<body>
  <div class="card">
    <h1>tidemarq</h1>
    <p>API is running. Frontend ships in Phase 5.</p>
    <p style="margin-top:.75rem"><code>GET /health</code> &nbsp;·&nbsp; <code>POST /api/v1/auth/login</code></p>
  </div>
</body>
</html>
`
