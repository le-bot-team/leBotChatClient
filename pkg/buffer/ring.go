// Package buffer provides a lock-free SPSC (Single-Producer Single-Consumer) ring buffer.
package buffer

import (
	"runtime"
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

	buf     []byte
	size    int64
	closed  int32 // 1 = closed (permanent)
	aborted int32 // 1 = aborted (resettable, breaks blocking Write)

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

// Write appends data to the buffer, blocking until all bytes are written,
// the buffer is closed, or the buffer is aborted. Returns the total number
// of bytes written.
// Only safe to call from a single producer goroutine.
//
// When the buffer is full, Write spins with short sleeps waiting for the
// consumer (PortAudio callback) to drain data. This keeps the reader side
// entirely lock-free while providing back-pressure so no audio data is lost.
func (rb *RingBuffer) Write(data []byte) int {
	total := 0
	remaining := data

	for len(remaining) > 0 {
		if atomic.LoadInt32(&rb.closed) == 1 || atomic.LoadInt32(&rb.aborted) == 1 {
			return total
		}

		r := atomic.LoadInt64(&rb.r)
		w := rb.w // producer owns w

		avail := rb.size - (w - r)
		if avail == 0 {
			// Buffer is full — yield and retry. A short Gosched
			// keeps CPU usage low while the PortAudio callback drains data.
			runtime.Gosched()
			continue
		}

		n := int64(len(remaining))
		if n > avail {
			n = avail
		}

		// Write position within the circular buffer.
		pos := w % rb.size
		// First segment: from pos to end of buffer (or n, whichever is smaller).
		first := min(n, rb.size-pos)
		copy(rb.buf[pos:pos+first], remaining[:first])
		// Second segment: wrap around to beginning.
		if first < n {
			copy(rb.buf[0:n-first], remaining[first:n])
		}

		// Publish the new write position.
		atomic.StoreInt64(&rb.w, w+n)

		total += int(n)
		remaining = remaining[n:]
	}

	return total
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

// Abort sets the abort flag, causing any in-progress or future Write calls to
// return immediately. Unlike Close, Abort is resettable via ResetAbort.
// Use this to break a blocking Write during playback interruption.
func (rb *RingBuffer) Abort() {
	atomic.StoreInt32(&rb.aborted, 1)
}

// ResetAbort clears the abort flag so that Write can block again.
// Typically called after clearing the buffer to prepare for a new session.
func (rb *RingBuffer) ResetAbort() {
	atomic.StoreInt32(&rb.aborted, 0)
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
