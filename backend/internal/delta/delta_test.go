package delta

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── Rolling checksum ──────────────────────────────────────────────────────────

func TestAdler32BlockKnownValue(t *testing.T) {
	// Adler-32 of "Wikipedia" is 0x11E60398 (from the spec).
	got := adler32Block([]byte("Wikipedia"))
	if got != 0x11E60398 {
		t.Fatalf("adler32Block(\"Wikipedia\") = %#x, want 0x11E60398", got)
	}
}

func TestRollingMatchesBlock(t *testing.T) {
	data := []byte("hello world this is a test block")
	blockSize := len(data)

	r := NewRolling(blockSize)
	r.Write(data)

	want := adler32Block(data)
	if r.Sum32() != want {
		t.Fatalf("rolling %#x != block %#x", r.Sum32(), want)
	}
}

func TestRollingSlide(t *testing.T) {
	// Feed 2*blockSize bytes, verify the rolling checksum matches the
	// last blockSize bytes.
	blockSize := 8
	data := []byte("ABCDEFGHIJKLMNOP") // 16 bytes

	r := NewRolling(blockSize)
	for _, b := range data {
		r.Roll(b)
	}

	// After sliding through all 16 bytes the window holds the last 8.
	want := adler32Block(data[8:])
	if r.Sum32() != want {
		t.Fatalf("sliding window %#x != expected %#x", r.Sum32(), want)
	}
}

// ── Signature ─────────────────────────────────────────────────────────────────

func TestComputeSignatureBlockCount(t *testing.T) {
	// 3 full blocks + 1 partial block → 4 BlockSigs
	blockSize := 16
	data := make([]byte, 3*blockSize+7)
	for i := range data {
		data[i] = byte(i)
	}

	sig, err := ComputeSignature(bytes.NewReader(data), blockSize)
	if err != nil {
		t.Fatal(err)
	}
	if len(sig.Blocks) != 4 {
		t.Fatalf("got %d blocks, want 4", len(sig.Blocks))
	}
	// Last block should be partial.
	if sig.Blocks[3].Length != 7 {
		t.Fatalf("last block length = %d, want 7", sig.Blocks[3].Length)
	}
}

func TestComputeSignatureOffsets(t *testing.T) {
	blockSize := 10
	data := make([]byte, 30)
	sig, _ := ComputeSignature(bytes.NewReader(data), blockSize)

	for i, b := range sig.Blocks {
		want := int64(i * blockSize)
		if b.Offset != want {
			t.Errorf("block %d: offset %d, want %d", i, b.Offset, want)
		}
	}
}

func TestSignatureLookupHit(t *testing.T) {
	blockSize := 8
	data := []byte("ABCDEFGHIJKLMNOP") // 2 blocks

	sig, _ := ComputeSignature(bytes.NewReader(data), blockSize)

	weak := adler32Block(data[:blockSize])
	strong := blake3strong(data[:blockSize])

	match := sig.lookup(weak, strong)
	if match == nil {
		t.Fatal("expected match, got nil")
	}
	if match.Offset != 0 {
		t.Fatalf("match.Offset = %d, want 0", match.Offset)
	}
}

func TestSignatureLookupMiss(t *testing.T) {
	blockSize := 8
	data := []byte("ABCDEFGHIJKLMNOP")
	sig, _ := ComputeSignature(bytes.NewReader(data), blockSize)

	// Block of all zeros won't match anything in the signature.
	block := make([]byte, blockSize)
	weak := adler32Block(block)
	strong := blake3strong(block)

	if sig.lookup(weak, strong) != nil {
		t.Fatal("expected no match for zero block")
	}
}

// ── Diff ──────────────────────────────────────────────────────────────────────

func TestDiffIdentical(t *testing.T) {
	// Diffing a file against itself should produce only OpCopy ops.
	data := []byte(strings.Repeat("the quick brown fox jumps over a lazy dog ", 100))
	blockSize := 32

	sig, _ := ComputeSignature(bytes.NewReader(data), blockSize)
	ops, stats, err := Diff(bytes.NewReader(data), sig)
	if err != nil {
		t.Fatal(err)
	}

	for _, op := range ops {
		if op.Type == OpLiteral {
			t.Errorf("unexpected literal op with %d bytes", len(op.Data))
		}
	}
	if stats.LiteralBytes != 0 {
		t.Errorf("literal bytes = %d, want 0", stats.LiteralBytes)
	}
}

func TestDiffCompletelyDifferent(t *testing.T) {
	// Two completely unrelated random buffers: everything is a literal.
	blockSize := 32
	basis := make([]byte, 512)
	source := make([]byte, 512)
	if _, err := rand.Read(basis); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(source); err != nil {
		t.Fatal(err)
	}

	sig, _ := ComputeSignature(bytes.NewReader(basis), blockSize)
	_, stats, err := Diff(bytes.NewReader(source), sig)
	if err != nil {
		t.Fatal(err)
	}

	if stats.CopyBytes > 0 {
		t.Logf("copy bytes = %d (small collision probability — acceptable)", stats.CopyBytes)
	}
	if stats.LiteralBytes+stats.CopyBytes != int64(len(source)) {
		t.Fatalf("bytes accounted for = %d, want %d",
			stats.LiteralBytes+stats.CopyBytes, len(source))
	}
}

func TestDiffAppend(t *testing.T) {
	// Source = basis + 20 extra bytes.
	basis := []byte(strings.Repeat("data block content. ", 20)) // 400 bytes
	appended := []byte(" extra bytes appended here!!")
	source := append(append([]byte{}, basis...), appended...)

	blockSize := 20
	sig, _ := ComputeSignature(bytes.NewReader(basis), blockSize)
	_, stats, err := Diff(bytes.NewReader(source), sig)
	if err != nil {
		t.Fatal(err)
	}

	if stats.CopyBytes < int64(len(basis)) {
		t.Errorf("copy bytes = %d, expected at least %d (all of basis)", stats.CopyBytes, len(basis))
	}
	// The appended region must show up as literals.
	if stats.LiteralBytes == 0 {
		t.Error("expected some literal bytes for the appended region")
	}
}

