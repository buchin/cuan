package agent

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestRingBuffer_Basics(t *testing.T) {
	rb := NewRingBuffer(10)
	if rb.Capacity() != 10 {
		t.Fatalf("expected capacity 10, got %d", rb.Capacity())
	}
	if rb.Len() != 0 {
		t.Fatalf("expected len 0, got %d", rb.Len())
	}

	n, err := rb.Write([]byte("hello"))
	if err != nil || n != 5 {
		t.Fatalf("write failed: n=%d, err=%v", n, err)
	}
	if rb.String() != "hello" {
		t.Fatalf("expected 'hello', got '%s'", rb.String())
	}
	if rb.TotalBytesWritten() != 5 {
		t.Fatalf("expected total 5, got %d", rb.TotalBytesWritten())
	}

	// Write more without overflowing
	_, _ = rb.Write([]byte("world"))
	if rb.String() != "helloworld" {
		t.Fatalf("expected 'helloworld', got '%s'", rb.String())
	}
	if rb.Len() != 10 {
		t.Fatalf("expected len 10, got %d", rb.Len())
	}

	// Overflow: write "!123" (4 bytes). Oldest 4 bytes ("hell") should be dropped
	_, _ = rb.Write([]byte("!123"))
	if rb.Len() != 10 {
		t.Fatalf("expected len 10, got %d", rb.Len())
	}
	if rb.String() != "oworld!123" {
		t.Fatalf("expected 'oworld!123', got '%s'", rb.String())
	}
	if rb.TotalBytesWritten() != 14 {
		t.Fatalf("expected total 14, got %d", rb.TotalBytesWritten())
	}

	// Write block larger than capacity: "abcdefghijkl" (12 bytes)
	_, _ = rb.Write([]byte("abcdefghijkl"))
	if rb.Len() != 10 {
		t.Fatalf("expected len 10, got %d", rb.Len())
	}
	// Last 10 bytes of "abcdefghijkl" is "cdefghijkl"
	if rb.String() != "cdefghijkl" {
		t.Fatalf("expected 'cdefghijkl', got '%s'", rb.String())
	}
}

func TestRingBuffer_ConcurrentWrites(t *testing.T) {
	rb := NewRingBuffer(50)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			line := fmt.Sprintf("line-%02d\n", idx)
			_, _ = rb.Write([]byte(line))
		}(i)
	}
	wg.Wait()

	if rb.Len() > 50 {
		t.Fatalf("buffer exceeded capacity: len=%d", rb.Len())
	}
	if rb.TotalBytesWritten() != 20*8 {
		t.Fatalf("expected total %d, got %d", 20*8, rb.TotalBytesWritten())
	}
	str := rb.String()
	if !strings.Contains(str, "line-") {
		t.Fatalf("expected buffer to contain line entries, got: %s", str)
	}
}

func TestRingBuffer_Reset(t *testing.T) {
	rb := NewRingBuffer(20)
	_, _ = rb.Write([]byte("some data to reset"))
	if rb.Len() == 0 {
		t.Fatalf("expected non-zero len before reset")
	}
	rb.Reset()
	if rb.Len() != 0 || rb.String() != "" || rb.TotalBytesWritten() != 0 {
		t.Fatalf("expected empty buffer after reset")
	}
}
