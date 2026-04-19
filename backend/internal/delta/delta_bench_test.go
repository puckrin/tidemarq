package delta

import (
	"bytes"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

// deterministicData returns n bytes of seeded pseudo-random data.
func deterministicData(seed int64, n int) []byte {
	r := rand.New(rand.NewSource(seed))
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(r.Intn(256))
	}
	return b
}

// patchedData returns a copy of basis with fraction of bytes randomly modified.
func patchedData(basis []byte, fraction float64, seed int64) []byte {
	out := append([]byte{}, basis...)
	r := rand.New(rand.NewSource(seed))
	n := int(float64(len(basis)) * fraction)
	for i := 0; i < n; i++ {
		pos := r.Intn(len(out))
		out[pos] = byte(r.Intn(256))
	}
	return out
}

// ── Signature generation ──────────────────────────────────────────────────────

func benchmarkSignature(b *testing.B, size int) {
	b.Helper()
	data := deterministicData(1, size)
	b.SetBytes(int64(size))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ComputeSignature(bytes.NewReader(data), DefaultBlockSize); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSignature_10MB(b *testing.B)  { benchmarkSignature(b, 10<<20) }
func BenchmarkSignature_100MB(b *testing.B) { benchmarkSignature(b, 100<<20) }

// ── Diff (delta instruction generation) ──────────────────────────────────────

type changeCase struct {
	name     string
	fraction float64
}

var changeCases = []changeCase{
	{"0pct_change", 0.0},
	{"5pct_change", 0.05},
	{"100pct_change", 1.0},
}

func benchmarkDiff(b *testing.B, size int) {
	b.Helper()
	basis := deterministicData(1, size)
	sig, err := ComputeSignature(bytes.NewReader(basis), DefaultBlockSize)
	if err != nil {
		b.Fatal(err)
	}

	for _, tc := range changeCases {
		tc := tc
		source := patchedData(basis, tc.fraction, 2)
		b.Run(tc.name, func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, stats, err := Diff(bytes.NewReader(source), sig)
				if err != nil {
					b.Fatal(err)
				}
				// Verify the 10% fallback threshold fires correctly on the identical case.
				if tc.fraction == 0.0 && stats.LiteralFraction() > 0.001 {
					b.Fatalf("0%% change: literal fraction = %.4f, expected ~0", stats.LiteralFraction())
				}
			}
		})
	}
}

func BenchmarkDiff_10MB(b *testing.B)  { benchmarkDiff(b, 10<<20) }
func BenchmarkDiff_100MB(b *testing.B) { benchmarkDiff(b, 100<<20) }

// ── Apply (patch reconstruction) ─────────────────────────────────────────────

func benchmarkApply(b *testing.B, size int, fraction float64) {
	b.Helper()
	dir := b.TempDir()
	basisPath := filepath.Join(dir, "basis.dat")
	destPath := filepath.Join(dir, "dest.dat")

	basis := deterministicData(1, size)
	source := patchedData(basis, fraction, 2)

	if err := os.WriteFile(basisPath, basis, 0o644); err != nil {
		b.Fatal(err)
	}

	sig, err := ComputeSignature(bytes.NewReader(basis), DefaultBlockSize)
	if err != nil {
		b.Fatal(err)
	}
	ops, _, err := Diff(bytes.NewReader(source), sig)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(size))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Reset dest to basis before each apply so the benchmark is repeatable.
		if err := os.WriteFile(destPath, basis, 0o644); err != nil {
			b.Fatal(err)
		}
		if err := Apply(basisPath, destPath, ops); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkApply_10MB_0pct(b *testing.B)   { benchmarkApply(b, 10<<20, 0.0) }
func BenchmarkApply_10MB_5pct(b *testing.B)   { benchmarkApply(b, 10<<20, 0.05) }
func BenchmarkApply_100MB_0pct(b *testing.B)  { benchmarkApply(b, 100<<20, 0.0) }
func BenchmarkApply_100MB_5pct(b *testing.B)  { benchmarkApply(b, 100<<20, 0.05) }

// ── Savings ratio (logged, not asserted) ─────────────────────────────────────

// BenchmarkSavingsRatio reports literal fraction for each change level.
// Run with -v to see the logged output.
func BenchmarkSavingsRatio_10MB(b *testing.B) {
	size := 10 << 20
	basis := deterministicData(1, size)
	sig, _ := ComputeSignature(bytes.NewReader(basis), DefaultBlockSize)

	for _, tc := range changeCases {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			source := patchedData(basis, tc.fraction, 2)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, stats, _ := Diff(bytes.NewReader(source), sig)
				if i == 0 {
					b.Logf("literal=%.1f%% copy=%.1f%%",
						stats.LiteralFraction()*100,
						(1-stats.LiteralFraction())*100)
				}
			}
		})
	}
}
