// Package buffer provides a lock-free SPSC (Single-Producer Single-Consumer) ring buffer.
package buffer

import (
	"sync/atomic"
	"unsafe"
)

// RingBuffer is a lock-free SPSC ring buffer.
//
// The writer goroutine is the sole modifier of w.
// The reader goroutine (PortAudio callback) is the sole modifier of r.
// Available data is computed as w - r (modular arithmetic on int64).
//
// int64 fields are placed first in the struct so that they are 8-byte aligned
// even on 32-bit ARM7 (struct base is always at least pointer-aligned).
type RingBuffer struct {
	// w is the cumulative number of bytes written (only modified by producer).
	w int64
	// r is the cumulative number of bytes read (only modified by consumer).
	r int64

	buf    []byte
	size   int64
	closed int32 // 1 = closed

	// pad prevents false sharing between w and r on separate cache lines.
	// On ARM Cortex-A7 the cache line is 32 or 64 bytes; this is good enough.
	_ [unsafe.Sizeof(int64(0))]byte
}

// New creates a new ring buffer with the given capacity in bytes.
func New(size int) *RingBuffer {
	return &RingBuffer{
		buf:  make([]byte, size),
		size: int64(size),
	}
}

// Write appends data to the buffer. Returns the number of bytes written.
// Only safe to call from a single producer goroutine.
func (rb *RingBuffer) Write(data []byte) int {
	if atomic.LoadInt32(&rb.closed) == 1 {
		return 0
	}

	r := atomic.LoadInt64(&rb.r)
	w := rb.w // producer owns w, no atomic load needed for own writes

	avail := rb.size - (w - r)
	n := int64(len(data))
	if n > avail {
		n = avail
	}
	if n == 0 {
		return 0
	}

	// Write position within the circular buffer.
	pos := w % rb.size
	// First segment: from pos to end of buffer (or n, whichever is smaller).
	first := min(n, rb.size-pos)
	copy(rb.buf[pos:pos+first], data[:first])
	// Second segment: wrap around to beginning.
	if first < n {
		copy(rb.buf[0:n-first], data[first:n])
	}

	// Publish the new write position. The store must be atomic so the
	// consumer sees a consistent value.
	atomic.StoreInt64(&rb.w, w+n)
	return int(n)
}

// Read fills out with available data. Returns the number of bytes read and
// whether the buffer is closed with no remaining data.
// Only safe to call from a single consumer goroutine (the PortAudio callback).
func (rb *RingBuffer) Read(out []byte) (int, bool) {
	w := atomic.LoadInt64(&rb.w)
	r := rb.r // consumer owns r

	avail := w - r
	n := int64(len(out))
	if n > avail {
		n = avail
	}

	if n == 0 {
		closed := atomic.LoadInt32(&rb.closed) == 1
		return 0, closed
	}

	pos := r % rb.size
	first := min(n, rb.size-pos)
	copy(out[:first], rb.buf[pos:pos+first])
	if first < n {
		copy(out[first:n], rb.buf[0:n-first])
	}

	atomic.StoreInt64(&rb.r, r+n)
	closed := atomic.LoadInt32(&rb.closed) == 1 && (r+n) == atomic.LoadInt64(&rb.w)
	return int(n), closed
}

// Length returns the number of unread bytes in the buffer.
func (rb *RingBuffer) Length() int {
	w := atomic.LoadInt64(&rb.w)
	r := atomic.LoadInt64(&rb.r)
	return int(w - r)
}

// Close marks the buffer as closed. Subsequent writes return 0.
// Reads continue to drain remaining data.
func (rb *RingBuffer) Close() {
	atomic.StoreInt32(&rb.closed, 1)
}

// IsClosed reports whether the buffer has been closed.
func (rb *RingBuffer) IsClosed() bool {
	return atomic.LoadInt32(&rb.closed) == 1
}

// IsEmpty reports whether there is no unread data.
func (rb *RingBuffer) IsEmpty() bool {
	return rb.Length() == 0
}

// Clear discards all unread data by advancing the read cursor to the
// current write cursor. Safe to call from the consumer side.
func (rb *RingBuffer) Clear() {
	w := atomic.LoadInt64(&rb.w)
	atomic.StoreInt64(&rb.r, w)
}
