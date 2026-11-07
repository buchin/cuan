package project

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/buchin/cuan/internal/git"
)

func TestTaskPullRequestApplyUsesSavedAccessAndExactSource(t *testing.T) {
	for _, scenario := range []string{"saved owner", "missing connection", "unreadable connection", "connection error"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("GH_TOKEN", "wrong-ambient-owner")
			t.Setenv("GITHUB_TOKEN", "another-ambient-owner")
			var calls atomic.Int32
			source := strings.Repeat("a", 40)
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				var payload map[string]string
				if json.NewDecoder(r.Body).Decode(&payload) != nil || payload["sha"] != source || r.Header.Get("Authorization") != "Bearer saved-owner" {
					t.Error("apply lost saved identity or reviewed source")
				}
				w.Write([]byte(`{"merged":true,"sha":"provider-merge-receipt"}`))
			}))
			defer api.Close()
			client := git.NewClient()
			client.SetAPIBaseURL(api.URL)
			store := NewMemoryStore()
			p := &Project{ID: "project", RepoOwner: "owner", RepoName: "repo"}
			if scenario == "unreadable connection" {
				p.EncryptedToken = "invalid-encrypted-token"
			}
			if err := store.CreateProject(context.Background(), p); err != nil {
				t.Fatal(err)
			}
			svc := NewService(store, client, "fixture-secret", t.TempDir())
			svc.SetSharedRepositoryAccess(func(context.Context) (string, error) {
				switch scenario {
				case "missing connection":
					return "", nil
				case "connection error":
					return "", errors.New("saved connection unavailable")
				default:
					return "saved-owner", nil
				}
			})
			result, err := svc.ApplyTaskPullRequest(context.Background(), p.ID, 7, "Approved change", "Observed source", source)
			if scenario == "saved owner" {
				if err != nil || result == nil || !result.Merged || calls.Load() != 1 {
					t.Fatal("saved apply failed", err)
				}
			} else if err == nil || result != nil || calls.Load() != 0 {
				t.Fatal("missing saved access used ambient identity", err, calls.Load())
			}
		})
	}
}
