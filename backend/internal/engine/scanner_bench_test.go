package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// buildSyntheticTree creates nDirs subdirectories each containing filesPerDir
// empty files under root. Setup cost is excluded from benchmark timing via
// b.ResetTimer().
func buildSyntheticTree(tb testing.TB, root string, nDirs, filesPerDir int) {
	tb.Helper()
	for d := 0; d < nDirs; d++ {
		dir := filepath.Join(root, fmt.Sprintf("dir%04d", d))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			tb.Fatal(err)
		}
		for f := 0; f < filesPerDir; f++ {
			path := filepath.Join(dir, fmt.Sprintf("file%04d.dat", f))
			if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
				tb.Fatal(err)
			}
		}
	}
}

// BenchmarkScanDir_10k measures scanDir over 10 000 files (500 dirs × 20 files).
// Sub-benchmarks sweep worker counts so you can see the parallelism benefit.
func BenchmarkScanDir_10k(b *testing.B) {
	root := b.TempDir()
	const nDirs, filesPerDir = 500, 20 // 10 000 files total
	buildSyntheticTree(b, root, nDirs, filesPerDir)

	workerCounts := []int{1, 2, 4, 8, runtime.NumCPU()}
	// Deduplicate in case NumCPU is already in the list.
	seen := map[int]bool{}
	unique := workerCounts[:0]
	for _, w := range workerCounts {
		if !seen[w] {
			seen[w] = true
			unique = append(unique, w)
		}
	}

	for _, workers := range unique {
		workers := workers
		b.Run(fmt.Sprintf("workers=%d", workers), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				files, err := scanDir(context.Background(), root, workers)
				if err != nil {
					b.Fatal(err)
				}
				if len(files) != nDirs*filesPerDir {
					b.Fatalf("got %d files, want %d", len(files), nDirs*filesPerDir)
				}
			}
		})
	}
}

// BenchmarkScanDir_Serial is a single-worker baseline for direct comparison
// with the parallel sub-benchmarks above.
func BenchmarkScanDir_Serial(b *testing.B) {
	root := b.TempDir()
	buildSyntheticTree(b, root, 500, 20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := scanDir(context.Background(), root, 1); err != nil {
			b.Fatal(err)
		}
	}
}
