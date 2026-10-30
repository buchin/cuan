package agent

import (
	"strings"
	"testing"
)

func TestEncryptDecryptCredential(t *testing.T) {
	key := "AIzaSyD-TestGeminiAPIKey1234567890"

	encrypted, err := EncryptCredential(key)
	if err != nil {
		t.Fatalf("EncryptCredential failed: %v", err)
	}

	if encrypted == "" || strings.Contains(encrypted, key) {
		t.Fatalf("encrypted credential contains plaintext: %q", encrypted)
	}

	decrypted, err := DecryptCredential(encrypted)
	if err != nil {
		t.Fatalf("DecryptCredential failed: %v", err)
	}

	if decrypted != key {
		t.Errorf("decrypted credential mismatch: got %q, want %q", decrypted, key)
	}

	// Empty checks
	if _, err := EncryptCredential(""); err != ErrEmptyCredential {
		t.Errorf("expected ErrEmptyCredential on encrypt empty, got %v", err)
	}
	if _, err := DecryptCredential(""); err != ErrEmptyCredential {
		t.Errorf("expected ErrEmptyCredential on decrypt empty, got %v", err)
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"short", "******"},
		{"AIzaSyBKX1234567890XYZ", "AIzaSy...0XYZ"},
		{"sk-proj-123456789abcdef", "sk-...cdef"},
		{"xai-1234567890abcdef", "xai-...cdef"},
		{"gsk_1234567890abcdef", "gsk_...cdef"},
		{"custom_secret_key_1234", "***1234"},
	}

	for _, tt := range tests {
		got := MaskAPIKey(tt.input)
		if got != tt.expected {
			t.Errorf("MaskAPIKey(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
