package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAccountModelUsageReadOnlyAndCache(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a, err := CreateProviderAccount("owner", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := ProviderAccountHome("owner", a.ID)
	os.MkdirAll(AntigravityConfigDirPath(home), 0700)
	credential := []byte(`{"token":{"access_token":"fixture-private-token"}}`)
	file := AntigravityCredentialsFilePath(home)
	os.WriteFile(file, credential, 0600)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/v1internal:fetchAvailableModels" || r.Header.Get("Authorization") != "Bearer fixture-private-token" {
			t.Error("wrong usage request")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"models":{"gemini-test":{"quotaInfo":{"remainingFraction":0,"resetTime":"2030-01-01T00:00:00Z"}},"claude-sonnet-test":{"quotaInfo":{"remainingFraction":0.8}},"gemini-image":{"quotaInfo":{"remainingFraction":1}},"bad":{"quotaInfo":{"remainingFraction":9}}}}`))
	}))
	defer server.Close()
	oldHosts, oldClient := modelUsageHosts, modelUsageClient
	modelUsageHosts = []string{server.URL}
	modelUsageClient = server.Client()
	defer func() { modelUsageHosts = oldHosts; modelUsageClient = oldClient }()
	result := FetchAccountModelUsage(context.Background(), "owner", a.ID)
	if !result.Available || len(result.Models) != 2 || result.Models[0].Model != "claude-sonnet-test" || result.Models[0].Remaining != 80 {
		t.Fatal(result)
	}
	FetchAccountModelUsage(context.Background(), "owner", a.ID)
	if calls != 1 {
		t.Fatal("usage not cached", calls)
	}
	if result := FetchAccountModelUsage(context.Background(), "other", a.ID); result.Available {
		t.Fatal("cross-owner usage")
	}
	actual, _ := os.ReadFile(file)
	if string(actual) != string(credential) {
		t.Fatal("usage changed credentials")
	}
	if strings.Contains(result.Message, "private") {
		t.Fatal("credential leaked")
	}
}
func TestAccountModelCooldownAndPolicies(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a, _ := CreateProviderAccount("owner", "fixture")
	home, _ := ProviderAccountHome("owner", a.ID)
	os.MkdirAll(AntigravityConfigDirPath(home), 0700)
	os.WriteFile(AntigravityCredentialsFilePath(home), []byte("fixture-credential"), 0600)
	if err := BindProviderAccount("owner", a.ID, "agent", "conn_profile_fixture"); err != nil {
		t.Fatal(err)
	}
	SetProviderAccountFailover("owner", a.ID, true)
	SetProviderAccountModelFallback("owner", a.ID, true)
	if err := RecordProviderModelQuota("owner", a.ID, "gemini-test"); err != nil {
		t.Fatal(err)
	}
	actual, _ := GetProviderAccount("owner", a.ID)
	if actual.State == "quota_limited" || !actual.AllowFailover || !actual.AllowModelFallback || !time.Now().Before(actual.ModelCooldowns["gemini-test"]) {
		t.Fatal(actual)
	}
	if actual.AgentID != "agent" || actual.ConnectionID != "conn_profile_fixture" {
		t.Fatal("binding changed")
	}
	os.Remove(AntigravityCredentialsFilePath(home))
	if _, err := SetProviderAccountFailover("owner", a.ID, false); err != nil {
		t.Fatal("could not revoke with expired login", err)
	}
}

func TestAccountLeaseAndOwnerIsolation(t *testing.T) {
	t.Setenv("CUAN_ACCOUNT_MAX_SESSIONS", "1")
	t.Setenv("HOME", t.TempDir())
	a, _ := CreateProviderAccount("owner", "fixture")
	release, err := TryLeaseProviderAccount("owner", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := TryLeaseProviderAccount("owner", a.ID); err == nil {
		t.Fatal("busy account leased twice")
	}
	if _, err := TryLeaseProviderAccount("other", a.ID); err == nil {
		t.Fatal("cross-owner lease")
	}
	release()
	release()
	next, err := TryLeaseProviderAccount("owner", a.ID)
	if err != nil {
		t.Fatal("lease not released", err)
	}
	next()
}
