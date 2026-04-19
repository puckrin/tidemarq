package engine_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tidemarq/tidemarq/internal/engine"
)

// TestEngine_Idempotency_5kFiles runs a full sync over 5 000 files and verifies
// that a second run with no changes produces zero copies and logs both run times
// so the fast-path speedup is visible without a fragile timing assertion.
func TestEngine_Idempotency_5kFiles(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	const nDirs, filesPerDir = 500, 10 // 5 000 files total

	for d := 0; d < nDirs; d++ {
		dir := filepath.Join(src, fmt.Sprintf("dir%04d", d))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for f := 0; f < filesPerDir; f++ {
			path := filepath.Join(dir, fmt.Sprintf("file%04d.txt", f))
			if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}

	cfg := engine.Config{
		JobID:           jobID,
		SourcePath:      src,
		DestinationPath: dst,
	}

	t0 := time.Now()
	first, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	firstDur := time.Since(t0)

	if first.FilesCopied != nDirs*filesPerDir {
		t.Errorf("first run: FilesCopied = %d, want %d", first.FilesCopied, nDirs*filesPerDir)
	}
	if len(first.Errors) != 0 {
		t.Fatalf("first run errors: %v", first.Errors)
	}

	t1 := time.Now()
	second, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	secondDur := time.Since(t1)

	t.Logf("first  run: %v (%d copied)", firstDur, first.FilesCopied)
	t.Logf("second run: %v (%d copied, %d skipped)", secondDur, second.FilesCopied, second.FilesSkipped)
	t.Logf("second/first ratio: %.1f%%", float64(secondDur)/float64(firstDur)*100)

	if second.FilesCopied != 0 {
		t.Errorf("idempotency: second run FilesCopied = %d, want 0", second.FilesCopied)
	}
	if second.FilesSkipped != nDirs*filesPerDir {
		t.Errorf("idempotency: second run FilesSkipped = %d, want %d",
			second.FilesSkipped, nDirs*filesPerDir)
	}
	if len(second.Errors) != 0 {
		t.Errorf("second run errors: %v", second.Errors)
	}
}
