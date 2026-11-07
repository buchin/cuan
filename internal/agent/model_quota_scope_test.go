package agent

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestModelQuotaResetKeepsOtherModelUsableInNativePool(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	p, err := CreateProviderAccount("owner", "Connected brain")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := ProviderAccountHome("owner", p.ID)
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
	if _, err = UpdateProviderAccountSettings("owner", p.ID, p.Label, "gemini-test"); err != nil {
		t.Fatal(err)
	}
	if err = RecordProviderModelQuotaWithDuration("owner", p.ID, "gemini-test", 40*time.Hour); err != nil {
		t.Fatal(err)
	}
	actual, err := GetProviderAccount("owner", p.ID)
	if err != nil || actual.State == "quota_limited" || time.Until(actual.ModelCooldowns["gemini-test"]) < 39*time.Hour {
		t.Fatal("model reset became account block or lost duration", actual, err)
	}
	if _, _, release, err := LeaseHealthyBrainForTask("owner", nil); !errors.Is(err, ErrBrainCapacity) {
		if release != nil {
			release()
		}
		t.Fatal("known exhausted model admitted", err)
	}
	if _, err = UpdateProviderAccountSettings("owner", p.ID, p.Label, "claude-sonnet-test"); err != nil {
		t.Fatal(err)
	}
	_, selected, release, err := LeaseHealthyBrainForTask("owner", nil)
	if err != nil {
		t.Fatal("Gemini quota blocked Sonnet", err)
	}
	if selected.ID != p.ID || selected.ModelID != "claude-sonnet-test" {
		t.Fatal("wrong route", selected)
	}
	release()
	if err = RecordProviderModelQuotaWithDuration("owner", p.ID, "", time.Hour); err != nil {
		t.Fatal(err)
	}
	_, _, release, err = LeaseHealthyBrainForTask("owner", nil)
	if err != nil {
		t.Fatal("Auto cooldown blocked explicit Sonnet", err)
	}
	release()
	if _, err = UpdateProviderAccountSettings("owner", p.ID, p.Label, ""); err != nil {
		t.Fatal(err)
	}
	if _, _, release, err := LeaseHealthyBrainForTask("owner", nil); !errors.Is(err, ErrBrainCapacity) {
		if release != nil {
			release()
		}
		t.Fatal("cooled Auto route admitted", err)
	}
}
