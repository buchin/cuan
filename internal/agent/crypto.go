package agent

import (
	"errors"
	"os"
	"strings"

	"github.com/buchin/cuan/internal/crypto"
)

const (
	// DefaultAgentVaultSeed is used as fallback key seed for agent credentials in dev/test.
	DefaultAgentVaultSeed = "cuan-agent-connection-vault-seed-v1"
)

var (
	ErrEmptyCredential = errors.New("agent/crypto: credential cannot be empty")
)

// resolveVaultSecret determines the master secret to use for credential encryption.
func resolveVaultSecret(customSecret ...string) string {
	if len(customSecret) > 0 && strings.TrimSpace(customSecret[0]) != "" {
		return strings.TrimSpace(customSecret[0])
	}
	if envKey := os.Getenv("VAULT_MASTER_KEY"); strings.TrimSpace(envKey) != "" {
		return strings.TrimSpace(envKey)
	}
	return DefaultAgentVaultSeed
}

// EncryptCredential encrypts sensitive agent provider credentials (e.g. Gemini API keys, OAuth tokens)
// using 256-bit AES-GCM with a random nonce.
func EncryptCredential(plaintext string, masterSecret ...string) (string, error) {
	trimmed := strings.TrimSpace(plaintext)
	if trimmed == "" {
		return "", ErrEmptyCredential
	}
	secret := resolveVaultSecret(masterSecret...)
	return crypto.EncryptStringAES256GCM(trimmed, secret)
}

// DecryptCredential decrypts an encrypted credential string produced by EncryptCredential.
func DecryptCredential(ciphertext string, masterSecret ...string) (string, error) {
	trimmed := strings.TrimSpace(ciphertext)
	if trimmed == "" {
		return "", ErrEmptyCredential
	}
	secret := resolveVaultSecret(masterSecret...)
	return crypto.DecryptStringAES256GCM(trimmed, secret)
}

// MaskAPIKey returns a safe, redacted representation of an API key for safe UI / log display.
// e.g. "AIzaSy...ABCD" for Google keys, "sk-...xxxx" for OpenAI/Anthropic/DeepSeek keys, or "***ABCD" for other keys.
func MaskAPIKey(key string) string {
	t := strings.TrimSpace(key)
	if t == "" {
		return ""
	}
	if len(t) <= 8 {
		return "******"
	}
	if strings.HasPrefix(t, "AIzaSy") && len(t) >= 12 {
		return t[:6] + "..." + t[len(t)-4:]
	}
	if strings.HasPrefix(t, "sk-") && len(t) >= 8 {
		return "sk-..." + t[len(t)-4:]
	}
	if strings.HasPrefix(t, "xai-") && len(t) >= 8 {
		return "xai-..." + t[len(t)-4:]
	}
	if strings.HasPrefix(t, "gsk_") && len(t) >= 8 {
		return "gsk_..." + t[len(t)-4:]
	}
	return "***" + t[len(t)-4:]
}
