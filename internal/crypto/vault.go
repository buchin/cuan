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
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	// DefaultMasterKeySeed is the deterministic fallback key seed used in development/test
	// when VAULT_MASTER_KEY environment variable is not explicitly configured.
	DefaultMasterKeySeed = "cuan-vault-master-key-seed-dev-fallback-v1"
	vaultDomainPrefix    = "cuan-vault-master-aes256-gcm-v1:"
)

var (
	ErrInvalidSSHKey = errors.New("invalid or unparseable SSH key")
)

// KeyVault manages symmetric AES-256-GCM encryption and decryption of sensitive secrets
// such as SSH private keys, ensuring secure storage at-rest.
type KeyVault struct {
	masterKey []byte
}

// NewKeyVault instantiates a KeyVault. If customSecret is provided, it derives
// the master key from it; otherwise, it checks the VAULT_MASTER_KEY environment variable,
// falling back to DefaultMasterKeySeed for local development and tests.
func NewKeyVault(customSecret ...string) *KeyVault {
	var secret string
	if len(customSecret) > 0 && customSecret[0] != "" {
		secret = customSecret[0]
	} else if envSecret := os.Getenv("VAULT_MASTER_KEY"); envSecret != "" {
		secret = envSecret
	} else {
		secret = DefaultMasterKeySeed
	}

	hash := sha256.Sum256([]byte(vaultDomainPrefix + secret))
	derivedKey := make([]byte, 32)
	copy(derivedKey, hash[:])

	return &KeyVault{
		masterKey: derivedKey,
	}
}

// DefaultKeyVault returns a KeyVault instance initialized from environment configuration.
func DefaultKeyVault() *KeyVault {
	return NewKeyVault()
}

// Encrypt encrypts plaintext bytes using AES-256-GCM with a 12-byte random nonce.
// Returns a standard base64 string containing nonce + ciphertext + authentication tag.
func (v *KeyVault) Encrypt(plaintext []byte) (string, error) {
	if len(plaintext) == 0 {
		return "", nil
	}

	block, err := aes.NewCipher(v.masterKey)
	if err != nil {
		return "", fmt.Errorf("create aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts a base64-encoded ciphertext created by Encrypt, verifying the authentication tag.
func (v *KeyVault) Decrypt(ciphertextB64 string) ([]byte, error) {
	if ciphertextB64 == "" {
		return nil, nil
	}

	data, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, ErrInvalidCiphertext
	}

	block, err := aes.NewCipher(v.masterKey)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, ErrInvalidCiphertext
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// VaultEncrypt is a package-level helper that encrypts using DefaultKeyVault.
func VaultEncrypt(plaintext []byte) (string, error) {
	return DefaultKeyVault().Encrypt(plaintext)
}

// VaultDecrypt is a package-level helper that decrypts using DefaultKeyVault.
func VaultDecrypt(ciphertextB64 string) ([]byte, error) {
	return DefaultKeyVault().Decrypt(ciphertextB64)
}

// CalculateSSHFingerprint calculates the standard OpenSSH SHA256 fingerprint (e.g. SHA256:...)
// from either a private key (PEM/OpenSSH format) or a public key / authorized_keys line.
func CalculateSSHFingerprint(keyBytes []byte) (string, error) {
	trimmed := strings.TrimSpace(string(keyBytes))
	if trimmed == "" {
		return "", ErrInvalidSSHKey
	}

	// 1. Try parsing as private key (RSA, Ed25519, ECDSA)
	if signer, err := ssh.ParsePrivateKey([]byte(trimmed)); err == nil {
		return ssh.FingerprintSHA256(signer.PublicKey()), nil
	}

	// 2. Try parsing as authorized_keys line (e.g., "ssh-ed25519 AAAAC3... comment")
	if pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(trimmed)); err == nil {
		return ssh.FingerprintSHA256(pubKey), nil
	}

	// 3. Try parsing as raw wire public key
	if pubKey, err := ssh.ParsePublicKey([]byte(trimmed)); err == nil {
		return ssh.FingerprintSHA256(pubKey), nil
	}

	return "", fmt.Errorf("%w: unrecognized key format", ErrInvalidSSHKey)
}

// CalculateFingerprint is a method on KeyVault for calculating the SSH key fingerprint.
func (v *KeyVault) CalculateFingerprint(keyBytes []byte) (string, error) {
	return CalculateSSHFingerprint(keyBytes)
}
