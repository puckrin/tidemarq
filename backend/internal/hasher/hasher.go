// Package hasher provides a registry of hash algorithms for file integrity checking.
// Supported algorithms: "sha256", "blake3".
//
// File-level integrity hashing (manifest, versions, conflicts) uses the algorithm
// configured per job via engine.Config.HashAlgo.  Chunk-level hashing for CDC
// deduplication uses BLAKE3 internally and does not go through this package.
package hasher

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"

	"github.com/zeebo/blake3"
)

const (
	SHA256 = "sha256"
	Blake3 = "blake3"

	// Default is the algorithm used when none is specified for new jobs.
	Default = Blake3
)

// New returns a new hash.Hash for the named algorithm.
// Returns an error for unknown algorithm names.
func New(algo string) (hash.Hash, error) {
	switch algo {
	case SHA256, "":
		return sha256.New(), nil
	case Blake3:
		return blake3.New(), nil
	default:
		return nil, fmt.Errorf("hasher: unknown algorithm %q", algo)
	}
}

// HashReader hashes all bytes from r using the named algorithm and returns a
// lowercase hex string. The caller is responsible for closing r if needed.
func HashReader(algo string, r io.Reader) (string, error) {
	h, err := New(algo)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashFile hashes the file at path using the named algorithm and returns a
// lowercase hex string.
func HashFile(algo, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return HashReader(algo, f)
}
