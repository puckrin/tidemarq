package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.db.GetSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get settings", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		VersionsToKeep          int `json:"versions_to_keep"`
		QuarantineRetentionDays int `json:"quarantine_retention_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	if in.VersionsToKeep < 0 {
		writeError(w, http.StatusBadRequest, "versions_to_keep must be 0 or greater", "bad_request")
		return
	}
	if in.QuarantineRetentionDays < 1 {
		writeError(w, http.StatusBadRequest, "quarantine_retention_days must be at least 1", "bad_request")
		return
	}
	settings, err := s.db.UpdateSettings(r.Context(), in.VersionsToKeep, in.QuarantineRetentionDays)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update settings", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}
