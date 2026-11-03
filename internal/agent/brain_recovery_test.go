package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUnavailableBrainRecheckRespectsOwnerCooldownAndLease(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	create := func(owner, state string, checked time.Time) *ProviderAccount {
		a, err := CreateProviderAccount(owner, "Fixture")
		if err != nil {
			t.Fatal(err)
		}
		home, _ := ProviderAccountHome(owner, a.ID)
		os.MkdirAll(AntigravityConfigDirPath(home), 0700)
		os.WriteFile(AntigravityCredentialsFilePath(home), []byte("fixture-credentials"), 0600)
		BindProviderAccount(owner, a.ID, "agent_fixture", "conn_profile_fixture")
		dir, _ := accountDirectory(a.ID)
		data, _ := os.ReadFile(filepath.Join(dir, "account.json"))
		var stored storedProviderAccount
		json.Unmarshal(data, &stored)
		stored.State = state
		stored.LastCheckedAt = &checked
		if state == "quota_limited" {
			retry := time.Now().Add(time.Hour)
			stored.QuotaRetryAt = &retry
		}
		data, _ = json.Marshal(stored)
		os.WriteFile(filepath.Join(dir, "account.json"), data, 0600)
		a, _ = GetProviderAccount(owner, a.ID)
		return a
	}
	old := time.Now().Add(-time.Hour)
	candidate := create("owner", "unavailable", old)
	create("other-owner", "unavailable", old)
	create("owner", "quota_limited", old)
	calls := 0
	probe := func(ctx context.Context, owner, id string) (*ProviderAccount, error) {
		calls++
		if owner != "owner" || id != candidate.ID {
			t.Fatal("wrong owner or cooling account probed")
		}
		return nil, nil
	}
	release, err := TryLeaseProviderAccount("owner", candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	recheckUnavailableBrain(context.Background(), "owner", probe)
	if calls != 0 {
		t.Fatal("busy account probed")
	}
	release()
	recheckUnavailableBrain(context.Background(), "owner", probe)
	if calls != 1 {
		t.Fatal("stale unavailable account never rechecked")
	}
	dir, _ := accountDirectory(candidate.ID)
	data, _ := os.ReadFile(filepath.Join(dir, "account.json"))
	var stored storedProviderAccount
	json.Unmarshal(data, &stored)
	now := time.Now()
	stored.LastCheckedAt = &now
	data, _ = json.Marshal(stored)
	os.WriteFile(filepath.Join(dir, "account.json"), data, 0600)
	recheckUnavailableBrain(context.Background(), "owner", probe)
	if calls != 1 {
		t.Fatal("recent failed probe repeated")
	}
	create("owner", "unknown", old)
	stored.LastCheckedAt = &old
	data, _ = json.Marshal(stored)
	os.WriteFile(filepath.Join(dir, "account.json"), data, 0600)
	recheckUnavailableBrain(context.Background(), "owner", probe)
	if calls != 1 {
		t.Fatal("probe ran despite existing ready account")
	}
}
