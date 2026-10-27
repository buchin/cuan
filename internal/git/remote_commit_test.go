package git

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRemoteBranchHeadUsesLiveRef(t *testing.T) {
	sha := strings.Repeat("a", 40)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/git/ref/heads/cuan/task-one" || r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("wrong ref or credential")
		}
		w.Write([]byte(`{"object":{"sha":"` + sha + `"}}`))
	}))
	defer api.Close()
	c := NewClient()
	c.SetAPIBaseURL(api.URL)
	got, e := c.RemoteBranchHead(context.Background(), "owner/repo", "cuan/task-one", "fixture")
	if e != nil || got != sha {
		t.Fatal(got, e)
	}
	sha = "not-a-commit"
	if _, e = c.RemoteBranchHead(context.Background(), "owner/repo", "cuan/task-one", "fixture"); e == nil {
		t.Fatal("invalid remote commit accepted")
	}
}
