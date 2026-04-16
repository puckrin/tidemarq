package api

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		log.Printf("writeJSON: failed to encode response: %v", err)
		http.Error(w, `{"error":"internal error","code":"internal_error"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func writeError(w http.ResponseWriter, status int, msg, code string) {
	writeJSON(w, status, errorResponse{Error: msg, Code: code})
}

func parseIDParam(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// parseOptionalJobID parses the optional "job_id" query parameter.
// Returns 0 if the parameter is absent, or writes a 400 and returns false if malformed.
func parseOptionalJobID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	q := r.URL.Query().Get("job_id")
	if q == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(q, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job_id", "bad_request")
		return 0, false
	}
	return id, true
}
