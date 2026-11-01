package agent

import (
	"sync"
)

// RingBuffer is a thread-safe circular bounded byte buffer.
// When written data exceeds its capacity, the oldest data is dropped,
// preserving the most recent tail of output up to capacity bytes.
// This prevents unbounded memory growth during high-volume agent logging.
type RingBuffer struct {
	mu       sync.Mutex
	capacity int
	buf      []byte
	total    int64
}

// NewRingBuffer creates a RingBuffer with the given maximum byte capacity.
// If capacity <= 0, a default of 512KB is used.
func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 512 * 1024
	}
	return &RingBuffer{
		capacity: capacity,
		buf:      make([]byte, 0, capacity),
	}
}

// Write appends p to the ring buffer.
func (r *RingBuffer) Write(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	n = len(p)
	r.total += int64(n)

	if n >= r.capacity {
		r.buf = append(r.buf[:0], p[n-r.capacity:]...)
		return n, nil
	}

	overflow := (len(r.buf) + n) - r.capacity
	if overflow > 0 {
		copy(r.buf, r.buf[overflow:])
		r.buf = r.buf[:len(r.buf)-overflow]
	}
	r.buf = append(r.buf, p...)
	return n, nil
}

// String returns the buffered content as a string.
func (r *RingBuffer) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return string(r.buf)
}

// Bytes returns a copy of the buffered bytes.
func (r *RingBuffer) Bytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]byte, len(r.buf))
	copy(out, r.buf)
	return out
}

// Len returns the current buffered byte count.
func (r *RingBuffer) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.buf)
}

// Capacity returns the maximum capacity of the ring buffer.
func (r *RingBuffer) Capacity() int {
	return r.capacity
}

// TotalBytesWritten returns the total number of bytes written over the buffer's lifetime.
func (r *RingBuffer) TotalBytesWritten() int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.total
}

// Reset clears the buffer.
func (r *RingBuffer) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf = r.buf[:0]
	r.total = 0
}
