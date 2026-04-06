package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON_success(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"hello": "world"})

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %q", ct)
	}
	var got map[string]string
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["hello"] != "world" {
		t.Fatalf("unexpected body: %v", got)
	}
}

func TestWriteJSON_unencodable(t *testing.T) {
	w := httptest.NewRecorder()
	// Channels cannot be JSON-encoded; this must not produce a 200 with a broken body.
	writeJSON(w, http.StatusOK, map[string]any{"bad": make(chan int)})

	res := w.Result()
	if res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", res.StatusCode)
	}
	// Body must be valid JSON with an error field, not empty or truncated.
	var got map[string]string
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if got["error"] == "" {
		t.Fatalf("expected error field in response, got: %v", got)
	}
}

func TestWriteJSON_statusPreserved(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]int{"id": 42})

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}
