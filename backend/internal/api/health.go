package api

import (
	"net/http"
	"time"
)

// Version is set at build time via -ldflags "-X github.com/tidemarq/tidemarq/internal/api.Version=x.y.z".
// Falls back to "dev" when built without ldflags (local development).
var Version = "dev"

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	dbStatus := "ok"
	if err := s.db.Ping(); err != nil {
		dbStatus = "error"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"version":  Version,
		"database": dbStatus,
		"uptime":   time.Since(s.startTime).String(),
	})
}
