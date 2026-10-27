package crypto

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"os"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestKeyVault_RoundTrip(t *testing.T) {
	vault := NewKeyVault("test-master-secret-12345")

	original := []byte("-----BEGIN OPENSSH PRIVATE KEY-----\ntest-super-secret-ssh-key\n-----END OPENSSH PRIVATE KEY-----")
	encrypted, err := vault.Encrypt(original)
	if err != nil {
		t.Fatalf("vault.Encrypt() unexpected error: %v", err)
	}

	if encrypted == "" {
		t.Fatalf("expected non-empty encrypted string")
	}

	// Verify it does not contain the plaintext
	if strings.Contains(encrypted, "test-super-secret-ssh-key") {
		t.Fatalf("ciphertext contains plaintext leak!")
	}

	decrypted, err := vault.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("vault.Decrypt() unexpected error: %v", err)
	}

	if !bytes.Equal(original, decrypted) {
		t.Fatalf("decrypted mismatch: expected %q, got %q", string(original), string(decrypted))
	}
}

func TestKeyVault_NonceUniqueness(t *testing.T) {
	vault := NewKeyVault("test-secret-nonce")
	secret := []byte("constant-payload-data")

	c1, err := vault.Encrypt(secret)
	if err != nil {
		t.Fatalf("Encrypt 1 failed: %v", err)
	}
	c2, err := vault.Encrypt(secret)
	if err != nil {
		t.Fatalf("Encrypt 2 failed: %v", err)
	}

	if c1 == c2 {
		t.Fatalf("expected different ciphertexts due to random 12-byte nonce, got identical")
	}

	// But both decrypt to the same payload
	p1, _ := vault.Decrypt(c1)
	p2, _ := vault.Decrypt(c2)
	if !bytes.Equal(p1, secret) || !bytes.Equal(p2, secret) {
		t.Fatalf("decryption failed for one of the ciphertexts")
	}
}

func TestKeyVault_EmptyPayloads(t *testing.T) {
	vault := NewKeyVault("test-empty")

	enc, err := vault.Encrypt([]byte{})
	if err != nil || enc != "" {
		t.Fatalf("expected empty string for empty plaintext, got %q (err: %v)", enc, err)
	}

	dec, err := vault.Decrypt("")
	if err != nil || dec != nil {
		t.Fatalf("expected nil for empty ciphertext, got %v (err: %v)", dec, err)
	}
}

func TestKeyVault_TamperDetection(t *testing.T) {
	vault := NewKeyVault("tamper-test-key")
	original := []byte("sensitive-server-private-key-data")

	ciphertext, err := vault.Encrypt(original)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Tamper with invalid base64
	_, err = vault.Decrypt("!!!invalid-base64???")
	if err == nil {
		t.Fatalf("expected error on invalid base64, got nil")
	}

	// Tamper with short data
	shortData := base64.StdEncoding.EncodeToString([]byte("short"))
	_, err = vault.Decrypt(shortData)
	if err == nil {
		t.Fatalf("expected error on truncated data, got nil")
	}

	// Tamper with ciphertext bytes
	raw, _ := base64.StdEncoding.DecodeString(ciphertext)
	raw[len(raw)-1] ^= 0xFF // flip last byte
	tamperedB64 := base64.StdEncoding.EncodeToString(raw)

	_, err = vault.Decrypt(tamperedB64)
	if err == nil {
		t.Fatalf("expected auth tag verification failure on tampered ciphertext, got nil")
	}
}

func TestKeyVault_KeyIsolation(t *testing.T) {
	v1 := NewKeyVault("secret-key-alpha")
	v2 := NewKeyVault("secret-key-beta")

	original := []byte("payload-to-test-isolation")
	c1, err := v1.Encrypt(original)
	if err != nil {
		t.Fatalf("v1.Encrypt failed: %v", err)
	}

	// v2 cannot decrypt c1
	_, err = v2.Decrypt(c1)
	if err == nil {
		t.Fatalf("expected decryption failure when using different master key, got success")
	}
}

