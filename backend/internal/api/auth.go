package api

import (
	"encoding/json"
	"net/http"

	"github.com/tidemarq/tidemarq/internal/auth"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusUnauthorized, "invalid credentials", "invalid_credentials")
		return
	}

	user, err := s.db.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		// Constant-time response to prevent username enumeration.
		writeError(w, http.StatusUnauthorized, "invalid credentials", "invalid_credentials")
		return
	}

	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials", "invalid_credentials")
		return
	}

	token, err := s.authSvc.IssueToken(user.ID, user.Username, user.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token", "internal_error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}
