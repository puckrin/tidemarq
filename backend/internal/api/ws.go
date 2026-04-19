package api

import (
	"crypto/sha256"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tidemarq/tidemarq/internal/auth"
)

// wsTokenTTL is how long a WS token is valid. Short-lived so that a leaked
// token expires quickly; the frontend requests a fresh one immediately before
// opening the socket, so there is no UX impact of a tight TTL.
const wsTokenTTL = 30 * time.Second

// upgrader is the gorilla WebSocket upgrader.
// CheckOrigin accepts connections where the Origin header is absent (non-browser
// clients, dev-proxy) or where the Origin host matches the request Host header.
// This restores the browser same-origin protection that "return true" bypassed.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			// No Origin header → not a browser cross-origin request; allow.
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		// In production the origin and request hosts match exactly.
		if strings.EqualFold(u.Host, r.Host) {
			return true
		}
		// During local development the Vite dev server proxies WebSocket
		// connections from a different port (e.g. localhost:5174 → localhost:8443).
		// The browser sends the Vite origin, but the backend sees its own host.
		// Allow any localhost-to-localhost connection regardless of port so the
		// dev proxy works without weakening same-origin protection in production.
		originHost := u.Hostname()
		requestHost := r.Host
		if i := strings.LastIndex(requestHost, ":"); i != -1 {
			requestHost = requestHost[:i]
		}
		isLocalOrigin := originHost == "localhost" || originHost == "127.0.0.1"
		isLocalRequest := requestHost == "localhost" || requestHost == "127.0.0.1"
		return isLocalOrigin && isLocalRequest
	},
}

// wsNonceStore enforces single-use on WS tokens. Each token is keyed by its
// SHA-256 hash and expires after wsTokenTTL, preventing replay within the
// token's validity window.
type wsNonceStore struct {
	mu   sync.Mutex
	seen map[[32]byte]time.Time // sha256(token) → expiry
}

func newWSNonceStore() *wsNonceStore {
	return &wsNonceStore{seen: make(map[[32]byte]time.Time)}
}

// consume marks token as seen and returns true. Returns false if the token was
// already consumed, indicating a replay attempt. Stale entries are purged on
// each call to keep the map bounded.
func (n *wsNonceStore) consume(token string) bool {
	key := sha256.Sum256([]byte(token))
	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Now()
	for k, exp := range n.seen {
		if now.After(exp) {
			delete(n.seen, k)
		}
	}

	if _, exists := n.seen[key]; exists {
		return false
	}
	n.seen[key] = now.Add(wsTokenTTL)
	return true
}

// handleWSToken issues a short-lived token for authenticating the WebSocket connection.
func (s *Server) handleWSToken(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	token, err := s.authSvc.IssueWSToken(claims.UserID, claims.Username, claims.Role, wsTokenTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token", "internal_error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// handleWS upgrades the connection to WebSocket and registers it with the hub.
// Authentication is via a short-lived token passed as the `token` query parameter
// or a Bearer token in the Authorization header.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		authHeader := r.Header.Get("Authorization")
		tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
	}
	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	if _, err := s.authSvc.ValidateToken(tokenStr); err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	// Single-use check: reject a token that has already opened a WS connection.
	// This runs before Upgrade so the rejection can still be sent as a plain
	// HTTP 401 response.
	if !s.wsNonces.consume(tokenStr) {
		http.Error(w, "token already used", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	unregister := s.hub.Register(conn)
	defer unregister()

	// Keep the connection alive; discard any incoming messages.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
