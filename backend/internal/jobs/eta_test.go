package jobs

import (
	"testing"
	"time"
)

// TestETATracker_ReturnsZeroWithoutSamplesOrPrior verifies that the UI sees a 0
// ETA (which renders as "Calculating…") when neither a prior run nor enough
// samples are available.
func TestETATracker_ReturnsZeroWithoutSamplesOrPrior(t *testing.T) {
	tr := newETATracker(0)
	if got := tr.EstimateETA(0, 1_000_000); got != 0 {
		t.Errorf("EstimateETA with no samples and no prior: got %d, want 0", got)
	}

	// One sample is not enough either.
	tr.Update(1_000)
	if got := tr.EstimateETA(1_000, 1_000_000); got != 0 {
		t.Errorf("EstimateETA with one sample: got %d, want 0", got)
	}
}

// TestETATracker_UsesPriorBeforeSamples verifies that the prior-run seed is used
// to produce a non-zero estimate before enough samples accumulate.
func TestETATracker_UsesPriorBeforeSamples(t *testing.T) {
	tr := newETATracker(10 * time.Minute)
	got := tr.EstimateETA(0, 1_000_000)
	if got <= 0 {
		t.Errorf("EstimateETA with prior duration: got %d, want a positive estimate", got)
	}
	// The prior was 10 min = 600 s; with virtually no elapsed time we should be near that.
	if got < 590 || got > 600 {
		t.Errorf("EstimateETA with 10-min prior: got %d, want ~600", got)
	}
}

// TestETATracker_ComputesRateFromSamples verifies that once enough samples are
// in the rolling window, the tracker prefers the live throughput rate over the
// prior-run seed.
func TestETATracker_ComputesRateFromSamples(t *testing.T) {
	tr := newETATracker(0)
	// Manually inject samples that span 10 seconds and 1 MB of progress
	// (= 100 KB/s). With 9 MB remaining the ETA should be ~90 s.
	start := time.Now().Add(-10 * time.Second)
	tr.samples = []etaSample{
		{at: start, bytes: 0},
		{at: start.Add(5 * time.Second), bytes: 500_000},
		{at: start.Add(10 * time.Second), bytes: 1_000_000},
	}
	got := tr.EstimateETA(1_000_000, 10_000_000)
	if got < 80 || got > 100 {
		t.Errorf("EstimateETA with 100 KB/s rate and 9 MB remaining: got %d, want ~90", got)
	}
}

// TestETATracker_ReturnsZeroWhenComplete verifies no ETA is reported once the
// review byte counter reaches the scanned total.
func TestETATracker_ReturnsZeroWhenComplete(t *testing.T) {
	tr := newETATracker(5 * time.Minute)
	tr.Update(1_000_000)
	tr.Update(1_500_000)
	tr.Update(2_000_000)
	if got := tr.EstimateETA(2_000_000, 2_000_000); got != 0 {
		t.Errorf("EstimateETA with bytesReviewed == bytesScanned: got %d, want 0", got)
	}
}
