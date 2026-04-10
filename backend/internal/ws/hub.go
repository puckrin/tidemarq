// Package ws provides a WebSocket hub for broadcasting job progress events.
package ws

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
)

// Event is a progress or lifecycle notification broadcast to all WS clients.
type Event struct {
	JobID        int64   `json:"job_id"`
	Event        string  `json:"event"` // "started","progress","paused","completed","error","conflict_resolved"
	FilesDone    int     `json:"files_done,omitempty"`
	FilesTotal   int     `json:"files_total,omitempty"`
	FilesSkipped int     `json:"files_skipped,omitempty"`
	BytesDone    int64   `json:"bytes_done,omitempty"`
	RateKBs      float64 `json:"rate_kbs,omitempty"`
	ETASecs      int     `json:"eta_secs,omitempty"`
	CurrentFile  string  `json:"current_file,omitempty"`
	FileAction   string  `json:"file_action,omitempty"` // "scanning","copying","copied","skipped","removing","present"
	Message      string  `json:"message,omitempty"`
}

// client wraps a WebSocket connection with a send channel.
type client struct {
	conn   *websocket.Conn
	sendCh chan []byte
}

// Hub maintains the set of connected WebSocket clients and broadcasts events to them.
// Handlers must never write directly to connections — all writes go through Broadcast.
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
}

// New creates a Hub.
func New() *Hub {
	return &Hub{clients: make(map[*client]struct{})}
}

// Register adds a WebSocket connection to the hub and starts its write pump.
// The caller must have performed the HTTP upgrade before calling Register.
func (h *Hub) Register(conn *websocket.Conn) {
	c := &client{conn: conn, sendCh: make(chan []byte, 64)}

	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	go c.writePump(func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
	})
}

// Broadcast sends e to all connected clients. Clients that cannot keep up are dropped.
func (h *Hub) Broadcast(e Event) {
	data, err := json.Marshal(e)
	if err != nil {
		return
	}

	var slow []*client

	h.mu.RLock()
	for c := range h.clients {
		select {
		case c.sendCh <- data:
		default:
			slow = append(slow, c)
		}
	}
	h.mu.RUnlock()

	// Remove slow clients under a write lock so the map is not mutated during RLock.
	if len(slow) > 0 {
		h.mu.Lock()
		for _, c := range slow {
			if _, ok := h.clients[c]; ok {
				close(c.sendCh)
				delete(h.clients, c)
			}
		}
		h.mu.Unlock()
	}
}

// writePump drains sendCh to the WebSocket connection.
func (c *client) writePump(onClose func()) {
	defer func() {
		c.conn.Close()
		onClose()
	}()
	for msg := range c.sendCh {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}
