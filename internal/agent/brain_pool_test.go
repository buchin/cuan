package agent

import (
	"os"
	"testing"
	"time"
)

func TestBrainPoolOwnerCooldownLeasesAndDistribution(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CUAN_ACCOUNT_MAX_SESSIONS", "1")
	makeBrain := func(owner string) *ProviderAccount {
		p, err := CreateProviderAccount(owner, "brain")
		if err != nil {
			t.Fatal(err)
		}
		home, _ := ProviderAccountHome(owner, p.ID)
		if err := os.MkdirAll(AntigravityConfigDirPath(home), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(AntigravityCredentialsFilePath(home), []byte(`{"access_token":"fixture-credential"}`), 0600); err != nil {
			t.Fatal(err)
		}
		p.State = "unknown"
		if err := saveProviderAccount(p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	a, b := makeBrain("owner"), makeBrain("owner")
	makeBrain("other-owner")
	firstHome, first, release, err := LeaseHealthyBrain("owner", nil)
	if err != nil || firstHome == "" {
		t.Fatal(err)
	}
	defer release()
	_, second, release2, err := LeaseHealthyBrain("owner", nil)
	if err != nil || second.ID == first.ID {
		t.Fatal("busy brain was not skipped", err)
	}
	release2()
	if err := RecordProviderAccountQuotaWithDuration("owner", second.ID, time.Hour, ""); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := LeaseHealthyBrain("owner", nil); err == nil {
		t.Fatal("cross-owner, busy or cooling brain leased")
	}
	release()
	if _, _, _, err := LeaseHealthyBrain("", nil); err == nil {
		t.Fatal("ambient owner fallback accepted")
	}
	if _, _, _, err := LeaseHealthyBrain("owner", map[string]bool{a.ID: true, b.ID: true}); err == nil {
		t.Fatal("excluded brains reused")
	}
}

func TestBrainSettingsPreserveConnectionAndCredentials(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	p, err := CreateProviderAccount("owner", "Original")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := ProviderAccountHome("owner", p.ID)
	_ = os.MkdirAll(AntigravityConfigDirPath(home), 0700)
	_ = os.WriteFile(AntigravityCredentialsFilePath(home), []byte("fixture-credentials-only"), 0600)
	if err := BindProviderAccount("owner", p.ID, "agent_brain", "conn_profile_fixture"); err != nil {
		t.Fatal(err)
	}
	saved, err := UpdateProviderAccountSettings("owner", p.ID, "Work brain", "gemini-test")
	if err != nil || saved.Label != "Work brain" || saved.ModelID != "gemini-test" || saved.ConnectionID != "conn_profile_fixture" || !saved.CredentialsPresent {
		t.Fatalf("metadata edit changed connection: %+v %v", saved, err)
	}
	if _, err := UpdateProviderAccountSettings("another-owner", p.ID, "Stolen", ""); err == nil {
		t.Fatal("cross-owner edit accepted")
	}
	if _, err := UpdateProviderAccountSettings("owner", p.ID, "Valid", "--unsafe-option"); err == nil {
		t.Fatal("invalid model accepted")
	}
	if _, err := UpdateProviderAccountSettings("owner", p.ID, "\n", ""); err == nil {
		t.Fatal("blank name accepted")
	}
	reloaded, err := GetProviderAccount("owner", p.ID)
	if err != nil || reloaded.Label != "Work brain" || reloaded.ModelID != "gemini-test" {
		t.Fatal("settings did not persist")
	}
}
