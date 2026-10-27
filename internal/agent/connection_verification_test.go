package agent

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestS26LegacyCheckPersistsAndInvalidates(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := home + "/legacy-token"
	t.Setenv("ANTIGRAVITY_TOKEN_PATH", path)
	os.WriteFile(path, []byte("fixture-legacy-credential-value"), 0600)
	now := time.Now().UTC()
	profile := &ProviderAccount{State: "verified", LastCheckedAt: &now, VerifiedAt: &now}
	if err := SaveLegacyConnectionVerification("owner", "agent_pin1", "conn_agent_pin1", profile); err != nil {
		t.Fatal(err)
	}
	read := LegacyConnectionVerification("owner", "agent_pin1", "conn_agent_pin1")
	if read.State != "verified" || read.VerifiedAt == nil {
		t.Fatal("reload lost successful verification")
	}
	if LegacyConnectionVerification("other-owner", "agent_pin1", "conn_agent_pin1").State != "unknown" {
		t.Fatal("another owner read verification")
	}
	if LegacyConnectionVerification("owner", "other-agent", "conn_agent_pin1").State != "unknown" {
		t.Fatal("another agent borrowed verification")
	}
	file := connectionVerificationPath("owner", "agent_pin1", "conn_agent_pin1")
	info, _ := os.Stat(file)
	if info.Mode().Perm() != 0600 {
		t.Fatal("verification metadata is not private")
	}
	data, _ := os.ReadFile(file)
	var saved storedConnectionVerification
	json.Unmarshal(data, &saved)
	past := time.Now().Add(-6 * time.Minute)
	saved.VerifiedAt = &past
	data, _ = json.Marshal(saved)
	os.WriteFile(file, data, 0600)
	if LegacyConnectionVerification("owner", "agent_pin1", "conn_agent_pin1").State != "verified" {
		t.Fatal("verified connection degraded over time despite valid credentials")
	}
	SaveLegacyConnectionVerification("owner", "agent_pin1", "conn_agent_pin1", profile)
	os.WriteFile(path, []byte("changed-fixture-credential-value"), 0600)
	if LegacyConnectionVerification("owner", "agent_pin1", "conn_agent_pin1").State != "unknown" {
		t.Fatal("changed credentials inherited success")
	}
	os.Remove(path)
	if LegacyConnectionVerification("owner", "agent_pin1", "conn_agent_pin1").State != "login_required" {
		t.Fatal("removed credentials still connected")
	}
}
