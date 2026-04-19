package engine_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tidemarq/tidemarq/internal/engine"
)

// TestEngine_LargeFileSync_HeapGrowth writes a 1 GiB file, syncs it with delta
// disabled (streaming io.Copy path), and asserts that live heap after GC grows
// by less than 50 MB. Delta is explicitly off so the engine never calls
// delta.Diff's io.ReadAll, which would load the whole file into memory.
func TestEngine_LargeFileSync_HeapGrowth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 1 GiB memory test in short mode")
	}

	eng, jobID, src, dst := testEnv(t)

	// Write a 1 GiB source file in 1 MiB chunks to avoid a 1 GB stack allocation.
	const fileSize = 1 << 30 // 1 GiB
	srcFile := filepath.Join(src, "large.bin")
	f, err := os.Create(srcFile)
	if err != nil {
		t.Fatal(err)
	}
	chunk := make([]byte, 1<<20) // 1 MiB
	for i := range chunk {
		chunk[i] = byte(i & 0xff)
	}
	for written := int64(0); written < fileSize; written += int64(len(chunk)) {
		if _, err := f.Write(chunk); err != nil {
			f.Close()
			t.Fatalf("writing source file: %v", err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	// Establish a clean heap baseline.
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	result, err := eng.Run(context.Background(), engine.Config{
		JobID:           jobID,
		SourcePath:      src,
		DestinationPath: dst,
		UseDelta:        false, // streaming path; delta would io.ReadAll the whole file
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.FilesCopied != 1 {
		t.Errorf("FilesCopied = %d, want 1", result.FilesCopied)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("run errors: %v", result.Errors)
	}

	// GC after the run to collect transient allocations.
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	// HeapInuse is the bytes in live heap spans after GC — a reliable measure
	// of memory actually retained (as opposed to HeapAlloc which includes dead objects).
	heapGrowthMB := float64(int64(after.HeapInuse)-int64(before.HeapInuse)) / (1 << 20)
	dstSize := fileSize >> 20
	t.Logf("copied %d MiB; heap growth after GC: %.1f MB (before=%s after=%s)",
		dstSize,
		heapGrowthMB,
		formatBytes(before.HeapInuse),
		formatBytes(after.HeapInuse),
	)

	const limitMB = 50
	if heapGrowthMB > limitMB {
		t.Errorf("heap grew by %.1f MB after syncing 1 GiB (limit %d MB) — "+
			"check for io.ReadAll or large buffer allocations in the copy path",
			heapGrowthMB, limitMB)
	}
}

func formatBytes(b uint64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(b)/(1<<20))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
