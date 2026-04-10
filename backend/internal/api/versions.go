package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/versions"
)

func (s *Server) handleListVersions(w http.ResponseWriter, r *http.Request) {
	jobID, err := strconv.ParseInt(r.URL.Query().Get("job_id"), 10, 64)
	if err != nil || jobID == 0 {
		writeError(w, http.StatusBadRequest, "job_id is required", "bad_request")
		return
	}
	relPath := r.URL.Query().Get("path")
	if relPath == "" {
		writeError(w, http.StatusBadRequest, "path is required", "bad_request")
		return
	}

	list, err := s.versionsSvc.ListVersions(r.Context(), jobID, relPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list versions", "internal_error")
		return
	}
	if list == nil {
		list = []*db.FileVersion{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleRestoreVersion(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid version id", "bad_request")
		return
	}

	v, err := s.versionsSvc.GetVersion(r.Context(), id)
	if err != nil {
		if errors.Is(err, versions.ErrNotFound) {
			writeError(w, http.StatusNotFound, "version not found", "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get version", "internal_error")
		return
	}

	job, err := s.jobsSvc.Get(r.Context(), v.JobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get job", "internal_error")
		return
	}

	if err := s.versionsSvc.RestoreVersion(r.Context(), id, job.DestinationPath); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "internal_error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListQuarantine(w http.ResponseWriter, r *http.Request) {
	var jobID int64
	if q := r.URL.Query().Get("job_id"); q != "" {
		id, err := strconv.ParseInt(q, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid job_id", "bad_request")
			return
		}
		jobID = id
	}

	list, err := s.versionsSvc.ListQuarantine(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list quarantine", "internal_error")
		return
	}
	if list == nil {
		list = []*db.QuarantineEntry{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleListRemovedQuarantine(w http.ResponseWriter, r *http.Request) {
	var jobID int64
	if q := r.URL.Query().Get("job_id"); q != "" {
		id, err := strconv.ParseInt(q, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid job_id", "bad_request")
			return
		}
		jobID = id
	}

	list, err := s.versionsSvc.ListRemovedQuarantine(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list removed quarantine", "internal_error")
		return
	}
	if list == nil {
		list = []*db.QuarantineEntry{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleClearRemovedQuarantine(w http.ResponseWriter, r *http.Request) {
	var jobID int64
	if q := r.URL.Query().Get("job_id"); q != "" {
		id, err := strconv.ParseInt(q, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid job_id", "bad_request")
			return
		}
		jobID = id
	}
	if err := s.versionsSvc.ClearRemovedQuarantine(r.Context(), jobID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear removed quarantine", "internal_error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDeleteQuarantine(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid quarantine id", "bad_request")
		return
	}

	e, err := s.versionsSvc.GetQuarantineEntry(r.Context(), id)
	if err != nil {
		if errors.Is(err, versions.ErrNotFound) {
			writeError(w, http.StatusNotFound, "quarantine entry not found", "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get quarantine entry", "internal_error")
		return
	}

	job, err := s.jobsSvc.Get(r.Context(), e.JobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get job", "internal_error")
		return
	}

	if err := s.versionsSvc.DeleteQuarantine(r.Context(), id, job.DestinationPath); err != nil {
		if errors.Is(err, versions.ErrNotFound) {
			writeError(w, http.StatusNotFound, "quarantine entry not found", "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error(), "internal_error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRestoreQuarantine(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid quarantine id", "bad_request")
		return
	}

	e, err := s.versionsSvc.GetQuarantineEntry(r.Context(), id)
	if err != nil {
		if errors.Is(err, versions.ErrNotFound) {
			writeError(w, http.StatusNotFound, "quarantine entry not found", "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get quarantine entry", "internal_error")
		return
	}

	job, err := s.jobsSvc.Get(r.Context(), e.JobID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get job", "internal_error")
		return
	}

	if err := s.versionsSvc.RestoreQuarantine(r.Context(), id, job.DestinationPath); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "internal_error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
