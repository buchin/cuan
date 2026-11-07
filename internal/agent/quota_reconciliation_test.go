package agent

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func oldQuotaFixture(t *testing.T) (*ProviderAccount, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	a, err := CreateProviderAccount("owner", "Existing label")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := ProviderAccountHome("owner", a.ID)
	if err = os.MkdirAll(AntigravityConfigDirPath(home), 0700); err != nil {
		t.Fatal(err)
	}
	credential := AntigravityCredentialsFilePath(home)
	if err = os.WriteFile(credential, []byte(`{"access_token":"fixture-credential"}`), 0600); err != nil {
		t.Fatal(err)
	}
	a.State = "unknown"
	if err = saveProviderAccount(a); err != nil {
		t.Fatal(err)
	}
	if err = BindProviderAccount("owner", a.ID, "agent_fixture", "conn_profile_fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err = SetProviderAccountFailover("owner", a.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err = UpdateProviderAccountSettings("owner", a.ID, a.Label, "claude-sonnet-test"); err != nil {
		t.Fatal(err)
	}
	if err = RecordProviderAccountQuotaWithDuration("owner", a.ID, 24*time.Hour, ""); err != nil {
		t.Fatal(err)
	}
	a, err = GetProviderAccount("owner", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	return a, credential
}

func TestFreshSelectedModelCapacityReconcilesOldUnscopedQuota(t *testing.T) {
	a, credential := oldQuotaFixture(t)
	original, _ := os.ReadFile(credential)
	deadline := *a.QuotaRetryAt
	usage := AccountModelUsage{Available: true, CheckedAt: time.Now().UTC(), Models: []ModelUsage{{Model: a.ModelID, Remaining: 100}}}
	actual, err := ReconcileSelectedModelQuota("owner", a.ID, usage)
	if err != nil || actual.State != "unknown" || actual.VerifiedAt != nil || actual.QuotaRetryAt != nil || !actual.ModelCooldowns[""].Equal(deadline) {
		t.Fatal("quota history lost or connection falsely verified", actual, err)
	}
	if actual.Label != a.Label || actual.ModelID != a.ModelID || actual.AgentID != a.AgentID || actual.ConnectionID != a.ConnectionID || actual.AllowFailover != a.AllowFailover || actual.AllowModelFallback != a.AllowModelFallback {
		t.Fatal("quota reconciliation changed identity/policy")
	}
	after, _ := os.ReadFile(credential)
	if !bytes.Equal(original, after) {
		t.Fatal("quota reconciliation changed credentials")
	}
	_, selected, release, err := LeaseHealthyBrainForTask("owner", nil)
	if err != nil {
		t.Fatal("fresh known Sonnet capacity still blocked", err)
	}
	if selected.ModelID != a.ModelID {
		t.Fatal("wrong selected model")
	}
	release()
	if _, err = UpdateProviderAccountSettings("owner", a.ID, a.Label, ""); err != nil {
		t.Fatal(err)
	}
	if _, _, release, err := LeaseHealthyBrainForTask("owner", nil); !errors.Is(err, ErrBrainCapacity) {
		if release != nil {
			release()
		}
		t.Fatal("historical Auto cooldown lost", err)
	}
}

func TestQuotaReconciliationRequiresFreshExactPositiveProviderEvidence(t *testing.T) {
	for _, scenario := range []string{"unavailable", "stale", "unknown quota", "zero quota", "different model", "before observation", "specific cooldown", "wrong owner"} {
		t.Run(scenario, func(t *testing.T) {
			a, _ := oldQuotaFixture(t)
			usage := AccountModelUsage{Available: true, CheckedAt: time.Now().UTC(), Models: []ModelUsage{{Model: a.ModelID, Remaining: 100}}}
			owner := "owner"
			switch scenario {
			case "unavailable":
				usage.Available = false
			case "stale":
				usage.CheckedAt = time.Now().Add(-time.Hour)
			case "unknown quota":
				usage.Models[0].Remaining = -1
			case "zero quota":
				usage.Models[0].Remaining = 0
			case "different model":
				usage.Models[0].Model = "gemini-test"
			case "before observation":
				usage.CheckedAt = *a.LastCheckedAt
			case "specific cooldown":
				if err := RecordProviderModelQuota("owner", a.ID, a.ModelID); err != nil {
					t.Fatal(err)
				}
			case "wrong owner":
				owner = "other-owner"
			}
			dir, _ := accountDirectory(a.ID)
			path := filepath.Join(dir, "account.json")
			before, _ := os.ReadFile(path)
			_, err := ReconcileSelectedModelQuota(owner, a.ID, usage)
			if (scenario == "wrong owner") != (err != nil) {
				t.Fatal("unexpected owner boundary result", err)
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(before, after) {
				t.Fatal("unproven quota observation changed account")
			}
		})
	}
}
