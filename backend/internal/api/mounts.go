package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/tidemarq/tidemarq/internal/mounts"
)

func (s *Server) handleListMounts(w http.ResponseWriter, r *http.Request) {
	list, err := s.mountsSvc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list mounts", "internal_error")
		return
	}
	if list == nil {
		list = []*mounts.MountView{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateMount(w http.ResponseWriter, r *http.Request) {
	var in mounts.MountInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	view, err := s.mountsSvc.Create(r.Context(), in)
	if errors.Is(err, mounts.ErrConflict) {
		writeError(w, http.StatusConflict, "mount name already in use", "conflict")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "bad_request")
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

func (s *Server) handleGetMount(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid mount id", "bad_request")
		return
	}

	view, err := s.mountsSvc.Get(r.Context(), id)
	if errors.Is(err, mounts.ErrNotFound) {
		writeError(w, http.StatusNotFound, "mount not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get mount", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleUpdateMount(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid mount id", "bad_request")
		return
	}

	var in mounts.MountInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	view, err := s.mountsSvc.Update(r.Context(), id, in)
	if errors.Is(err, mounts.ErrNotFound) {
		writeError(w, http.StatusNotFound, "mount not found", "not_found")
		return
	}
	if errors.Is(err, mounts.ErrConflict) {
		writeError(w, http.StatusConflict, "mount name already in use", "conflict")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update mount", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleDeleteMount(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid mount id", "bad_request")
		return
	}

	if err := s.mountsSvc.Delete(r.Context(), id); errors.Is(err, mounts.ErrNotFound) {
		writeError(w, http.StatusNotFound, "mount not found", "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete mount", "internal_error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleTestMount(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid mount id", "bad_request")
		return
	}

	if err := s.mountsSvc.TestConnectivity(r.Context(), id); errors.Is(err, mounts.ErrNotFound) {
		writeError(w, http.StatusNotFound, "mount not found", "not_found")
		return
	} else if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

