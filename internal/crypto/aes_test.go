package crypto

import (
	"crypto/rand"
	"strings"
	"testing"
)

func TestAES256GCM_RawBytes(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("failed to read rand key: %v", err)
	}

	plaintext := []byte("cuan-gemini-pro-api-key-test-12345")

	ciphertext, err := EncryptAES256GCM(plaintext, key)
	if err != nil {
		t.Fatalf("EncryptAES256GCM failed: %v", err)
	}

	if len(ciphertext) <= len(plaintext) {
		t.Fatalf("ciphertext too short, expected nonce + ct + tag: len=%d", len(ciphertext))
	}

	decrypted, err := DecryptAES256GCM(ciphertext, key)
	if err != nil {
		t.Fatalf("DecryptAES256GCM failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted text mismatch: got %q, want %q", string(decrypted), string(plaintext))
	}

	// Test invalid key size
	invalidKey := make([]byte, 16)
	if _, err := EncryptAES256GCM(plaintext, invalidKey); err != ErrInvalidKeySize {
		t.Errorf("expected ErrInvalidKeySize on Encrypt, got %v", err)
	}
	if _, err := DecryptAES256GCM(ciphertext, invalidKey); err != ErrInvalidKeySize {
		t.Errorf("expected ErrInvalidKeySize on Decrypt, got %v", err)
	}

	// Test corrupted ciphertext
	corrupted := make([]byte, len(ciphertext))
	copy(corrupted, ciphertext)
	corrupted[len(corrupted)-1] ^= 0xFF

	if _, err := DecryptAES256GCM(corrupted, key); err != ErrDecryptionFailed {
		t.Errorf("expected ErrDecryptionFailed on tampered ciphertext, got %v", err)
	}

	// Test truncated ciphertext
	if _, err := DecryptAES256GCM(ciphertext[:5], key); err != ErrInvalidCiphertext {
		t.Errorf("expected ErrInvalidCiphertext on truncated ciphertext, got %v", err)
	}
}

func TestAES256GCM_StringHelpers(t *testing.T) {
	secret := "production-vault-master-secret-phrase"
	apiKey := "AIzaSyD-GeminiProSecretKeyForTestingSprint09"

	encrypted, err := EncryptStringAES256GCM(apiKey, secret)
	if err != nil {
		t.Fatalf("EncryptStringAES256GCM failed: %v", err)
	}

	if encrypted == "" || strings.Contains(encrypted, apiKey) {
		t.Errorf("encrypted string contains plaintext or is empty: %q", encrypted)
	}

	decrypted, err := DecryptStringAES256GCM(encrypted, secret)
	if err != nil {
		t.Fatalf("DecryptStringAES256GCM failed: %v", err)
	}

	if decrypted != apiKey {
		t.Errorf("decrypted mismatch: got %q, want %q", decrypted, apiKey)
	}

	// Empty string handling
	emptyEnc, err := EncryptStringAES256GCM("", secret)
	if err != nil || emptyEnc != "" {
		t.Errorf("expected empty string result on empty input, got %q, err=%v", emptyEnc, err)
	}

	emptyDec, err := DecryptStringAES256GCM("", secret)
	if err != nil || emptyDec != "" {
		t.Errorf("expected empty string result on empty input, got %q, err=%v", emptyDec, err)
	}

	// Wrong secret
	_, err = DecryptStringAES256GCM(encrypted, "wrong-secret")
	if err == nil {
		t.Errorf("expected error decrypting with wrong secret, got nil")
	}
}

func TestApplicationPasswordEncryption(t *testing.T) {
	appPassword := "abcd 1234 efgh 5678"
	secret := "test-vault-encryption-secret-12345"

	encrypted, err := EncryptApplicationPassword(appPassword, secret)
	if err != nil {
		t.Fatalf("EncryptApplicationPassword failed: %v", err)
	}
	if encrypted == appPassword || encrypted == "" {
		t.Fatalf("expected encrypted ciphertext, got %q", encrypted)
	}

	decrypted, err := DecryptApplicationPassword(encrypted, secret)
	if err != nil {
		t.Fatalf("DecryptApplicationPassword failed: %v", err)
	}
	if decrypted != appPassword {
		t.Errorf("expected decrypted %q, got %q", appPassword, decrypted)
	}

	// Tampered ciphertext fails
	_, err = DecryptApplicationPassword("invalid-base64-random-junk", secret)
	if err == nil {
		t.Errorf("expected error decrypting tampered ciphertext")
	}
}

