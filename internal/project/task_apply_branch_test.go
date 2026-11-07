package project

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/buchin/cuan/internal/git"
)

func TestRemoteTaskBranchApplyUsesSavedConnectionAndPreservesCache(t *testing.T) {
	for _, connected := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing access", true: "saved owner"}[connected], func(t *testing.T) {
			t.Setenv("GH_TOKEN", "wrong-host-owner")
			ctx := context.Background()
			cache := t.TempDir()
			file := filepath.Join(cache, "retained.txt")
			if err := os.WriteFile(file, []byte("pending user work"), 0600); err != nil {
				t.Fatal(err)
			}
			source, resultSHA := strings.Repeat("a", 40), strings.Repeat("b", 40)
			var calls atomic.Int32
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				var payload map[string]string
				if r.URL.Path != "/repos/owner/repo/merges" || r.Header.Get("Authorization") != "Bearer saved-owner" || json.NewDecoder(r.Body).Decode(&payload) != nil || payload["base"] != "dev" || payload["head"] != source {
					t.Error("task source, target or saved identity changed")
				}
				w.WriteHeader(201)
				json.NewEncoder(w).Encode(map[string]string{"sha": resultSHA})
			}))
			defer api.Close()
			client := git.NewClient()
			client.SetAPIBaseURL(api.URL)
			store := NewMemoryStore()
			p := &Project{ID: "project", RepoURL: "https://github.com/owner/repo.git", RepoOwner: "stale", RepoName: "metadata", DefaultBranch: "dev", LocalPath: cache}
			if err := store.CreateProject(ctx, p); err != nil {
				t.Fatal(err)
			}
			svc := NewService(store, client, "fixture", t.TempDir())
			if connected {
				svc.SetSharedRepositoryAccess(func(context.Context) (string, error) { return "saved-owner", nil })
			}
			result, err := svc.ApplyTaskBranch(ctx, p.ID, "task/one", "Apply tested work", "Original scope", source)
			if connected {
				if err != nil || result == nil || result.SHA != resultSHA || calls.Load() != 1 {
					t.Fatal("remote task branch apply failed", err)
				}
			} else if err == nil || result != nil || calls.Load() != 0 {
				t.Fatal("missing saved connection used host identity")
			}
			data, err := os.ReadFile(file)
			if err != nil || string(data) != "pending user work" {
				t.Fatal("remote publication touched retained cache")
			}
			if _, err := os.Stat(filepath.Join(cache, ".git")); !os.IsNotExist(err) {
				t.Fatal("remote publication prepared another local checkout")
			}
		})
	}
}
