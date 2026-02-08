// Package buffer provides a thread-safe ring buffer implementation
package buffer

import "sync/atomic"

// RingBuffer is a thread-safe ring buffer implementation
type RingBuffer struct {
	buf    []byte
	size   int
	r, w   int32
	count  int32
	closed int32
}

// New creates a new ring buffer
func New(size int) *RingBuffer {
	return &RingBuffer{
		buf:  make([]byte, size),
		size: size,
	}
}

// Write writes data to the buffer
// Returns the number of bytes actually written
func (rb *RingBuffer) Write(data []byte) int {
	if atomic.LoadInt32(&rb.closed) == 1 {
		return 0
	}

	total := 0
	for len(data) > 0 {
		// Atomically get current state
		r := atomic.LoadInt32(&rb.r)
		w := atomic.LoadInt32(&rb.w)
		count := atomic.LoadInt32(&rb.count)

		// Calculate available space
		avail := rb.size - int(count)
		if avail == 0 {
			break // Buffer is full
		}

		var toWrite int
		if w < r {
			// Write region is before read region
			toWrite = min(len(data), int(r)-int(w))
		} else {
			// Write region is after read region
			toWrite = min(len(data), rb.size-int(w))
			if toWrite == 0 && r > 0 {
				// If tail space is insufficient but head has space
				atomic.StoreInt32(&rb.w, 0)
				w = 0
				toWrite = min(len(data), int(r))
			}
		}

		if toWrite == 0 {
			break
		}

		copy(rb.buf[w:], data[:toWrite])
		newW := (w + int32(toWrite)) % int32(rb.size)
		atomic.StoreInt32(&rb.w, newW)
		atomic.AddInt32(&rb.count, int32(toWrite))

		data = data[toWrite:]
		total += toWrite
	}
	return total
}

// Read reads data from the buffer
// Returns the number of bytes actually read and whether the buffer is closed
func (rb *RingBuffer) Read(out []byte) (int, bool) {
	if atomic.LoadInt32(&rb.closed) == 1 && atomic.LoadInt32(&rb.count) == 0 {
		return 0, true // Buffer is closed and has no data
	}

	total := 0
	for len(out) > 0 {
		// Atomically get current state
		r := atomic.LoadInt32(&rb.r)
		w := atomic.LoadInt32(&rb.w)
		count := atomic.LoadInt32(&rb.count)

		if count <= 0 {
			break // No data to read
		}

		var toRead int
		if r < w {
			// Read region is before write region
			toRead = min(len(out), int(w)-int(r))
		} else {
			// Read region is after write region
			toRead = min(len(out), rb.size-int(r))
		}

		if toRead == 0 {
			break
		}

		copy(out, rb.buf[r:r+int32(toRead)])
		newR := (r + int32(toRead)) % int32(rb.size)
		atomic.StoreInt32(&rb.r, newR)
		atomic.AddInt32(&rb.count, int32(-toRead))

		out = out[toRead:]
		total += toRead
	}

	closed := atomic.LoadInt32(&rb.closed) == 1 && atomic.LoadInt32(&rb.count) == 0
	return total, closed
}

// Length returns the current data length in the buffer
func (rb *RingBuffer) Length() int {
	return int(atomic.LoadInt32(&rb.count))
}

// Close closes the buffer
func (rb *RingBuffer) Close() {
	atomic.StoreInt32(&rb.closed, 1)
}

// IsClosed checks if the buffer is closed
func (rb *RingBuffer) IsClosed() bool {
	return atomic.LoadInt32(&rb.closed) == 1
}

// IsEmpty checks if the buffer is empty
func (rb *RingBuffer) IsEmpty() bool {
	return atomic.LoadInt32(&rb.count) == 0
}

// Clear clears all data in the buffer
func (rb *RingBuffer) Clear() {
	atomic.StoreInt32(&rb.r, 0)
	atomic.StoreInt32(&rb.w, 0)
	atomic.StoreInt32(&rb.count, 0)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
