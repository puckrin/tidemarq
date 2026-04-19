package delta

import (
	"io"
)

// OpType distinguishes the two kinds of delta operations.
type OpType uint8

const (
	// OpCopy means "copy Length bytes from the basis file starting at Offset".
	OpCopy OpType = 1
	// OpLiteral means "insert the bytes in Data verbatim".
	OpLiteral OpType = 2
)

// Op is a single element of a delta instruction stream.
type Op struct {
	Type   OpType
	Offset int64  // OpCopy: byte offset in the basis file
	Length int32  // OpCopy: byte count to copy
	Data   []byte // OpLiteral: raw bytes to insert
}

// DiffStats summarises what happened during a Diff call.
type DiffStats struct {
	CopyBytes    int64 // bytes covered by OpCopy instructions
	LiteralBytes int64 // bytes carried as OpLiteral data
	OpsTotal     int
}

// LiteralFraction returns the fraction of source bytes that became literals
// (0.0 = perfect deduplication, 1.0 = no match at all).
func (s DiffStats) LiteralFraction() float64 {
	total := s.CopyBytes + s.LiteralBytes
	if total == 0 {
		return 0
	}
	return float64(s.LiteralBytes) / float64(total)
}

// Diff reads the source from r and produces a minimal sequence of Ops that,
// when applied via Apply to the basis file described by sig, reproduces the
// source exactly.
//
// The algorithm:
//  1. Slide a blockSize window across the source.
//  2. At each position compute the rolling Adler-32.
//  3. If the weak checksum is known in sig, compute the strong (BLAKE3) hash
//     of the window and do a full lookup.
//  4. On a match: flush accumulated literals, emit OpCopy, skip blockSize bytes.
//  5. No match: accumulate the current byte as a literal, advance by one.
//
// Consecutive literal bytes are coalesced into a single OpLiteral to keep the
// op stream compact.
func Diff(r io.Reader, sig *Signature) ([]Op, DiffStats, error) {
	blockSize := sig.BlockSize
	if blockSize <= 0 {
		blockSize = DefaultBlockSize
	}

	// Read the entire source into memory. For the file sizes where delta
	// transfer is useful (>= DeltaMinBytes, typically 64 KB–several GB) this is
	// acceptable; extremely large files should increase the block size to keep
	// the window manageable. A streaming implementation could replace this but
	// adds significant complexity for negligible practical gain.
	src, err := io.ReadAll(r)
	if err != nil {
		return nil, DiffStats{}, err
	}

	var ops []Op
	var stats DiffStats
	var literals []byte

	flushLiterals := func() {
		if len(literals) == 0 {
			return
		}
		ops = append(ops, Op{Type: OpLiteral, Data: literals})
		stats.LiteralBytes += int64(len(literals))
		stats.OpsTotal++
		literals = nil
	}

	rolling := NewRolling(blockSize)
	i := 0

	// Seed the rolling window with the first full block so that subsequent
	// misses only need a single Roll() call rather than a full Reset+Write.
	if len(src) >= blockSize {
		rolling.Write(src[0:blockSize])
	}

	for i < len(src) {
		// Can we form a full block starting at i?
		if i+blockSize > len(src) {
			// Remaining bytes are smaller than one block.
			// Try to match them against the signature's own partial last block
			// (which may be shorter than blockSize) before falling back to literals.
			remaining := src[i:]
			weak := adler32Block(remaining)
			strong := blake3strong(remaining)
			matched := false
			for _, idx := range sig.index[weak] {
				b := &sig.Blocks[idx]
				if int(b.Length) == len(remaining) && b.Strong == strong {
					flushLiterals()
					ops = append(ops, Op{Type: OpCopy, Offset: b.Offset, Length: b.Length})
					stats.CopyBytes += int64(b.Length)
					stats.OpsTotal++
					matched = true
					break
				}
			}
			if !matched {
				literals = append(literals, remaining...)
			}
			break
		}

		// rolling.Sum32() always represents src[i:i+blockSize] at this point.
		weak := rolling.Sum32()

		// Quick weak-hash check before the more expensive strong hash.
		if _, exists := sig.index[weak]; exists {
			block := src[i : i+blockSize]
			strong := blake3strong(block)
			if match := sig.lookup(weak, strong); match != nil {
				flushLiterals()
				// Coalesce adjacent copies from the same region if possible.
				if len(ops) > 0 {
					prev := &ops[len(ops)-1]
					if prev.Type == OpCopy &&
						prev.Offset+int64(prev.Length) == match.Offset &&
						int64(prev.Length)+int64(match.Length) <= 1<<30 {
						prev.Length += match.Length
						stats.CopyBytes += int64(match.Length)
						i += blockSize
						// Re-seed for the new block position.
						rolling.Reset()
						if i+blockSize <= len(src) {
							rolling.Write(src[i : i+blockSize])
						}
						continue
					}
				}
				ops = append(ops, Op{
					Type:   OpCopy,
					Offset: match.Offset,
					Length: match.Length,
				})
				stats.CopyBytes += int64(match.Length)
				stats.OpsTotal++
				i += blockSize
				// Re-seed for the new block position.
				rolling.Reset()
				if i+blockSize <= len(src) {
					rolling.Write(src[i : i+blockSize])
				}
				continue
			}
		}

		// No match — accumulate as literal and slide the window one byte forward.
		// Roll() evicts src[i] and brings in src[i+blockSize], giving the checksum
		// of src[i+1:i+1+blockSize] ready for the next iteration. This is O(1)
		// versus the previous Reset+Write which was O(blockSize) per miss.
		literals = append(literals, src[i])
		i++
		if i+blockSize-1 < len(src) {
			rolling.Roll(src[i+blockSize-1])
		}
	}

	flushLiterals()
	return ops, stats, nil
}
