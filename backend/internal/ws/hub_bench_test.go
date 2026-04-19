package ws_test

import (
	"sync"
	"testing"
	"time"

	"github.com/tidemarq/tidemarq/internal/ws"
)

// fakeConn is a fast in-memory wsConn that records message count.
// WriteMessage is lock-protected so it can be called from the hub's writePump
// goroutine concurrently with the test goroutine.
type fakeConn struct {
	mu   sync.Mutex
	msgs int
}

func (f *fakeConn) WriteMessage(_ int, _ []byte) error {
	f.mu.Lock()
	f.msgs++
	f.mu.Unlock()
	return nil
}

func (f *fakeConn) Close() error { return nil }

// slowConn blocks WriteMessage until done is closed. Used to simulate a client
// whose TCP window is full so the send channel fills up and triggers a drop.
type slowConn struct {
	once  sync.Once
	ready chan struct{} // closed on first WriteMessage call
	done  chan struct{} // closed to unblock WriteMessage
}

func (c *slowConn) WriteMessage(_ int, _ []byte) error {
	c.once.Do(func() { close(c.ready) })
	<-c.done
	return nil
}

func (c *slowConn) Close() error { return nil }

// ── Benchmark ─────────────────────────────────────────────────────────────────

// BenchmarkHub_Broadcast_10clients measures per-Broadcast cost with 10 connected
// clients. The hub must JSON-encode once, acquire a read lock, and do 10
// non-blocking channel sends per call.
func BenchmarkHub_Broadcast_10clients(b *testing.B) {
	hub := ws.New()
	for i := 0; i < 10; i++ {
		hub.Register(&fakeConn{})
	}
	evt := ws.Event{
		JobID:       1,
		Event:       "progress",
		FilesDone:   50,
		FilesTotal:  100,
		BytesDone:   1 << 20,
		CurrentFile: "subdir/large_file.bin",
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hub.Broadcast(evt)
	}
}

// ── Slow-client drop test ────────────────────────────────────────────────────

// TestHub_SlowClient_DroppedWithoutBlockingBroadcast verifies that a client
// whose send channel is full is dropped synchronously during Broadcast and does
// not delay the 9 fast clients. This is the core non-blocking guarantee of the hub.
func TestHub_SlowClient_DroppedWithoutBlockingBroadcast(t *testing.T) {
	hub := ws.New()

	for i := 0; i < 9; i++ {
		hub.Register(&fakeConn{})
	}

	ready := make(chan struct{})
	done := make(chan struct{})
	slow := &slowConn{ready: ready, done: done}
	hub.Register(slow)
	t.Cleanup(func() { close(done) }) // let writePump goroutine exit

	evt := ws.Event{JobID: 1, Event: "progress"}

	// First broadcast: writePump dequeues it and blocks inside WriteMessage.
	hub.Broadcast(evt)
	select {
	case <-ready:
		// writePump confirmed blocked — send channel has full capacity again.
	case <-time.After(2 * time.Second):
		t.Fatal("slow client writePump did not start within 2s")
	}

	// Fill the send channel to capacity (64 slots).
	for i := 0; i < 64; i++ {
		hub.Broadcast(evt)
	}

	// Next broadcast: channel is full → slow client dropped synchronously.
	start := time.Now()
	hub.Broadcast(evt)
	if elapsed := time.Since(start); elapsed > 10*time.Millisecond {
		t.Errorf("Broadcast blocked for %v after slow client had full channel (want <10ms)", elapsed)
	}
}
