package api

import (
	"encoding/json"
	"net/http"

	"github.com/tidemarq/tidemarq/internal/auth"
	"github.com/tidemarq/tidemarq/internal/db"
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
		// Run bcrypt against a dummy hash so the response time is
		// indistinguishable from a valid username with a wrong password.
		// Without this, timing alone reveals whether a username exists.
		auth.CheckPasswordDummy(req.Password)
		writeError(w, http.StatusUnauthorized, "invalid credentials", "invalid_credentials")
		return
	}

	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid credentials", "invalid_credentials")
		return
	}

	token, err := s.authSvc.IssueToken(user.ID, user.Username, user.Role, user.PasswordChangeRequired)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token", "internal_error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// handleChangePassword rotates the authenticated user's password and issues a
// fresh JWT without the password_change_required flag. Verifies the current
// password to prevent a stolen JWT from being used to take over the account.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "current_password and new_password are required", "bad_request")
		return
	}
	if len(req.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "new password must be at least 8 characters", "bad_request")
		return
	}
	if req.NewPassword == req.CurrentPassword {
		writeError(w, http.StatusBadRequest, "new password must differ from current password", "bad_request")
		return
	}

	user, err := s.db.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load user", "internal_error")
		return
	}

	if !auth.CheckPassword(user.PasswordHash, req.CurrentPassword) {
		// 400 — not 401 — because the JWT is fine; only the body field is
		// wrong. Returning 401 would trip the frontend's session-expired
		// handler and log the user out instead of showing the form error.
		writeError(w, http.StatusBadRequest, "current password is incorrect", "invalid_current_password")
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password", "internal_error")
		return
	}

	if _, err := s.db.UpdateUser(r.Context(), user.ID, db.UpdateUserParams{PasswordHash: &hash}); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update password", "internal_error")
		return
	}
	if err := s.db.ClearPasswordChangeRequired(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear password change flag", "internal_error")
		return
	}

	token, err := s.authSvc.IssueToken(user.ID, user.Username, user.Role, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token", "internal_error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}
