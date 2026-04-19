package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCreateUser_PasswordTooShort(t *testing.T) {
	ts, adminToken := newTestServer(t)

	body, _ := json.Marshal(map[string]string{"username": "newuser", "password": "short", "role": "viewer"})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/users", &adminToken, bytes.NewReader(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for short password, got %d", resp.StatusCode)
	}
}

func TestCreateUser_EmptyPassword(t *testing.T) {
	ts, adminToken := newTestServer(t)

	body, _ := json.Marshal(map[string]string{"username": "newuser", "password": "", "role": "viewer"})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/users", &adminToken, bytes.NewReader(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty password, got %d", resp.StatusCode)
	}
}

func TestCreateUser_PasswordExactlyMinLength(t *testing.T) {
	ts, adminToken := newTestServer(t)

	body, _ := json.Marshal(map[string]string{"username": "newuser", "password": "12345678", "role": "viewer"})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/users", &adminToken, bytes.NewReader(body))
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for 8-char password, got %d", resp.StatusCode)
	}
}

func TestUpdateUser_PasswordTooShort(t *testing.T) {
	ts, adminToken := newTestServer(t)

	// Create a user first.
	body, _ := json.Marshal(map[string]string{"username": "patchme", "password": "goodpasswd", "role": "viewer"})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/users", &adminToken, bytes.NewReader(body))
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", resp.StatusCode)
	}

	// List to get ID.
	listResp := doRequest(t, ts, http.MethodGet, "/api/v1/users", &adminToken, nil)
	defer listResp.Body.Close()
	var users []map[string]any
	json.NewDecoder(listResp.Body).Decode(&users) //nolint:errcheck
	var userID float64
	for _, u := range users {
		if u["username"] == "patchme" {
			userID = u["id"].(float64)
		}
	}

	// Try to set a short password.
	upd, _ := json.Marshal(map[string]string{"password": "tiny"})
	updResp := doRequest(t, ts, http.MethodPut, "/api/v1/users/"+idStr(userID), &adminToken, bytes.NewReader(upd))
	defer updResp.Body.Close()

	if updResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for short update password, got %d", updResp.StatusCode)
	}
}

func TestResolveConflict_InvalidAction(t *testing.T) {
	ts, token, src, dst := newFullTestServer(t)

	// Create a two-way job and a conflict.
	body, _ := json.Marshal(map[string]any{
		"name": "inv-action-job", "source_path": src, "destination_path": dst,
		"mode": "two-way", "conflict_strategy": "ask-user",
	})
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/jobs", &token, bytes.NewReader(body))
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created) //nolint:errcheck
	resp.Body.Close()

	// Use a fake conflict ID — the action validation fires before the DB lookup.
	resolveBody, _ := json.Marshal(map[string]string{"action": "destroy-everything"})
	resolveResp := doRequest(t, ts, http.MethodPost, "/api/v1/conflicts/999/resolve", &token, bytes.NewReader(resolveBody))
	defer resolveResp.Body.Close()

	if resolveResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid action: expected 400, got %d", resolveResp.StatusCode)
	}
}
