package delta

import (
	"io"
	"os"

	"github.com/zeebo/blake3"
)

const (
	// DefaultBlockSize is used when no explicit block size is configured.
	// 2 KiB matches the rsync default and gives good granularity for most files.
	DefaultBlockSize = 2048

	// strongHashLen is the number of BLAKE3 bytes stored per block.
	// 16 bytes gives 128-bit collision resistance — sufficient for block matching;
	// we do not need preimage resistance here.
	strongHashLen = 16
)

// BlockSig is the signature of one fixed-size block within the basis file.
type BlockSig struct {
	Offset int64    // byte offset of the block in the basis file
	Length int32    // actual byte count (last block may be smaller than BlockSize)
	Weak   uint32   // Adler-32 of the block bytes
	Strong [strongHashLen]byte // truncated BLAKE3 of the block bytes
}

// Signature holds the complete signature of a basis file and a pre-built
// lookup table for the diff phase.
type Signature struct {
	BlockSize int
	Blocks    []BlockSig

	// index maps weak checksum → slice of indices into Blocks.
	// Built once by buildIndex; multiple blocks may share a weak checksum
	// (hash collision), so strong verification is always required.
	index map[uint32][]int
}

// ComputeSignature reads r and produces a Signature with one BlockSig per
// blockSize-byte chunk. r must be positioned at the start of the basis file.
func ComputeSignature(r io.Reader, blockSize int) (*Signature, error) {
	if blockSize <= 0 {
		blockSize = DefaultBlockSize
	}

	buf := make([]byte, blockSize)
	sig := &Signature{BlockSize: blockSize}
	var offset int64

	for {
		n, err := io.ReadFull(r, buf)
		if n > 0 {
			block := buf[:n]
			weak := adler32Block(block)
			strong := blake3strong(block)
			sig.Blocks = append(sig.Blocks, BlockSig{
				Offset: offset,
				Length: int32(n),
				Weak:   weak,
				Strong: strong,
			})
			offset += int64(n)
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	sig.buildIndex()
	return sig, nil
}

// ComputeSignatureFile is a convenience wrapper that opens path and calls
// ComputeSignature.
func ComputeSignatureFile(path string, blockSize int) (*Signature, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ComputeSignature(f, blockSize)
}

// buildIndex constructs the weak→[]blockIndex lookup table.
func (s *Signature) buildIndex() {
	s.index = make(map[uint32][]int, len(s.Blocks))
	for i, b := range s.Blocks {
		s.index[b.Weak] = append(s.index[b.Weak], i)
	}
}

// lookup returns the BlockSig that matches both weak and strong checksums, or
// nil if no match exists.
func (s *Signature) lookup(weak uint32, strong [strongHashLen]byte) *BlockSig {
	for _, idx := range s.index[weak] {
		if s.Blocks[idx].Strong == strong {
			return &s.Blocks[idx]
		}
	}
	return nil
}

// blake3strong returns the first strongHashLen bytes of the BLAKE3 hash of p.
func blake3strong(p []byte) [strongHashLen]byte {
	full := blake3.Sum256(p)
	var out [strongHashLen]byte
	copy(out[:], full[:])
	return out
}
