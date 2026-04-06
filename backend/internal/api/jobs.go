package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/jobs"
)

type createJobRequest struct {
	Name             string `json:"name"`
	SourcePath       string `json:"source_path"`
	DestinationPath  string `json:"destination_path"`
	SourceMountID    *int64 `json:"source_mount_id,omitempty"`
	DestMountID      *int64 `json:"dest_mount_id,omitempty"`
	Mode             string `json:"mode"`
	BandwidthLimitKB int64  `json:"bandwidth_limit_kb"`
	ConflictStrategy string `json:"conflict_strategy"`
	CronSchedule     string `json:"cron_schedule"`
	WatchEnabled     bool   `json:"watch_enabled"`
	FullChecksum     bool   `json:"full_checksum"`
}

type updateJobRequest struct {
	Name             string `json:"name"`
	SourcePath       string `json:"source_path"`
	DestinationPath  string `json:"destination_path"`
	SourceMountID    *int64 `json:"source_mount_id,omitempty"`
	DestMountID      *int64 `json:"dest_mount_id,omitempty"`
	Mode             string `json:"mode"`
	BandwidthLimitKB int64  `json:"bandwidth_limit_kb"`
	ConflictStrategy string `json:"conflict_strategy"`
	CronSchedule     string `json:"cron_schedule"`
	WatchEnabled     bool   `json:"watch_enabled"`
	FullChecksum     bool   `json:"full_checksum"`
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	list, err := s.jobsSvc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list jobs", "internal_error")
		return
	}
	if list == nil {
		list = []*db.Job{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req createJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	job, err := s.jobsSvc.Create(r.Context(), jobs.CreateParams{
		Name:             req.Name,
		SourcePath:       req.SourcePath,
		DestinationPath:  req.DestinationPath,
		SourceMountID:    req.SourceMountID,
		DestMountID:      req.DestMountID,
		Mode:             req.Mode,
		BandwidthLimitKB: req.BandwidthLimitKB,
		ConflictStrategy: req.ConflictStrategy,
		CronSchedule:     req.CronSchedule,
		WatchEnabled:     req.WatchEnabled,
		FullChecksum:     req.FullChecksum,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "bad_request")
		return
	}

	writeJSON(w, http.StatusCreated, job)
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id", "bad_request")
		return
	}

	job, err := s.jobsSvc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, jobs.ErrNotFound) {
			writeError(w, http.StatusNotFound, "job not found", "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get job", "internal_error")
		return
	}

	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleUpdateJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id", "bad_request")
		return
	}

	var req updateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	job, err := s.jobsSvc.Update(r.Context(), id, jobs.UpdateParams{
		Name:             req.Name,
		SourcePath:       req.SourcePath,
		DestinationPath:  req.DestinationPath,
		SourceMountID:    req.SourceMountID,
		DestMountID:      req.DestMountID,
		Mode:             req.Mode,
		BandwidthLimitKB: req.BandwidthLimitKB,
		ConflictStrategy: req.ConflictStrategy,
		CronSchedule:     req.CronSchedule,
		WatchEnabled:     req.WatchEnabled,
		FullChecksum:     req.FullChecksum,
	})
	if err != nil {
		if errors.Is(err, jobs.ErrNotFound) {
			writeError(w, http.StatusNotFound, "job not found", "not_found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error(), "bad_request")
		return
	}

	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id", "bad_request")
		return
	}

	if err := s.jobsSvc.Delete(r.Context(), id); err != nil {
		if errors.Is(err, jobs.ErrNotFound) {
			writeError(w, http.StatusNotFound, "job not found", "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete job", "internal_error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRunJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id", "bad_request")
		return
	}

	if err := s.jobsSvc.Run(r.Context(), id); err != nil {
		switch {
		case errors.Is(err, jobs.ErrNotFound):
			writeError(w, http.StatusNotFound, "job not found", "not_found")
		case errors.Is(err, jobs.ErrAlreadyRunning):
			writeError(w, http.StatusConflict, "job is already running", "already_running")
		default:
			writeError(w, http.StatusInternalServerError, "failed to start job", "internal_error")
		}
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handlePauseJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id", "bad_request")
		return
	}

	if err := s.jobsSvc.Pause(r.Context(), id); err != nil {
		switch {
		case errors.Is(err, jobs.ErrNotFound):
			writeError(w, http.StatusNotFound, "job not found", "not_found")
		case errors.Is(err, jobs.ErrNotRunning):
			writeError(w, http.StatusConflict, "job is not running", "not_running")
		default:
			writeError(w, http.StatusInternalServerError, "failed to pause job", "internal_error")
		}
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleResumeJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseJobID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id", "bad_request")
		return
	}

	if err := s.jobsSvc.Resume(r.Context(), id); err != nil {
		switch {
		case errors.Is(err, jobs.ErrNotFound):
			writeError(w, http.StatusNotFound, "job not found", "not_found")
		case errors.Is(err, jobs.ErrAlreadyRunning):
			writeError(w, http.StatusConflict, "job is already running", "already_running")
		default:
			writeError(w, http.StatusInternalServerError, "failed to resume job", "internal_error")
		}
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func parseJobID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}
