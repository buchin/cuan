package agent

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBrainPoolPreferredBeforeExplicitFallback(t *testing.T) {
	for _, mode := range []string{"preferred", "fallback", "both limited", "disabled", "catalog missing", "account limited", "policy revoked", "route exclusion", "preferred restored"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			bin := t.TempDir()
			catalog := "claude-sonnet-test\ngemini-test\n"
			if mode == "catalog missing" {
				catalog = "claude-sonnet-test\n"
			}
			if err := os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\n[ \"$1\" = models ] || exit 40\nprintf '"+catalog+"'\n"), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
			profiles := []*ProviderAccount{}
			for i := 0; i < 2; i++ {
				p, err := CreateProviderAccount("owner", "Connected brain")
				if err != nil {
					t.Fatal(err)
				}
				home, _ := ProviderAccountHome("owner", p.ID)
				if err := os.MkdirAll(AntigravityConfigDirPath(home), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(AntigravityCredentialsFilePath(home), []byte("fixture-only-credential"), 0600); err != nil {
					t.Fatal(err)
				}
				p.State, p.ModelID, p.FallbackModelID, p.AllowModelFallback = "unknown", "claude-sonnet-test", "gemini-test", true
				p.ModelCooldowns = map[string]time.Time{}
				if i == 0 || mode != "preferred" {
					p.ModelCooldowns[p.ModelID] = time.Now().Add(time.Hour)
				}
				if mode == "preferred restored" {
					p.ModelCooldowns[p.ModelID] = time.Now().Add(-time.Second)
				}
				if mode == "both limited" {
					p.ModelCooldowns[p.FallbackModelID] = time.Now().Add(time.Hour)
				}
				if mode == "disabled" {
					p.AllowModelFallback = false
				}
				if mode == "account limited" {
					p.State = "quota_limited"
					until := time.Now().Add(time.Hour)
					p.QuotaRetryAt = &until
				}
				if err := saveProviderAccount(p); err != nil {
					t.Fatal(err)
				}
				profiles = append(profiles, p)
			}
			excluded := map[string]bool{}
			if mode == "route exclusion" {
				for _, p := range profiles {
					excluded[BrainRouteKey(p.ID, p.ModelID)] = true
				}
			}
			lease := TryLeaseProviderAccountForTask
			if mode == "policy revoked" {
				lease = func(owner, id string) (func(), error) {
					if _, err := SetProviderAccountModelFallback(owner, id, false); err != nil {
						return nil, err
					}
					return TryLeaseProviderAccountForTask(owner, id)
				}
			}
			_, selected, release, err := leaseHealthyBrain("owner", excluded, lease)
			wantFailure := mode == "both limited" || mode == "disabled" || mode == "catalog missing" || mode == "account limited" || mode == "policy revoked"
			if wantFailure {
				if release != nil {
					release()
				}
				if !errors.Is(err, ErrBrainCapacity) {
					t.Fatal("unavailable/denied route admitted", selected, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			release()
			want := "gemini-test"
			if mode == "preferred" || mode == "preferred restored" {
				want = "claude-sonnet-test"
			}
			if selected.ExecutionModelID() != want {
				t.Fatal("model priority violated", selected.ExecutionModelID(), want)
			}
			saved, err := GetProviderAccount("owner", selected.ID)
			if err != nil || saved.ModelID != "claude-sonnet-test" || saved.RuntimeModelID != "" || saved.FallbackModelID != "gemini-test" {
				t.Fatal("route lease changed saved preference", saved, err)
			}
		})
	}
}
