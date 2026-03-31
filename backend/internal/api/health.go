package api

import (
	"net/http"
	"time"
)

const Version = "0.1.0"

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
