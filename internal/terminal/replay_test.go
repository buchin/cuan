package terminal

import (
	"bytes"
	"strings"
	"testing"
)

func TestSanitizePTYReplay_Empty(t *testing.T) {
	if got := SanitizePTYReplay(nil, 0); got != nil {
		t.Fatalf("expected nil for nil input, got: %v", got)
	}
	if got := SanitizePTYReplay([]byte{}, 0); got != nil {
		t.Fatalf("expected nil for empty input, got: %v", got)
	}
}

func TestSanitizePTYReplay_PreservesSmallRawBytes(t *testing.T) {
	raw := []byte("chunk-1\nchunk-2\nchunk-3\n")
	got := SanitizePTYReplay(raw, 32*1024)
	if !bytes.Equal(got, raw) {
		t.Fatalf("expected raw bytes preserved for small clean input, got %q, want %q", got, raw)
	}
}

func TestSanitizePTYReplay_StripsDECSync(t *testing.T) {
	raw := []byte("\x1b[?2026hhello world\x1b[?2026l")
	got := SanitizePTYReplay(raw, 32*1024)
	if bytes.Contains(got, decSyncBegin) {
		t.Fatalf("expected DEC sync begin marker stripped, got %q", got)
	}
	if !strings.Contains(string(got), "hello world") {
		t.Fatalf("expected content preserved, got %q", got)
	}
}

func TestSanitizePTYReplay_CapsLargeBuffer(t *testing.T) {
	large := bytes.Repeat([]byte("line of text in terminal scrollback\n"), 2000) // ~72 KB
	if len(large) <= 32*1024 {
		t.Fatalf("test data should exceed 32 KiB, got %d", len(large))
	}

	got := SanitizePTYReplay(large, 32*1024)
	if len(got) > 32*1024+128 { // allow small prefix/suffix margin
		t.Fatalf("expected output bounded to ~32 KiB, got %d", len(got))
	}
	if !strings.Contains(string(got), "line of text in terminal scrollback\n") {
		t.Fatalf("expected newest lines retained")
	}
}

func TestSanitizePTYReplay_PrefersLastClearOrigin(t *testing.T) {
	oldGarbage := bytes.Repeat([]byte("stale old screen text\n"), 500)
	clearSeq := []byte("\x1b[2J")
	newScreen := []byte("fresh active prompt $ ")
	raw := append(oldGarbage, append(clearSeq, newScreen...)...)

	got := SanitizePTYReplay(raw, 32*1024)
	if strings.Contains(string(got), "stale old screen text") {
		t.Fatalf("expected text before last clear screen to be excluded, got: %q", string(got))
	}
	if !strings.Contains(string(got), "fresh active prompt $ ") {
		t.Fatalf("expected fresh screen text to be present, got: %q", string(got))
	}
}

func TestSanitizePTYReplay_AlignsToEscapeBoundary(t *testing.T) {
	// Simulate slicing where tail starts with partial escape "\x1b[32mhello"
	prefix := bytes.Repeat([]byte("x"), 100)
	colorSeq := []byte("\x1b[32mcolored\x1b[0m")
	raw := append(prefix, colorSeq...)

	// Cap to size of colorSeq + 10 bytes of prefix
	capSize := len(colorSeq) + 10
	got := SanitizePTYReplay(raw, capSize)
	if !strings.Contains(string(got), "colored") {
		t.Fatalf("expected 'colored' in output, got %q", string(got))
	}
}
