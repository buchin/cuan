package crypto

import (
	"testing"
)

func TestEncryptDecrypt_Success(t *testing.T) {
	secret := "super-secure-owner-secret-key-12345"
	original := "ghp_1234567890abcdefghijklmnopqrstuvwxyz"

	encrypted, err := Encrypt(original, secret)
	if err != nil {
		t.Fatalf("unexpected error encrypting: %v", err)
	}

	if encrypted == original {
		t.Fatalf("encrypted text must not match plaintext")
	}

	decrypted, err := Decrypt(encrypted, secret)
	if err != nil {
		t.Fatalf("unexpected error decrypting: %v", err)
	}

	if decrypted != original {
		t.Fatalf("expected %q, got %q", original, decrypted)
	}
}

func TestEncryptDecrypt_WrongKey(t *testing.T) {
	secret1 := "key-one"
	secret2 := "key-two"
	original := "ghp_token_value"

	encrypted, err := Encrypt(original, secret1)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	_, err = Decrypt(encrypted, secret2)
	if err == nil {
		t.Fatalf("expected decryption error with wrong key, got nil")
	}
}

func TestEncryptDecrypt_Empty(t *testing.T) {
	enc, err := Encrypt("", "secret")
	if err != nil || enc != "" {
		t.Fatalf("expected empty string for empty input, got %q, err: %v", enc, err)
	}

	dec, err := Decrypt("", "secret")
	if err != nil || dec != "" {
		t.Fatalf("expected empty string for empty input, got %q, err: %v", dec, err)
	}
}

func TestEncryptDecrypt_Corrupted(t *testing.T) {
	_, err := Decrypt("not-valid-hex", "secret")
	if err != ErrInvalidCiphertext {
		t.Fatalf("expected ErrInvalidCiphertext, got %v", err)
	}

	_, err = Decrypt("010203", "secret")
	if err != ErrInvalidCiphertext {
		t.Fatalf("expected ErrInvalidCiphertext for short ciphertext, got %v", err)
	}
}

func TestTokenPreview(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"short", "***"},
		{"ghp_ABCDEFGHIJKLMN1234", "ghp_***1234"},
		{"github_pat_11ABCD_longstringending9876", "github_pat_***9876"},
		{"custom_token_abcdef123456", "***3456"},
	}

	for _, tc := range cases {
		out := TokenPreview(tc.input)
		if out != tc.expected {
			t.Errorf("TokenPreview(%q) = %q; expected %q", tc.input, out, tc.expected)
		}
	}
}
