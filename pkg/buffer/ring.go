// Package buffer provides a thread-safe ring buffer implementation
package buffer

import "sync/atomic"

// RingBuffer 线程安全的环形缓冲区实现
type RingBuffer struct {
	buf    []byte
	size   int
	r, w   int32
	count  int32
	closed int32
}

// New 创建新的环形缓冲区
func New(size int) *RingBuffer {
	return &RingBuffer{
		buf:  make([]byte, size),
		size: size,
	}
}

// Write 写入数据到缓冲区
// 返回实际写入的字节数
func (rb *RingBuffer) Write(data []byte) int {
	if atomic.LoadInt32(&rb.closed) == 1 {
		return 0
	}

	total := 0
	for len(data) > 0 {
		// 原子获取当前状态
		r := atomic.LoadInt32(&rb.r)
		w := atomic.LoadInt32(&rb.w)
		count := atomic.LoadInt32(&rb.count)

		// 计算可用空间
		avail := rb.size - int(count)
		if avail == 0 {
			break // 缓冲区已满
		}

		var toWrite int
		if w < r {
			// 写入区域在读取区域之前
			toWrite = min(len(data), int(r)-int(w))
		} else {
			// 写入区域在读取区域之后
			toWrite = min(len(data), rb.size-int(w))
			if toWrite == 0 && r > 0 {
				// 如果尾部空间不足，但头部有空间
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

// Read 从缓冲区读取数据
// 返回实际读取的字节数和是否已关闭
func (rb *RingBuffer) Read(out []byte) (int, bool) {
	if atomic.LoadInt32(&rb.closed) == 1 && atomic.LoadInt32(&rb.count) == 0 {
		return 0, true // 缓冲区已关闭且无数据
	}

	total := 0
	for len(out) > 0 {
		// 原子获取当前状态
		r := atomic.LoadInt32(&rb.r)
		w := atomic.LoadInt32(&rb.w)
		count := atomic.LoadInt32(&rb.count)

		if count <= 0 {
			break // 无数据可读
		}

		var toRead int
		if r < w {
			// 读取区域在写入区域之前
			toRead = min(len(out), int(w)-int(r))
		} else {
			// 读取区域在写入区域之后
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

// Length 返回当前缓冲区中的数据长度
func (rb *RingBuffer) Length() int {
	return int(atomic.LoadInt32(&rb.count))
}

// Close 关闭缓冲区
func (rb *RingBuffer) Close() {
	atomic.StoreInt32(&rb.closed, 1)
}

// IsClosed 检查缓冲区是否已关闭
func (rb *RingBuffer) IsClosed() bool {
	return atomic.LoadInt32(&rb.closed) == 1
}

// IsEmpty 检查缓冲区是否为空
func (rb *RingBuffer) IsEmpty() bool {
	return atomic.LoadInt32(&rb.count) == 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
