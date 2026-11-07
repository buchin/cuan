package project

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/git"
)

func TestNativeReleaseApplyUsesFrozenBranchAndSavedOwner(t *testing.T) {
	for _, scenario := range []string{"valid", "foreign repo", "wrong branch", "no deploy", "no access", "disabled"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			store := NewMemoryStore()
			posts, reads := 0, 0
			source, merged := strings.Repeat("a", 40), strings.Repeat("b", 40)
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer saved-owner" {
					t.Error("ambient identity used")
				}
				if r.Method == http.MethodGet {
					reads++
					if strings.Contains(r.URL.Path, "/commits/") {
						if !strings.HasSuffix(r.URL.Path, "/release") {
							t.Error("observation ignored frozen target")
						}
						json.NewEncoder(w).Encode(map[string]string{"sha": merged})
						return
					}
					json.NewEncoder(w).Encode(map[string]any{"status": "diverged", "merge_base_commit": map[string]string{"sha": strings.Repeat("c", 40)}})
					return
				}
				posts++
				var payload map[string]string
				if json.NewDecoder(r.Body).Decode(&payload) != nil || payload["base"] != "release" || payload["head"] != source || r.URL.Path != "/repos/owner/repo/merges" {
					t.Error("merge lost frozen source/target")
				}
				w.WriteHeader(201)
				json.NewEncoder(w).Encode(map[string]string{"sha": merged})
			}))
			defer api.Close()
			client := git.NewClient()
			client.SetAPIBaseURL(api.URL)
			p := &Project{ID: "project", RepoURL: "https://github.com/owner/repo.git", DefaultBranch: "dev", Metadata: json.RawMessage(`{"cuan_settings":{"workspace_revision":"initial","deploy_branch":"release","auto_deploy":true,"environment":"CUAN_MANAGED_RELEASE=1"}}`)}
			if scenario == "disabled" {
				p.Metadata = json.RawMessage(`{"cuan_settings":{"workspace_revision":"initial","deploy_branch":"release","auto_deploy":false,"environment":"CUAN_MANAGED_RELEASE=1"}}`)
			}
			if err := store.CreateProject(ctx, p); err != nil {
				t.Fatal(err)
			}
			svc := NewService(store, client, "fixture", t.TempDir())
			svc.SetSharedRepositoryAccess(func(context.Context) (string, error) {
				if scenario == "no access" {
					return "", nil
				}
				return "saved-owner", nil
			})
			view, err := svc.Get(ctx, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			instructions := SnapshotProjectGuidance("Implement and deploy the approved change", view)
			// Change current configuration after admission; the task still targets release.
			p.Metadata = json.RawMessage(`{"cuan_settings":{"workspace_revision":"initial","deploy_branch":"other","auto_deploy":false}}`)
			if err := store.UpdateProject(ctx, p); err != nil {
				t.Fatal(err)
			}
			repository, target := "owner/repo", "release"
			switch scenario {
			case "foreign repo":
				repository = "foreign/repo"
			case "wrong branch":
				target = "other"
			case "no deploy":
				instructions = SnapshotProjectGuidance("Do not deploy", view)
			}
			t.Setenv("GH_TOKEN", "wrong-host-owner")
			result, err := svc.ApplyNativeReleaseSource(ctx, p.ID, instructions, repository, target, source)
			if scenario == "valid" {
				if err != nil || result == nil || result.SHA != merged || posts != 1 || reads != 2 {
					t.Fatalf("frozen merge failed: %v, reads=%d writes=%d", err, reads, posts)
				}
			} else if err == nil || result != nil || posts != 0 || reads != 0 {
				t.Fatalf("unauthorized publication occurred: %v %d/%d", err, reads, posts)
			}
		})
	}
}
