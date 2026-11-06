package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTaskPublicationRequiresExactSavedReceipt(t *testing.T) {
	commit := strings.Repeat("a", 40)
	for _, scenario := range []string{"confirmed", "wrong commit", "wrong branch", "not pushed", "empty response", "rejected", "trailing response"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("GH_TOKEN", "AMBIENT_HOST_TOKEN")
			t.Setenv("GITHUB_TOKEN", "AMBIENT_OTHER_TOKEN")
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/task/publish" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer bound-capability" || r.ContentLength > 0 {
					t.Error("publication escaped bound capability contract")
				}
				if scenario == "rejected" {
					w.WriteHeader(409)
					return
				}
				if scenario == "empty response" {
					return
				}
				actual, branch, pushed := commit, "task/id", true
				if scenario == "wrong commit" {
					actual = strings.Repeat("b", 40)
				}
				if scenario == "wrong branch" {
					branch = "task/other"
				}
				if scenario == "not pushed" {
					pushed = false
				}
				fmt.Fprintf(w, `{"commit_sha":%q,"branch":%q,"pushed":%t,"pull_request":{"number":7,"html_url":"https://github.com/owner/repo/pull/7"}}`, actual, branch, pushed)
				if scenario == "trailing response" {
					fmt.Fprint(w, `{}`)
				}
			}))
			defer api.Close()
			r := &Runner{httpClient: api.Client()}
			result := &CompleteAttemptRequest{CommitSHA: commit}
			err := r.publishTaskExecution(context.Background(), &TaskAgentCapability{API: api.URL + "/task", Token: "bound-capability"}, result, "task/id")
			if scenario == "confirmed" {
				if err != nil || result.PRNumber != 7 {
					t.Fatal("confirmed publication lost", err, result)
				}
			} else if err == nil || result.PRNumber != 0 || result.CommitSHA != commit {
				t.Fatal("invalid receipt accepted or local result lost", err, result)
			}
		})
	}
}
