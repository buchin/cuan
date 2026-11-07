package server

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
)

func TestExplicitFallbackSettingsUseOwnedNativeCatalog(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, token := setupTestChatServer(t)
	owner, err := s.authSvc.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := agent.CreateProviderAccount(owner.ID, "Connected brain")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := agent.ProviderAccountHome(owner.ID, profile.ID)
	if err := os.MkdirAll(agent.AntigravityConfigDirPath(home), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agent.AntigravityCredentialsFilePath(home), []byte("fixture-private-credential"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := agent.UpdateProviderAccountSettings(owner.ID, profile.ID, profile.Label, "claude-sonnet-test"); err != nil {
		t.Fatal(err)
	}
	if err := agent.RecordProviderModelQuotaWithDuration(owner.ID, profile.ID, "claude-sonnet-test", time.Hour); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\n[ \"$1\" = models ] || exit 22\nprintf 'claude-sonnet-test\\ngemini-test\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	call := func(id, body, bearer string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("PATCH", "/api/v1/provider-accounts/"+id+"/model-fallback", strings.NewReader(body))
		if bearer != "" {
			r.Header.Set("Authorization", "Bearer "+bearer)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	for _, body := range []string{`{"enabled":true,"model_id":"invented-model"}`, `{"enabled":true,"model_id":"claude-sonnet-test"}`, `{"enabled":true,"model_id":""}`} {
		w := call(profile.ID, body, token)
		if w.Code != 400 && w.Code != 409 {
			t.Fatal("invalid fallback accepted", w.Code, w.Body.String())
		}
	}
	w := call(profile.ID, `{"enabled":true,"model_id":"gemini-test"}`, token)
	if w.Code != 200 || strings.Contains(w.Body.String(), "fixture-private") {
		t.Fatal("owned native fallback not saved safely", w.Code, w.Body.String())
	}
	saved, err := agent.GetProviderAccount(owner.ID, profile.ID)
	if err != nil || saved.ModelID != "claude-sonnet-test" || saved.FallbackModelID != "gemini-test" || !saved.AllowModelFallback || !time.Now().Before(saved.ModelCooldowns[saved.ModelID]) {
		t.Fatal("fallback rewrote primary selection or quota", saved, err)
	}
	if w := call(profile.ID, `{"enabled":true,"model_id":"gemini-test"}`, ""); w.Code != 401 {
		t.Fatal("unauthenticated policy update", w.Code)
	}
	other, err := agent.CreateProviderAccount("other-owner", "Other")
	if err != nil {
		t.Fatal(err)
	}
	if w := call(other.ID, `{"enabled":true,"model_id":"gemini-test"}`, token); w.Code != 404 {
		t.Fatal("foreign account settings changed", w.Code)
	}
	if w := call(profile.ID, `{"enabled":false,"model_id":""}`, token); w.Code != 200 {
		t.Fatal("fallback disable failed", w.Code)
	}
	saved, err = agent.GetProviderAccount(owner.ID, profile.ID)
	if err != nil || saved.AllowModelFallback || saved.FallbackModelID != "" || saved.ModelID != "claude-sonnet-test" {
		t.Fatal("fallback disable changed primary", saved, err)
	}
}
