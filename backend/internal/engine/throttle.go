package engine

import (
	"io"
	"time"
)

// throttledReader wraps an io.Reader, limits throughput to rateKBs KB/s, and
// checks pauseCh before each chunk. A closed pauseCh causes Read to return
// (0, io.EOF) so the copy loop stops cleanly at a chunk boundary.
type throttledReader struct {
	r       io.Reader
	rateKBs int64
	pauseCh <-chan struct{}
	start   time.Time
	read    int64
	paused  bool
}

// newThrottledReader returns r wrapped in a rate-limiter and pause checker.
// If rateKBs <= 0 and pauseCh is nil, r is returned unchanged.
func newThrottledReader(r io.Reader, rateKBs int64, pauseCh <-chan struct{}) io.Reader {
	if rateKBs <= 0 && pauseCh == nil {
		return r
	}
	return &throttledReader{r: r, rateKBs: rateKBs, pauseCh: pauseCh, start: time.Now()}
}

func (t *throttledReader) Read(p []byte) (int, error) {
	// Check for pause before each chunk.
	if t.pauseCh != nil {
		select {
		case <-t.pauseCh:
			t.paused = true
			return 0, io.EOF // stop io.Copy cleanly
		default:
		}
	}

	n, err := t.r.Read(p)
	if n > 0 && t.rateKBs > 0 {
		t.read += int64(n)
		expectedMs := t.read * 1000 / (t.rateKBs * 1024)
		expected := time.Duration(expectedMs) * time.Millisecond
		if elapsed := time.Since(t.start); elapsed < expected {
			time.Sleep(expected - elapsed)
		}
	}
	return n, err
}
