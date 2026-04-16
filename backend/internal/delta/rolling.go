// Package delta implements rolling-checksum delta transfer (rsync algorithm).
// Files are compared block-by-block against an existing destination version;
// only changed regions are transferred as literal bytes, unchanged regions are
// represented as copy-from-destination references. This reduces I/O for large
// files with small changes without requiring a persistent chunk store.
package delta

const (
	modAdler uint32 = 65521 // largest prime smaller than 2^16
)

// Rolling is an incremental Adler-32 checksum that can advance one byte at a
// time in O(1) without re-reading the window. It is used during the diff phase
// to slide a fixed-size window across the source file.
type Rolling struct {
	a, b      uint32
	blockSize int
	window    []byte // circular buffer of the current window
	pos       int    // next write position in window
	count     int    // how many bytes are in the window
}

// NewRolling returns a Rolling checksum initialised with the given block size.
func NewRolling(blockSize int) *Rolling {
	return &Rolling{
		a:         1,
		blockSize: blockSize,
		window:    make([]byte, blockSize),
	}
}

// Reset clears the rolling state without reallocating the window buffer.
func (r *Rolling) Reset() {
	r.a = 1
	r.b = 0
	r.pos = 0
	r.count = 0
}

// Roll advances the window by one byte. If the window is full the oldest byte
// is evicted and the new byte is added.
func (r *Rolling) Roll(in byte) {
	if r.count == r.blockSize {
		// Evict oldest byte from front of window.
		// Correct rolling Adler-32 update:
		//   a_new = a_old - out + in
		//   b_new = b_old + a_new - 1 - n*out
		// Applied in two steps (evict then add) to share the add path:
		//   after evict: a' = a-out,  b' = b - n*out - 1
		//   after add:   a_new = a'+in, b_new = b' + a_new  ✓
		out := r.window[r.pos]
		r.a = (r.a - uint32(out) + modAdler) % modAdler
		r.b = (r.b - (uint32(r.blockSize)*uint32(out))%modAdler - 1 + 2*modAdler) % modAdler
	} else {
		r.count++
	}

	// Add new byte (shared by both initial-fill and slide paths).
	r.window[r.pos] = in
	r.pos = (r.pos + 1) % r.blockSize
	r.a = (r.a + uint32(in)) % modAdler
	r.b = (r.b + r.a) % modAdler
}

// Write feeds a slice of bytes into the rolling checksum.
func (r *Rolling) Write(p []byte) {
	for _, b := range p {
		r.Roll(b)
	}
}

// Sum32 returns the current Adler-32 value for the window.
func (r *Rolling) Sum32() uint32 {
	return (r.b << 16) | r.a
}

// Full returns true when the window holds exactly blockSize bytes.
func (r *Rolling) Full() bool {
	return r.count == r.blockSize
}

// adler32Block computes the Adler-32 checksum of a complete, fixed-size block
// in one pass. Used during signature computation.
func adler32Block(p []byte) uint32 {
	a, b := uint32(1), uint32(0)
	for _, v := range p {
		a = (a + uint32(v)) % modAdler
		b = (b + a) % modAdler
	}
	return (b << 16) | a
}
