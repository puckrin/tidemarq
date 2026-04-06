package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tidemarq/tidemarq/internal/auth"
)

const wsTokenTTL = 60 * time.Second

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins; TLS + JWT provides security.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// handleWSToken issues a short-lived token for authenticating the WebSocket connection.
func (s *Server) handleWSToken(w http.ResponseWriter, r *http.Request) {
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "unauthorized")
		return
	}
	token, err := s.authSvc.IssueTokenTTL(claims.UserID, claims.Username, claims.Role, wsTokenTTL)
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

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	s.hub.Register(conn)

	// Keep the connection alive; discard any incoming messages.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
