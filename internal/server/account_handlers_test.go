package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/auth"
)

func TestS26AccountAPIAuthOwnerAndQuotaIsolation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s, token := setupTestChatServer(t)
	request := func(method, path, body, token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		s.Router().ServeHTTP(w, r)
		return w
	}
	for _, route := range []string{"/api/v1/provider-accounts", "/api/v1/provider-accounts/acct_00000000000000000000000000000000/auth-init"} {
		method := "POST"
		if route == "/api/v1/provider-accounts" {
			method = "GET"
		}
		w := request(method, route, `{}`, "")
		if w.Code != 401 {
			t.Fatalf("owner gate %d", w.Code)
		}
	}
	w := request("POST", "/api/v1/provider-accounts", `{"label":"Google kerja"}`, token)
	if w.Code != 201 {
		t.Fatalf("create %d %s", w.Code, w.Body.String())
	}
	var response struct {
		Account agent.ProviderAccount `json:"account"`
	}
	if json.Unmarshal(w.Body.Bytes(), &response) != nil {
		t.Fatal("bad create response")
	}
	id := response.Account.ID
	// A different authenticated owner sees no accounts and cannot operate this one.
	store := auth.NewMemoryStore()
	service := auth.NewService(store)
	other, err := service.Setup(context.Background(), "other@example.test", "other-password-fixture")
	if err != nil {
		t.Fatal(err)
	}
	original := s.authSvc
	s.authSvc = service
	w = request("POST", "/api/v1/provider-accounts/"+id+"/auth-init", `{}`, other.Token)
	if w.Code != 404 {
		t.Fatalf("owner isolation %d", w.Code)
	}
	s.authSvc = original
	// Existing service credential and quota must not prevent a fresh isolated login.
	credential := filepath.Join(t.TempDir(), "legacy-credential")
	os.WriteFile(credential, []byte("legacy-fixture-credential-value"), 0600)
	t.Setenv("ANTIGRAVITY_TOKEN_PATH", credential)
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	cli := `#!/bin/sh
if [ "$1" = "--print" ]; then echo '429 quota private-marker' >&2; exit 1; fi
printf 'https://accounts.google.com/o/oauth2/v2/auth?client_id=fixture&redirect_uri=https%%3A%%2F%%2Fantigravity.google%%2Foauth-callback&response_type=code\n'
while read value; do :; done
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(cli), 0700); err != nil {
		t.Fatal(err)
	}
	w = request("POST", "/api/v1/provider-accounts/"+id+"/auth-init", `{}`, token)
	if w.Code != 200 {
		t.Fatalf("new account blocked by legacy quota: %d %s", w.Code, w.Body.String())
	}
	var session struct {
		ID string `json:"session_id"`
	}
	json.Unmarshal(w.Body.Bytes(), &session)
	if session.ID == "" {
		t.Fatal("no OAuth session")
	}
	defer agent.CancelAntigravityAuthSession(session.ID)
	w = request("POST", "/api/v1/agents/auth-code?cancel=true", `{"session_id":"`+session.ID+`"}`, token)
	if w.Code != 404 {
		t.Fatal("legacy endpoint cancelled scoped session")
	}
	w = request("POST", "/api/v1/provider-accounts/"+id+"/auth-code?cancel=true", `{"session_id":"`+session.ID+`"}`, token)
	if w.Code != 200 {
		t.Fatalf("cancel %d", w.Code)
	}
	if data, _ := os.ReadFile(credential); string(data) != "legacy-fixture-credential-value" {
		t.Fatal("legacy credential changed")
	}
	w = request("GET", "/api/v1/provider-accounts", "", token)
	if w.Code != 200 || strings.Contains(w.Body.String(), "private-marker") || strings.Contains(w.Body.String(), "legacy-fixture") {
		t.Fatal("account DTO unsafe")
	}
}
