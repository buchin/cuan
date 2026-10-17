package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var (
	ErrInvalidKeySize = errors.New("crypto/aes: invalid key size: must be 32 bytes for AES-256")
)

const (
	aesGCMNonceSize = 12
	keyDerivePrefix = "cuan-aes256-gcm-vault-v1:"
)

// DeriveKeySHA256 generates a 32-byte AES-256 symmetric key from any secret string using SHA-256 with domain separation.
func DeriveKeySHA256(secret string) []byte {
	hash := sha256.Sum256([]byte(keyDerivePrefix + secret))
	key := make([]byte, 32)
	copy(key, hash[:])
	return key
}

// EncryptAES256GCM encrypts raw plaintext bytes using AES-256-GCM with the provided 32-byte key.
// It generates a secure random 12-byte nonce and returns [nonce + ciphertext + tag].
func EncryptAES256GCM(plaintext, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return sealed, nil
}

// DecryptAES256GCM decrypts raw ciphertext bytes created by EncryptAES256GCM using the 32-byte key.
// It checks authentication integrity via GCM tag and returns decrypted plaintext.
func DecryptAES256GCM(ciphertext, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKeySize
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher new gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrInvalidCiphertext
	}

	nonce := ciphertext[:nonceSize]
	actualCiphertext := ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, actualCiphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// EncryptStringAES256GCM encrypts a string using AES-256-GCM with a key derived from secret.
// Returns base64 encoded string of [nonce + ciphertext + tag].
func EncryptStringAES256GCM(plaintext, secret string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if secret == "" {
		return "", errors.New("encryption secret cannot be empty")
	}

	key := DeriveKeySHA256(secret)
	sealed, err := EncryptAES256GCM([]byte(plaintext), key)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptStringAES256GCM decrypts a base64 encoded ciphertext using AES-256-GCM with a key derived from secret.
func DecryptStringAES256GCM(encodedCiphertext, secret string) (string, error) {
	if encodedCiphertext == "" {
		return "", nil
	}
	if secret == "" {
		return "", errors.New("decryption secret cannot be empty")
	}

	data, err := base64.StdEncoding.DecodeString(encodedCiphertext)
	if err != nil {
		return "", ErrInvalidCiphertext
	}

	key := DeriveKeySHA256(secret)
	plaintext, err := DecryptAES256GCM(data, key)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// EncryptApplicationPassword encrypts an application password (e.g. for WordPress) via AES-256-GCM.
func EncryptApplicationPassword(appPassword, secret string) (string, error) {
	return EncryptStringAES256GCM(appPassword, secret)
}

// DecryptApplicationPassword decrypts an application password via AES-256-GCM.
func DecryptApplicationPassword(encodedCiphertext, secret string) (string, error) {
	return DecryptStringAES256GCM(encodedCiphertext, secret)
}

