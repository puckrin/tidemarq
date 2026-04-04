package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tidemarq/tidemarq/internal/notifications"
)

// ─── Targets ──────────────────────────────────────────────────────────────────

func (s *Server) handleListNotificationTargets(w http.ResponseWriter, r *http.Request) {
	list, err := s.notifSvc.ListTargets(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list targets", "internal_error")
		return
	}
	if list == nil {
		list = []*notifications.TargetView{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleCreateNotificationTarget(w http.ResponseWriter, r *http.Request) {
	var in notifications.TargetInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	view, err := s.notifSvc.CreateTarget(r.Context(), in)
	if errors.Is(err, notifications.ErrConflict) {
		writeError(w, http.StatusConflict, "target name already in use", "conflict")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "bad_request")
		return
	}
	writeJSON(w, http.StatusCreated, view)
}

func (s *Server) handleGetNotificationTarget(w http.ResponseWriter, r *http.Request) {
	id, err := parseNotifID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id", "bad_request")
		return
	}
	view, err := s.notifSvc.GetTarget(r.Context(), id)
	if errors.Is(err, notifications.ErrNotFound) {
		writeError(w, http.StatusNotFound, "target not found", "not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get target", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleUpdateNotificationTarget(w http.ResponseWriter, r *http.Request) {
	id, err := parseNotifID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id", "bad_request")
		return
	}
	var in notifications.TargetInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	view, err := s.notifSvc.UpdateTarget(r.Context(), id, in)
	if errors.Is(err, notifications.ErrNotFound) {
		writeError(w, http.StatusNotFound, "target not found", "not_found")
		return
	}
	if errors.Is(err, notifications.ErrConflict) {
		writeError(w, http.StatusConflict, "target name already in use", "conflict")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update target", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleDeleteNotificationTarget(w http.ResponseWriter, r *http.Request) {
	id, err := parseNotifID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id", "bad_request")
		return
	}
	if err := s.notifSvc.DeleteTarget(r.Context(), id); errors.Is(err, notifications.ErrNotFound) {
		writeError(w, http.StatusNotFound, "target not found", "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete target", "internal_error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ─── Rules ────────────────────────────────────────────────────────────────────

func (s *Server) handleListNotificationRules(w http.ResponseWriter, r *http.Request) {
	// Optional ?target_id= filter.
	var targetID int64
	if v := r.URL.Query().Get("target_id"); v != "" {
		var err error
		targetID, err = strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid target_id", "bad_request")
			return
		}
	}
	rules, err := s.notifSvc.ListRules(r.Context(), targetID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list rules", "internal_error")
		return
	}
	if rules == nil {
		rules = nil
	}
	writeJSON(w, http.StatusOK, rules)
}

func (s *Server) handleCreateNotificationRule(w http.ResponseWriter, r *http.Request) {
	var in notifications.RuleInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}
	rule, err := s.notifSvc.CreateRule(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), "bad_request")
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) handleDeleteNotificationRule(w http.ResponseWriter, r *http.Request) {
	id, err := parseNotifID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id", "bad_request")
		return
	}
	if err := s.notifSvc.DeleteRule(r.Context(), id); errors.Is(err, notifications.ErrNotFound) {
		writeError(w, http.StatusNotFound, "rule not found", "not_found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete rule", "internal_error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseNotifID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}
