package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

var (
	ErrInvalidCiphertext = errors.New("invalid or corrupted ciphertext")
	ErrDecryptionFailed  = errors.New("decryption failed: incorrect key or corrupted data")
)

// deriveKey derives a 32-byte AES-256 key from a given secret string using SHA-256.
func deriveKey(secret string) []byte {
	hash := sha256.Sum256([]byte("cuan-credential-encryption-v1:" + secret))
	return hash[:]
}

// Encrypt encrypts plaintext using AES-256-GCM with a key derived from secret.
// Returns a hex-encoded string containing nonce + ciphertext + auth tag.
func Encrypt(plaintext, secret string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if secret == "" {
		return "", errors.New("encryption secret cannot be empty")
	}

	key := deriveKey(secret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(sealed), nil
}

// Decrypt decrypts a hex-encoded AES-256-GCM ciphertext using a key derived from secret.
func Decrypt(ciphertextHex, secret string) (string, error) {
	if ciphertextHex == "" {
		return "", nil
	}
	if secret == "" {
		return "", errors.New("decryption secret cannot be empty")
	}

	data, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return "", ErrInvalidCiphertext
	}

	key := deriveKey(secret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", ErrInvalidCiphertext
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintext), nil
}

// TokenPreview creates a sanitized preview of a token (e.g. ghp_***1a2b)
// without revealing the sensitive characters.
func TokenPreview(token string) string {
	t := strings.TrimSpace(token)
	if t == "" {
		return ""
	}

	// GitHub personal access tokens classic format: ghp_...
	// GitHub fine-grained PAT: github_pat_...
	if strings.HasPrefix(t, "ghp_") && len(t) > 8 {
		suffix := t[len(t)-4:]
		return "ghp_***" + suffix
	}
	if strings.HasPrefix(t, "github_pat_") && len(t) > 16 {
		suffix := t[len(t)-4:]
		return "github_pat_***" + suffix
	}

	if len(t) <= 6 {
		return "***"
	}
	return "***" + t[len(t)-4:]
}
