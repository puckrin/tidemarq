package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/filter"
	"github.com/tidemarq/tidemarq/internal/jobs"
)

type jobRequest struct {
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
	// HashAlgo selects the file integrity hash algorithm: "sha256" or "blake3".
	// Omit or set to "" to accept the server default (currently "blake3").
	HashAlgo       string `json:"hash_algo,omitempty"`
	UseDelta       bool   `json:"use_delta"`
	DeltaBlockSize int64  `json:"delta_block_size,omitempty"`
	DeltaMinBytes  int64  `json:"delta_min_bytes,omitempty"`
	// Filters is the §3.3 file-filtering ruleset. Omit or set to null for
	// no filtering. The service validates the ruleset before persisting it
	// and returns 400 on any validation failure.
	//
	// Update semantics: omitting filters in an Update request CLEARS the
	// existing ruleset (matches how every other field on this endpoint
	// works — the request body is the full intended state, not a patch).
	Filters *filter.Ruleset `json:"filters,omitempty"`
}

// jobResponse mirrors db.Job for the API surface but exposes the persisted
// ruleset as a parsed object rather than the raw filters_json blob.
type jobResponse struct {
	*db.Job
	Filters *filter.Ruleset `json:"filters,omitempty"`
}

func toJobResponse(j *db.Job) jobResponse {
	rs, err := decodeFiltersField(j.FiltersJSON)
	if err != nil {
		// Persisted JSON failed validation — log it and return the job
		// without filters so the rest of the response still arrives. The
		// frontend will see no rules; the operator can reset them via Update.
		log.Printf("api: job %d has invalid filters_json (%v); returning without filters", j.ID, err)
	}
	return jobResponse{Job: j, Filters: rs}
}

func toJobResponses(jobs []*db.Job) []jobResponse {
	out := make([]jobResponse, len(jobs))
	for i, j := range jobs {
		out[i] = toJobResponse(j)
	}
	return out
}

// decodeFiltersField parses a persisted filters_json blob into a ruleset for
// the API response. "{}" and "" collapse to nil — the same sentinel the
// engine treats as "no filtering". Validation runs because the persisted
// value MAY have come from a hand-edit; we don't want to round-trip a
// malformed ruleset out to the client.
func decodeFiltersField(s string) (*filter.Ruleset, error) {
	if s == "" || s == "{}" {
		return nil, nil
	}
	var rs filter.Ruleset
	if err := json.Unmarshal([]byte(s), &rs); err != nil {
		return nil, err
	}
	if err := rs.Validate(); err != nil {
		return nil, err
	}
	if !rs.ExcludeHidden && len(rs.Rules) == 0 {
		return nil, nil
	}
	return &rs, nil
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
	writeJSON(w, http.StatusOK, toJobResponses(list))
}

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var req jobRequest
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
		WatchEnabled:   req.WatchEnabled,
		FullChecksum:   req.FullChecksum,
		HashAlgo:       req.HashAlgo,
		UseDelta:       req.UseDelta,
		DeltaBlockSize: req.DeltaBlockSize,
		DeltaMinBytes:  req.DeltaMinBytes,
		Filters:        req.Filters,
	})
	if err != nil {
		if errors.Is(err, jobs.ErrNameConflict) {
			writeError(w, http.StatusConflict, err.Error(), "conflict")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error(), "bad_request")
		return
	}

	writeJSON(w, http.StatusCreated, toJobResponse(job))
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
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

	writeJSON(w, http.StatusOK, toJobResponse(job))
}

func (s *Server) handleUpdateJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id", "bad_request")
		return
	}

	var req jobRequest
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
		WatchEnabled:   req.WatchEnabled,
		FullChecksum:   req.FullChecksum,
		HashAlgo:       req.HashAlgo,
		UseDelta:       req.UseDelta,
		DeltaBlockSize: req.DeltaBlockSize,
		DeltaMinBytes:  req.DeltaMinBytes,
		Filters:        req.Filters,
	})
	if err != nil {
		if errors.Is(err, jobs.ErrNotFound) {
			writeError(w, http.StatusNotFound, "job not found", "not_found")
			return
		}
		if errors.Is(err, jobs.ErrNameConflict) {
			writeError(w, http.StatusConflict, err.Error(), "conflict")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error(), "bad_request")
		return
	}

	writeJSON(w, http.StatusOK, toJobResponse(job))
}

func (s *Server) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
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
	id, err := parseIDParam(r)
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
	id, err := parseIDParam(r)
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
	id, err := parseIDParam(r)
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