// ── Patch ─────────────────────────────────────────────────────────────────────

func TestApplyRoundtrip(t *testing.T) {
	dir := t.TempDir()
	basisPath := filepath.Join(dir, "basis.dat")
	destPath := filepath.Join(dir, "dest.dat")

	basis := []byte(strings.Repeat("hello world ", 50))
	if err := os.WriteFile(basisPath, basis, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destPath, basis, 0o644); err != nil { // dest starts identical
		t.Fatal(err)
	}

	// Modify middle portion.
	source := append([]byte{}, basis...)
	copy(source[100:120], []byte("CHANGED_CONTENT!!!!!"))

	sig, _ := ComputeSignature(bytes.NewReader(basis), 16)
	ops, _, err := Diff(bytes.NewReader(source), sig)
	if err != nil {
		t.Fatal(err)
	}

	if err := Apply(basisPath, destPath, ops); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, source) {
		t.Fatalf("after Apply, dest does not match source\ngot len=%d want len=%d", len(got), len(source))
	}
}

func TestApplyEmptyOps(t *testing.T) {
	// Applying zero ops should produce an empty file.
	dir := t.TempDir()
	basisPath := filepath.Join(dir, "basis.dat")
	destPath := filepath.Join(dir, "dest.dat")

	if err := os.WriteFile(basisPath, []byte("some content"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destPath, []byte("some content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Apply(basisPath, destPath, nil); err != nil {
		t.Fatalf("Apply with nil ops: %v", err)
	}

	got, _ := os.ReadFile(destPath)
	if len(got) != 0 {
		t.Fatalf("expected empty file after applying zero ops, got %d bytes", len(got))
	}
}

func TestApplyPreservesOriginalOnError(t *testing.T) {
	dir := t.TempDir()
	basisPath := filepath.Join(dir, "nonexistent.dat")
	destPath := filepath.Join(dir, "dest.dat")

	original := []byte("original content")
	if err := os.WriteFile(destPath, original, 0o644); err != nil {
		t.Fatal(err)
	}

	// basis doesn't exist — Apply must fail.
	err := Apply(basisPath, destPath, []Op{{Type: OpCopy, Offset: 0, Length: 5}})
	if err == nil {
		t.Fatal("expected error when basis file is missing")
	}

	// dest must be untouched.
	got, _ := os.ReadFile(destPath)
	if !bytes.Equal(got, original) {
		t.Fatal("dest was modified despite Apply failing")
	}
}

// ── End-to-end ────────────────────────────────────────────────────────────────

func TestEndToEnd_SmallEdit(t *testing.T) {
	// Full pipeline: basis file, source with small change, verify dest matches source.
	dir := t.TempDir()
	basisPath := filepath.Join(dir, "v1.dat")
	destPath := filepath.Join(dir, "v2.dat")

	// ~4 KB of structured data.
	var buf bytes.Buffer
	for i := 0; i < 200; i++ {
		buf.WriteString("line of content number ")
		buf.WriteByte(byte('0' + i%10))
		buf.WriteString(" padding padding padding\n")
	}
	basis := buf.Bytes()

	source := append([]byte{}, basis...)
	// Overwrite bytes 500–520 (middle of the file).
	copy(source[500:], []byte("** MODIFIED REGION **"))

	if err := os.WriteFile(basisPath, basis, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destPath, basis, 0o644); err != nil {
		t.Fatal(err)
	}

	sig, err := ComputeSignatureFile(basisPath, 64)
	if err != nil {
		t.Fatal(err)
	}

	ops, stats, err := Diff(bytes.NewReader(source), sig)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("copy=%.0f%% literal=%.0f%%",
		(1-stats.LiteralFraction())*100, stats.LiteralFraction()*100)

	if stats.LiteralFraction() > 0.1 {
		t.Errorf("literal fraction = %.2f, expected < 0.10 for a small edit", stats.LiteralFraction())
	}

	if err := Apply(basisPath, destPath, ops); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(destPath)
	if !bytes.Equal(got, source) {
		t.Fatal("end-to-end: dest does not match source after apply")
	}
}

func TestEndToEnd_Idempotent(t *testing.T) {
	// Applying a delta twice (basis is already the target) must be a no-op in
	// terms of content. Running the diff will produce only OpCopy instructions
	// and the result must equal the source.
	dir := t.TempDir()
	path := filepath.Join(dir, "file.dat")

	data := []byte(strings.Repeat("idempotent sync content line\n", 50))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	sig, _ := ComputeSignatureFile(path, 32)
	ops, stats, _ := Diff(bytes.NewReader(data), sig)

	if stats.LiteralBytes != 0 {
		t.Errorf("idempotent diff produced %d literal bytes", stats.LiteralBytes)
	}

	if err := Apply(path, path, ops); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, data) {
		t.Fatal("content changed after idempotent apply")
	}
}

// ── SizeBytes ─────────────────────────────────────────────────────────────────

func TestSizeBytes(t *testing.T) {
	ops := []Op{
		{Type: OpCopy, Offset: 0, Length: 100},
		{Type: OpLiteral, Data: make([]byte, 50)},
		{Type: OpCopy, Offset: 200, Length: 75},
	}
	got := SizeBytes(ops)
	if got != 225 {
		t.Fatalf("SizeBytes = %d, want 225", got)
	}
}
