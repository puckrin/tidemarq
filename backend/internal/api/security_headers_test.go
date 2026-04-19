package api_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestSecurityHeaders_Present(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := doRequest(t, ts, http.MethodGet, "/health", nil, nil)
	defer resp.Body.Close()

	want := map[string]string{
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains",
		"X-Frame-Options":           "DENY",
		"X-Content-Type-Options":    "nosniff",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Permissions-Policy":        "camera=(), microphone=(), geolocation=()",
	}
	for header, wantVal := range want {
		if got := resp.Header.Get(header); got != wantVal {
			t.Errorf("%s: want %q, got %q", header, wantVal, got)
		}
	}
}

func TestSecurityHeaders_CSP(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := doRequest(t, ts, http.MethodGet, "/health", nil, nil)
	defer resp.Body.Close()

	csp := resp.Header.Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("Content-Security-Policy header is missing")
	}
	for _, directive := range []string{
		"default-src 'self'",
		"frame-ancestors 'none'",
		"form-action 'self'",
		"connect-src 'self'",
	} {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP missing %q; full value: %s", directive, csp)
		}
	}
}

func TestSecurityHeaders_OnAuthenticatedEndpoint(t *testing.T) {
	ts, token := newTestServer(t)
	resp := doRequest(t, ts, http.MethodGet, "/api/v1/jobs", &token, nil)
	defer resp.Body.Close()

	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options: want nosniff, got %q", got)
	}
	if got := resp.Header.Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("X-Frame-Options: want DENY, got %q", got)
	}
}

func TestSecurityHeaders_OnLoginEndpoint(t *testing.T) {
	ts, _ := newTestServer(t)
	resp := doRequest(t, ts, http.MethodPost, "/api/v1/auth/login", nil, nil)
	defer resp.Body.Close()

	if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options on login: want nosniff, got %q", got)
	}
}
