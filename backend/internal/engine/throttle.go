package engine

import (
	"io"
	"time"
)

// throttledReader wraps an io.Reader and limits throughput to rateKBs kilobytes per second.
type throttledReader struct {
	r       io.Reader
	rateKBs int64
	start   time.Time
	read    int64
}

// newThrottledReader returns r wrapped in a rate-limiter. If rateKBs <= 0, r is returned unchanged.
func newThrottledReader(r io.Reader, rateKBs int64) io.Reader {
	if rateKBs <= 0 {
		return r
	}
	return &throttledReader{r: r, rateKBs: rateKBs, start: time.Now()}
}

func (t *throttledReader) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if n > 0 {
		t.read += int64(n)
		// Time it should have taken to transfer t.read bytes at the configured rate.
		expectedMs := t.read * 1000 / (t.rateKBs * 1024)
		expected := time.Duration(expectedMs) * time.Millisecond
		if elapsed := time.Since(t.start); elapsed < expected {
			time.Sleep(expected - elapsed)
		}
	}
	return n, err
}
