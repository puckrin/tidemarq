package hasher_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tidemarq/tidemarq/internal/hasher"
)

func TestNew_UnknownAlgo(t *testing.T) {
	_, err := hasher.New("md5")
	if err == nil {
		t.Fatal("expected error for unknown algorithm")
	}
}

func TestHashReader_SHA256(t *testing.T) {
	// echo -n "hello" | sha256sum = 2cf24dba...
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	got, err := hasher.HashReader(hasher.SHA256, strings.NewReader("hello"))
	if err != nil {
		t.Fatalf("HashReader: %v", err)
	}
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestHashReader_Blake3(t *testing.T) {
	// Two calls with the same input must return the same hash.
	h1, err := hasher.HashReader(hasher.Blake3, strings.NewReader("hello world"))
	if err != nil {
		t.Fatalf("first HashReader: %v", err)
	}
	h2, err := hasher.HashReader(hasher.Blake3, strings.NewReader("hello world"))
	if err != nil {
		t.Fatalf("second HashReader: %v", err)
	}
	if h1 != h2 {
		t.Errorf("non-deterministic: %s vs %s", h1, h2)
	}
	// Must differ from SHA-256 of the same input.
	hSHA, _ := hasher.HashReader(hasher.SHA256, strings.NewReader("hello world"))
	if h1 == hSHA {
		t.Error("blake3 and sha256 returned the same hash")
	}
}

func TestHashReader_DefaultIsSHA256(t *testing.T) {
	hDef, err := hasher.HashReader("", strings.NewReader("test"))
	if err != nil {
		t.Fatalf("HashReader default: %v", err)
	}
	hSHA, _ := hasher.HashReader(hasher.SHA256, strings.NewReader("test"))
	if hDef != hSHA {
		t.Errorf("default should be sha256: got %s, want %s", hDef, hSHA)
	}
}

func TestHashFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("file content"), 0o644); err != nil {
		t.Fatal(err)
	}

	h1, err := hasher.HashFile(hasher.SHA256, path)
	if err != nil {
		t.Fatalf("HashFile sha256: %v", err)
	}
	h2, err := hasher.HashFile(hasher.Blake3, path)
	if err != nil {
		t.Fatalf("HashFile blake3: %v", err)
	}
	if h1 == h2 {
		t.Error("sha256 and blake3 of the same file should differ")
	}
	if len(h1) != 64 { // sha256 = 32 bytes = 64 hex chars
		t.Errorf("sha256 hex length: got %d, want 64", len(h1))
	}
	if len(h2) != 64 { // blake3 default = 32 bytes = 64 hex chars
		t.Errorf("blake3 hex length: got %d, want 64", len(h2))
	}
}
