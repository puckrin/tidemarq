package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/tidemarq/tidemarq/internal/auth"
	"github.com/tidemarq/tidemarq/internal/db"
)

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type updateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

var validRoles = map[string]bool{
	"admin":    true,
	"operator": true,
	"viewer":   true,
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.db.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list users", "internal_error")
		return
	}
	if users == nil {
		users = []*db.User{}
	}
	writeJSON(w, http.StatusOK, users)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password are required", "bad_request")
		return
	}
	if !validRoles[req.Role] {
		writeError(w, http.StatusBadRequest, "role must be admin, operator, or viewer", "bad_request")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password", "internal_error")
		return
	}

	user, err := s.db.CreateUser(r.Context(), req.Username, hash, req.Role)
	if err != nil {
		if err == db.ErrConflict {
			writeError(w, http.StatusConflict, "username already exists", "conflict")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create user", "internal_error")
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id", "bad_request")
		return
	}

	user, err := s.db.GetUserByID(r.Context(), id)
	if err != nil {
		if err == db.ErrNotFound {
			writeError(w, http.StatusNotFound, "user not found", "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get user", "internal_error")
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id", "bad_request")
		return
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "bad_request")
		return
	}

	if req.Role != "" && !validRoles[req.Role] {
		writeError(w, http.StatusBadRequest, "role must be admin, operator, or viewer", "bad_request")
		return
	}

	params := db.UpdateUserParams{}
	if req.Username != "" {
		params.Username = &req.Username
	}
	if req.Password != "" {
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to hash password", "internal_error")
			return
		}
		params.PasswordHash = &hash
	}
	if req.Role != "" {
		params.Role = &req.Role
	}

	user, err := s.db.UpdateUser(r.Context(), id, params)
	if err != nil {
		switch err {
		case db.ErrNotFound:
			writeError(w, http.StatusNotFound, "user not found", "not_found")
		case db.ErrConflict:
			writeError(w, http.StatusConflict, "username already exists", "conflict")
		default:
			writeError(w, http.StatusInternalServerError, "failed to update user", "internal_error")
		}
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id", "bad_request")
		return
	}

	if err := s.db.DeleteUser(r.Context(), id); err != nil {
		if err == db.ErrNotFound {
			writeError(w, http.StatusNotFound, "user not found", "not_found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete user", "internal_error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseIDParam(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}
