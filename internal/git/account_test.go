package git

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAccountDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("missing authentication")
		}
		if r.URL.Path == "/user" {
			w.Write([]byte(`{"login":"owner","name":"Owner"}`))
			return
		}
		if r.URL.Path != "/user/repos" || r.URL.Query().Get("page") != "2" {
			t.Error("wrong pagination")
		}
		w.Write([]byte(`[{"full_name":"owner/app","default_branch":"trunk","private":true,"permissions":{"push":false}}]`))
	}))
	defer server.Close()
	client := NewClient()
	client.SetAPIBaseURL(server.URL)
	account, err := client.Account(context.Background(), "fixture")
	if err != nil || account.Login != "owner" {
		t.Fatal(account, err)
	}
	repos, err := client.AccountRepositories(context.Background(), "fixture", 2)
	if err != nil || len(repos) != 1 || repos[0].DefaultBranch != "trunk" || repos[0].Permissions.Push {
		t.Fatal(repos, err)
	}
	if _, err = client.Account(context.Background(), ""); !errors.Is(err, ErrAccessDenied) {
		t.Fatal("empty credential accepted")
	}
}
func TestAccountDiscoveryRejectsRedirect(t *testing.T) {
	reached := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }))
	defer source.Close()
	client := NewClient()
	client.SetAPIBaseURL(source.URL)
	if _, err := client.Account(context.Background(), "fixture"); err == nil || reached {
		t.Fatal("credential request followed redirect")
	}
}
