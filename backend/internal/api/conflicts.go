package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tidemarq/tidemarq/internal/conflicts"
	"github.com/tidemarq/tidemarq/internal/db"
)

func (s *Server) handleListConflicts(w http.ResponseWriter, r *http.Request) {
	var jobID int64
	if q := r.URL.Query().Get("job_id"); q != "" {
		id, err := strconv.ParseInt(q, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid job_id", "bad_request")
			return
		}
		jobID = id
	}

	list, err := s.conflictsSvc.List(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list conflicts", "internal_error")
		return
	}
	if list == nil {
		list = []*db.Conflict{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetConflict(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conflict id", "bad_request")
		return
	}

	c, err := s.conflictsSvc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, conflicts.ErrNotFound) {
			writeError(w, http.StatusNotFound, "conflict not found", "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get conflict", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

type resolveConflictRequest struct {
	Action string `json:"action"` // keep-source | keep-dest | keep-both
}

func (s *Server) handleResolveConflict(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid conflict id", "bad_request")
		return
	}

	var req resolveConflictRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	c, err := s.conflictsSvc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, conflicts.ErrNotFound) {
			writeError(w, http.StatusNotFound, "conflict not found", "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get conflict", "internal_error")
		return
	}

	// Resolve the job's source/dest paths for this conflict's file.
	job, err := s.jobsSvc.Get(r.Context(), c.JobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get job", "internal_error")
		return
	}

	if err := s.conflictsSvc.Resolve(r.Context(), id, req.Action, job.SourcePath+"/"+c.RelPath, job.DestinationPath+"/"+c.RelPath); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "bad_request")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