func TestKeyVault_EnvMasterKey(t *testing.T) {
	oldEnv := os.Getenv("VAULT_MASTER_KEY")
	defer os.Setenv("VAULT_MASTER_KEY", oldEnv)

	os.Setenv("VAULT_MASTER_KEY", "env-configured-vault-secret-999")
	v := DefaultKeyVault()

	payload := []byte("testing-env-vault-master")
	enc, err := v.Encrypt(payload)
	if err != nil {
		t.Fatalf("v.Encrypt failed: %v", err)
	}

	dec, err := v.Decrypt(enc)
	if err != nil || !bytes.Equal(dec, payload) {
		t.Fatalf("v.Decrypt failed or mismatch: %v", err)
	}

	// Also test package-level helpers
	pkgEnc, err := VaultEncrypt(payload)
	if err != nil {
		t.Fatalf("VaultEncrypt failed: %v", err)
	}
	pkgDec, err := VaultDecrypt(pkgEnc)
	if err != nil || !bytes.Equal(pkgDec, payload) {
		t.Fatalf("VaultDecrypt failed or mismatch: %v", err)
	}
}

func TestCalculateSSHFingerprint(t *testing.T) {
	// 1. Generate an Ed25519 keypair for testing
	pubEd25519, privEd25519, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519.GenerateKey failed: %v", err)
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(privEd25519)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey failed: %v", err)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	})

	sshPubKey, err := ssh.NewPublicKey(pubEd25519)
	if err != nil {
		t.Fatalf("ssh.NewPublicKey failed: %v", err)
	}
	expectedFingerprint := ssh.FingerprintSHA256(sshPubKey)

	// Fingerprint from private key PEM
	fpFromPriv, err := CalculateSSHFingerprint(privPEM)
	if err != nil {
		t.Fatalf("CalculateSSHFingerprint(privPEM) unexpected error: %v", err)
	}
	if fpFromPriv != expectedFingerprint {
		t.Errorf("fingerprint mismatch from private key: expected %s, got %s", expectedFingerprint, fpFromPriv)
	}
	if !strings.HasPrefix(fpFromPriv, "SHA256:") {
		t.Errorf("fingerprint does not have SHA256: prefix: %s", fpFromPriv)
	}

	// Fingerprint from authorized_keys line
	authKeyLine := string(ssh.MarshalAuthorizedKey(sshPubKey))
	fpFromAuth, err := CalculateSSHFingerprint([]byte(authKeyLine))
	if err != nil {
		t.Fatalf("CalculateSSHFingerprint(authKeyLine) unexpected error: %v", err)
	}
	if fpFromAuth != expectedFingerprint {
		t.Errorf("fingerprint mismatch from authorized_keys: expected %s, got %s", expectedFingerprint, fpFromAuth)
	}

	// 2. Generate an RSA keypair for testing
	rsaPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey failed: %v", err)
	}
	rsaPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaPriv),
	})
	rsaSSHPublic, err := ssh.NewPublicKey(&rsaPriv.PublicKey)
	if err != nil {
		t.Fatalf("ssh.NewPublicKey RSA failed: %v", err)
	}
	expectedRSAFingerprint := ssh.FingerprintSHA256(rsaSSHPublic)

	fpFromRSA, err := CalculateSSHFingerprint(rsaPEM)
	if err != nil {
		t.Fatalf("CalculateSSHFingerprint(rsaPEM) unexpected error: %v", err)
	}
	if fpFromRSA != expectedRSAFingerprint {
		t.Errorf("RSA fingerprint mismatch: expected %s, got %s", expectedRSAFingerprint, fpFromRSA)
	}

	// 3. Invalid key input
	_, err = CalculateSSHFingerprint([]byte("not-a-valid-ssh-key-at-all"))
	if err == nil {
		t.Fatalf("expected error on invalid key string, got nil")
	}

	_, err = CalculateSSHFingerprint([]byte(""))
	if err == nil {
		t.Fatalf("expected error on empty key string, got nil")
	}
}
