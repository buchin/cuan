package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConnectionVerificationUsesProviderDefault(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	bin := t.TempDir()
	script := `#!/bin/sh
 if [ "$1" = models ]; then printf 'claude-sonnet-test\tSonnet\nclaude-opus-test\tOpus\n'; exit 0; fi
 for arg in "$@"; do [ "$arg" != --model ] || exit 31; done
 printf 'CUAN_AUTH_OK'
 `
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	a, err := CreateProviderAccount("owner", "Fixture")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := ProviderAccountHome("owner", a.ID)
	if err = os.MkdirAll(AntigravityConfigDirPath(home), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(AntigravityCredentialsFilePath(home), []byte(`{"token":{"access_token":"fixture-verification-token"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1internal:fetchAvailableModels" {
			t.Error("unexpected usage path")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"models":{"gemini-obsolete":{"quotaInfo":{"remainingFraction":1}},"gemini-test":{"quotaInfo":{"remainingFraction":0.0015}},"claude-opus-test":{"quotaInfo":{"remainingFraction":0.326}},"claude-sonnet-test":{"quotaInfo":{"remainingFraction":0.326}}}}`))
	}))
	defer server.Close()
	oldHosts, oldClient := modelUsageHosts, modelUsageClient
	modelUsageHosts = []string{server.URL}
	modelUsageClient = server.Client()
	defer func() { modelUsageHosts = oldHosts; modelUsageClient = oldClient }()
	verified, err := VerifyProviderAccount(context.Background(), "owner", a.ID)
	if err != nil || verified.State != "verified" || strings.Contains(verified.Message, " using ") {
		t.Fatal("verification used wrong default", verified, err)
	}
	if verified.AllowModelFallback {
		t.Fatal("verification enabled model fallback")
	}
}

func TestAutomaticModelRanksUnknownBelowKnownQuota(t *testing.T) {
	usage := AccountModelUsage{Available: true, Models: []ModelUsage{{Model: "claude-sonnet-test", Remaining: -1}, {Model: "claude-opus-test", Remaining: 12}, {Model: "gemini-obsolete", Remaining: 0}}}
	best, ok := AutomaticTextModel(usage, nil, time.Now())
	if !ok || best.Model != "claude-opus-test" {
		t.Fatal("unknown model outranked known quota", best, ok)
	}
	usage.Models[1].Remaining = -1
	best, ok = AutomaticTextModel(usage, nil, time.Now())
	if !ok || best.Model != "claude-sonnet-test" || best.Remaining != -1 {
		t.Fatal("unknown availability misrepresented", best, ok)
	}
	best, ok = AutomaticTextModel(usage, map[string]time.Time{"claude-sonnet-test": time.Now().Add(time.Hour)}, time.Now())
	if !ok || best.Model != "claude-opus-test" {
		t.Fatal("known cooldown ignored", best, ok)
	}
}

func TestVerificationTimeoutIsUnknownRatherThanPermanentUnavailable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\nsleep 0.15\nprintf CUAN_AUTH_OK\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	a, err := CreateProviderAccount("owner", "Fixture")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := ProviderAccountHome("owner", a.ID)
	os.MkdirAll(AntigravityConfigDirPath(home), 0700)
	os.WriteFile(AntigravityCredentialsFilePath(home), []byte("fixture-credentials"), 0600)
	a.Email = "fixture@example.invalid"
	a.State = "unavailable"
	if err := saveProviderAccount(a); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	checked, err := VerifyProviderAccount(ctx, "owner", a.ID)
	if err != nil || checked.State != "unknown" || checked.VerifiedAt != nil || !checked.CredentialsPresent {
		t.Fatal("probe timeout falsely excluded saved sign-in", checked, err)
	}
	if checked.AllowModelFallback || checked.AllowFailover {
		t.Fatal("probe changed routing permissions")
	}
	persisted, _ := GetProviderAccount("owner", a.ID)
	if persisted.State != "unknown" {
		t.Fatal("unknown readiness was not persisted")
	}
	retry := time.Now().Add(time.Hour)
	persisted.State = "quota_limited"
	persisted.QuotaRetryAt = &retry
	if err := saveProviderAccount(persisted); err != nil {
		t.Fatal(err)
	}
	quotaCtx, quotaCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer quotaCancel()
	limited, err := VerifyProviderAccount(quotaCtx, "owner", a.ID)
	if err != nil || limited.State != "quota_limited" || limited.QuotaRetryAt == nil || !limited.QuotaRetryAt.Equal(retry) {
		t.Fatal("timed out probe erased runtime quota evidence", limited, err)
	}
}
