package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type ConnectionVerification struct {
	State         string     `json:"state"`
	LastCheckedAt *time.Time `json:"last_checked_at,omitempty"`
	VerifiedAt    *time.Time `json:"verified_at,omitempty"`
}
type storedConnectionVerification struct {
	ConnectionVerification
	Owner          string `json:"owner"`
	AgentID        string `json:"agent_id"`
	ConnectionID   string `json:"connection_id"`
	CredentialHash string `json:"credential_hash"`
}

func legacyCredentialFingerprint() string {
	path, found := ResolveAntigravityTokenPath()
	if !found {
		return ""
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func connectionVerificationPath(owner, agentID, connectionID string) string {
	sum := sha256.Sum256([]byte(owner + "\x00" + agentID + "\x00" + connectionID))
	return filepath.Join(accountRoot(), "connection-checks", hex.EncodeToString(sum[:])+".json")
}

// Persist only verification metadata, never move or rewrite a legacy credential.
func SaveLegacyConnectionVerification(owner, agentID, connectionID string, profile *ProviderAccount) error {
	path := connectionVerificationPath(owner, agentID, connectionID)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if err := os.Chmod(accountRoot(), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(storedConnectionVerification{ConnectionVerification: ConnectionVerification{State: profile.State, LastCheckedAt: profile.LastCheckedAt, VerifiedAt: profile.VerifiedAt}, Owner: owner, AgentID: agentID, ConnectionID: connectionID, CredentialHash: legacyCredentialFingerprint()})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".check-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func LegacyConnectionVerification(owner, agentID, connectionID string) ConnectionVerification {
	unknown := ConnectionVerification{State: "unknown"}
	if owner == "" {
		return unknown
	}
	raw, err := os.ReadFile(connectionVerificationPath(owner, agentID, connectionID))
	if err != nil {
		return unknown
	}
	var record storedConnectionVerification
	if json.Unmarshal(raw, &record) != nil || record.Owner != owner || record.AgentID != agentID || record.ConnectionID != connectionID {
		return unknown
	}
	result := record.ConnectionVerification
	fingerprint := legacyCredentialFingerprint()
	if fingerprint == "" {
		result.State = "login_required"
		result.VerifiedAt = nil
		return result
	}
	if fingerprint != record.CredentialHash {
		result.State = "unknown"
		result.VerifiedAt = nil
		return result
	}
	if result.State == "verified" && result.VerifiedAt == nil {
		result.State = "unknown"
	}
	return result
}
