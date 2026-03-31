package api

import (
	"net/http"
	"os"
	"path/filepath"
)

// staticHandler serves files from dir. For any path that doesn't exist it
// falls back to index.html (SPA client-side routing). If the dist directory
// itself is absent it returns a plain holding page.
func staticHandler(dir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No dist yet — holding page until Phase 5.
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(holdingPage)) //nolint:errcheck
			return
		}

		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// Fall back to index.html for SPA routing.
			path = filepath.Join(dir, "index.html")
		}
		http.ServeFile(w, r, path)
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
