package hasher

import (
	"bytes"
	"math/rand"
	"testing"
)

// syntheticData returns n bytes of deterministic pseudo-random data.
func syntheticData(n int) []byte {
	r := rand.New(rand.NewSource(42))
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(r.Intn(256))
	}
	return b
}

func benchmarkHashReader(b *testing.B, algo string, size int) {
	b.Helper()
	data := syntheticData(size)
	b.SetBytes(int64(size))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := HashReader(algo, bytes.NewReader(data)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHashReader_BLAKE3_1KB(b *testing.B)   { benchmarkHashReader(b, Blake3, 1<<10) }
func BenchmarkHashReader_BLAKE3_1MB(b *testing.B)   { benchmarkHashReader(b, Blake3, 1<<20) }
func BenchmarkHashReader_BLAKE3_64MB(b *testing.B)  { benchmarkHashReader(b, Blake3, 64<<20) }
func BenchmarkHashReader_BLAKE3_512MB(b *testing.B) { benchmarkHashReader(b, Blake3, 512<<20) }

func BenchmarkHashReader_SHA256_1KB(b *testing.B)   { benchmarkHashReader(b, SHA256, 1<<10) }
func BenchmarkHashReader_SHA256_1MB(b *testing.B)   { benchmarkHashReader(b, SHA256, 1<<20) }
func BenchmarkHashReader_SHA256_64MB(b *testing.B)  { benchmarkHashReader(b, SHA256, 64<<20) }
func BenchmarkHashReader_SHA256_512MB(b *testing.B) { benchmarkHashReader(b, SHA256, 512<<20) }
