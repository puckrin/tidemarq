package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tidemarq/tidemarq/internal/conflicts"
	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/ws"
)

func (s *Server) handleListConflicts(w http.ResponseWriter, r *http.Request) {
	jobID, ok := parseOptionalJobID(w, r)
	if !ok {
		return
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

	validActions := map[string]bool{"keep-source": true, "keep-dest": true, "keep-both": true}
	if !validActions[req.Action] {
		writeError(w, http.StatusBadRequest, "action must be keep-source, keep-dest, or keep-both", "bad_request")
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

	if err := s.conflictsSvc.Resolve(r.Context(), id, req.Action, filepath.Join(job.SourcePath, c.RelPath), filepath.Join(job.DestinationPath, c.RelPath)); err != nil {
		if errors.Is(err, conflicts.ErrAlreadyResolved) {
			writeError(w, http.StatusConflict, "conflict is already resolved", "conflict")
			return
		}
		log.Printf("resolve conflict %d: %v", id, err)
		writeError(w, http.StatusInternalServerError, "failed to resolve conflict", "internal_error")
		return
	}

	s.hub.Broadcast(ws.Event{JobID: c.JobID, Event: "conflict_resolved"})
	w.WriteHeader(http.StatusNoContent)
}

// handleClearResolvedConflicts handles POST /api/v1/conflicts/clear-resolved.
func (s *Server) handleClearResolvedConflicts(w http.ResponseWriter, r *http.Request) {
	jobID, ok := parseOptionalJobID(w, r)
	if !ok {
		return
	}
	if err := s.conflictsSvc.ClearResolved(r.Context(), jobID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear resolved conflicts", "internal_error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
