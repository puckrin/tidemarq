package jobs

import (
	"sync"
	"time"
)

// etaWindow is the rolling-window length used to compute current throughput.
// 30 s is long enough to smooth over a single large file but short enough to
// react when the engine moves from a folder of huge files to many small ones.
const etaWindow = 30 * time.Second

// etaMinSamples is the minimum number of samples required before the rolling
// throughput estimate is trusted. Below this the tracker falls back to the
// prior-run seed (if any).
const etaMinSamples = 3

// etaSample records cumulative bytes reviewed at a point in time.
type etaSample struct {
	at    time.Time
	bytes int64
}

// etaTracker estimates time remaining for a sync run.
//
// It consumes cumulative BytesReviewed observations from the engine's Progress
// stream, computes throughput from the last etaWindow of samples, and reports a
// remaining-seconds estimate based on the still-to-review byte count.
//
// When fewer than etaMinSamples are available it falls back to a prior-run seed:
// the wall-clock duration of the most recent successful run for the same job,
// linearly decayed by time elapsed in the current run. With no prior and not
// enough samples, EstimateETA returns 0 — the UI shows "Calculating…" in that case.
type etaTracker struct {
	mu        sync.Mutex
	samples   []etaSample
	startedAt time.Time
	prior     time.Duration // 0 means no prior run is available
}

func newETATracker(prior time.Duration) *etaTracker {
	return &etaTracker{
		startedAt: time.Now(),
		prior:     prior,
	}
}

// Update records the latest cumulative bytes-reviewed observation. Old samples
// outside the rolling window are pruned.
func (t *etaTracker) Update(bytesReviewed int64) {
	if bytesReviewed <= 0 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	t.samples = append(t.samples, etaSample{at: now, bytes: bytesReviewed})
	cutoff := now.Add(-etaWindow)
	keep := 0
	for i, s := range t.samples {
		if !s.at.Before(cutoff) {
			keep = i
			break
		}
	}
	if keep > 0 {
		t.samples = t.samples[keep:]
	}
}

// EstimateETA returns the predicted seconds remaining until bytesScanned has
// been fully reviewed. Returns 0 when no useful estimate is available, so the
// UI can display "Calculating…" instead of a misleading number.
func (t *etaTracker) EstimateETA(bytesReviewed, bytesScanned int64) int {
	t.mu.Lock()
	defer t.mu.Unlock()

	remaining := bytesScanned - bytesReviewed
	if remaining <= 0 {
		return 0
	}

	if len(t.samples) >= etaMinSamples {
		oldest := t.samples[0]
		newest := t.samples[len(t.samples)-1]
		elapsed := newest.at.Sub(oldest.at).Seconds()
		delta := newest.bytes - oldest.bytes
		if elapsed > 0 && delta > 0 {
			rate := float64(delta) / elapsed // bytes/sec
			return int(float64(remaining) / rate)
		}
	}

	// Not enough samples yet — fall back to the prior-run seed decayed by elapsed time.
	if t.prior > 0 {
		left := t.prior - time.Since(t.startedAt)
		if left > 0 {
			return int(left.Seconds())
		}
	}
	return 0
}
