package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestUsageRefreshExpiredTokenAtomically(t *testing.T) {
	for _, scenario := range []string{"success", "rejected", "busy", "forced-401"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			a, _ := CreateProviderAccount("owner", "refresh fixture")
			home, _ := ProviderAccountHome("owner", a.ID)
			os.MkdirAll(AntigravityConfigDirPath(home), 0700)
			expiry := "2020-01-01T00:00:00Z"
			if scenario == "forced-401" {
				expiry = "2099-01-01T00:00:00Z"
			}
			original := []byte(`{"auth_method":"unchanged","id_token":"identity-marker","token":{"access_token":"fixture-old-token","refresh_token":"fixture-refresh-token","expiry":"` + expiry + `"}}`)
			file := AntigravityCredentialsFilePath(home)
			os.WriteFile(file, original, 0600)
			refreshes, quotas := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/token" {
					refreshes++
					r.ParseForm()
					if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "fixture-refresh-token" {
						t.Error("wrong refresh grant")
					}
					if scenario == "rejected" {
						w.WriteHeader(400)
						w.Write([]byte(`{"error":"invalid_grant","error_description":"private-details"}`))
						return
					}
					w.Write([]byte(`{"access_token":"fixture-new-token","expires_in":3600,"token_type":"Bearer"}`))
					return
				}
				quotas++
				if r.Header.Get("Authorization") == "Bearer fixture-old-token" {
					w.WriteHeader(401)
					return
				}
				w.Write([]byte(`{"models":{"claude-sonnet-test":{"quotaInfo":{"remainingFraction":0.8}}}}`))
			}))
			defer server.Close()
			oldHosts, oldClient, oldURL := modelUsageHosts, modelUsageClient, usageOAuthURL
			modelUsageHosts = []string{server.URL}
			modelUsageClient = server.Client()
			usageOAuthURL = server.URL + "/token"
			defer func() { modelUsageHosts = oldHosts; modelUsageClient = oldClient; usageOAuthURL = oldURL }()
			if scenario == "busy" {
				release, err := TryLeaseProviderAccount("owner", a.ID)
				if err != nil {
					t.Fatal(err)
				}
				defer release()
			}
			result := FetchAccountModelUsage(context.Background(), "owner", a.ID)
			actual, _ := os.ReadFile(file)
			if scenario == "success" || scenario == "forced-401" {
				if !result.Available || refreshes != 1 || quotas < 1 {
					t.Fatal("refresh/usage not completed", result, refreshes, quotas)
				}
				var v map[string]any
				json.Unmarshal(actual, &v)
				tokens := v["token"].(map[string]any)
				if v["auth_method"] != "unchanged" || v["id_token"] != "identity-marker" || tokens["refresh_token"] != "fixture-refresh-token" || tokens["access_token"] != "fixture-new-token" {
					t.Fatal("credential identity or refresh token lost")
				}
				info, _ := os.Stat(file)
				if info.Mode().Perm() != 0600 {
					t.Fatal("unsafe credential permissions")
				}
			} else {
				if result.Available || string(actual) != string(original) {
					t.Fatal("failed/busy refresh changed credentials")
				}
				if scenario == "busy" && refreshes != 0 {
					t.Fatal("refreshed busy account")
				}
			}
		})
	}
}

func TestUsageRefreshRejectedGrantCooldown(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a, _ := CreateProviderAccount("owner", "cooldown fixture")
	home, _ := ProviderAccountHome("owner", a.ID)
	os.MkdirAll(AntigravityConfigDirPath(home), 0700)
	os.WriteFile(AntigravityCredentialsFilePath(home), []byte(`{"token":{"access_token":"fixture-old-token","refresh_token":"fixture-refresh-token","expiry":"2020-01-01T00:00:00Z"}}`), 0600)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"invalid_grant"}`))
	}))
	defer server.Close()
	oldClient, oldURL := modelUsageClient, usageOAuthURL
	modelUsageClient = server.Client()
	usageOAuthURL = server.URL
	defer func() { modelUsageClient = oldClient; usageOAuthURL = oldURL }()
	FetchAccountModelUsage(context.Background(), "owner", a.ID)
	FetchAccountModelUsage(context.Background(), "owner", a.ID)
	if calls != 1 {
		t.Fatal("rejected refresh was retried too quickly", calls)
	}
}
