package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/tidemarq/tidemarq/internal/db"
)

func (s *Server) handleListAuditLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	f := db.AuditFilter{}

	if v := q.Get("job_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid job_id", "bad_request")
			return
		}
		f.JobID = &id
	}

	if v := q.Get("event"); v != "" {
		f.Event = v
	}

	if v := q.Get("since"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid since (RFC3339 expected)", "bad_request")
			return
		}
		f.Since = &t
	}

	if v := q.Get("until"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid until (RFC3339 expected)", "bad_request")
			return
		}
		f.Until = &t
	}

	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "invalid limit", "bad_request")
			return
		}
		f.Limit = n
	}

	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset", "bad_request")
			return
		}
		f.Offset = n
	}

	entries, err := s.auditSvc.Query(r.Context(), f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query audit log", "internal_error")
		return
	}
	if entries == nil {
		entries = []*db.AuditEntry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleExportAuditLog(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	f := db.AuditFilter{Limit: 100_000}

	switch format {
	case "csv":
		data, err := s.auditSvc.ExportCSV(r.Context(), f)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "export failed", "internal_error")
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", `attachment; filename="audit_log.csv"`)
		w.WriteHeader(http.StatusOK)
		w.Write(data) //nolint:errcheck

	case "json":
		data, err := s.auditSvc.ExportJSON(r.Context(), f)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "export failed", "internal_error")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="audit_log.json"`)
		w.WriteHeader(http.StatusOK)
		w.Write(data) //nolint:errcheck

	default:
		writeError(w, http.StatusBadRequest, "format must be csv or json", "bad_request")
	}
}
