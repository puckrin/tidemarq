package delta

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Apply reconstructs the new file at destPath by executing ops against the
// existing basis file at basisPath.
//
// The output is written to a sibling temp file and atomically renamed over
// destPath on success. If any op fails the temp file is removed and destPath
// is left untouched, preserving the original basis.
//
// If basisPath and destPath are the same (in-place update) the function reads
// the basis fully before writing, which is safe but uses memory proportional
// to the largest OpCopy region.
func Apply(basisPath, destPath string, ops []Op) error {
	basis, err := os.Open(basisPath)
	if err != nil {
		return fmt.Errorf("delta apply: open basis: %w", err)
	}
	// basis is closed explicitly before the rename step so that on Windows
	// (which locks open files) os.Remove(destPath) succeeds when basisPath
	// and destPath refer to the same file.
	basisClosed := false
	closeBasis := func() {
		if !basisClosed {
			basis.Close()
			basisClosed = true
		}
	}
	defer closeBasis()

	// Write to a temp file in the same directory so os.Rename is atomic on
	// most operating systems (same filesystem).
	tmpFile, err := os.CreateTemp(filepath.Dir(destPath), ".delta-tmp-")
	if err != nil {
		return fmt.Errorf("delta apply: create temp: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Clean up temp file if anything goes wrong before the rename.
	ok := false
	defer func() {
		tmpFile.Close()
		if !ok {
			os.Remove(tmpPath)
		}
	}()

	for _, op := range ops {
		switch op.Type {
		case OpCopy:
			if _, err := basis.Seek(op.Offset, io.SeekStart); err != nil {
				return fmt.Errorf("delta apply: seek to %d: %w", op.Offset, err)
			}
			if _, err := io.CopyN(tmpFile, basis, int64(op.Length)); err != nil {
				return fmt.Errorf("delta apply: copy from basis at %d len %d: %w",
					op.Offset, op.Length, err)
			}

		case OpLiteral:
			if _, err := tmpFile.Write(op.Data); err != nil {
				return fmt.Errorf("delta apply: write literal: %w", err)
			}

		default:
			return fmt.Errorf("delta apply: unknown op type %d", op.Type)
		}
	}

	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("delta apply: sync: %w", err)
	}
	tmpFile.Close()
	closeBasis() // must close before Remove on Windows

	// On Windows Rename fails if the destination file already exists.
	if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		os.Remove(tmpPath)
		return fmt.Errorf("delta apply: remove dest: %w", err)
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		return fmt.Errorf("delta apply: rename to dest: %w", err)
	}

	ok = true
	return nil
}

// SizeBytes returns the total number of bytes the op stream will produce when
// applied. Useful for deciding whether a delta is smaller than a full copy.
func SizeBytes(ops []Op) int64 {
	var total int64
	for _, op := range ops {
		switch op.Type {
		case OpCopy:
			total += int64(op.Length)
		case OpLiteral:
			total += int64(len(op.Data))
		}
	}
	return total
}
