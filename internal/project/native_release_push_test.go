package project

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestNativeReleasePushRejectsScopeAndReleaseBranchBeforeAccess(t *testing.T) {
	for _, scenario := range []string{"no push", "no deploy", "release branch", "foreign repo", "disabled", "missing connection"} {
		t.Run(scenario, func(t *testing.T) {
			store := NewMemoryStore()
			p := &Project{ID: "project", RepoURL: "https://github.com/owner/repo.git", DefaultBranch: "dev", Metadata: json.RawMessage(`{"cuan_settings":{"workspace_revision":"initial","auto_deploy":true,"deploy_branch":"production","environment":"CUAN_MANAGED_RELEASE=1"}}`)}
			if scenario == "disabled" {
				p.Metadata = json.RawMessage(`{"cuan_settings":{"workspace_revision":"initial","auto_deploy":false}}`)
			}
			if err := store.CreateProject(context.Background(), p); err != nil {
				t.Fatal(err)
			}
			svc := NewService(store, nil, "fixture", t.TempDir())
			reads := 0
			svc.SetSharedRepositoryAccess(func(context.Context) (string, error) { reads++; return "", nil })
			instructions, branch, repo := "Implement and deploy approved change", "task/source", "owner/repo"
			switch scenario {
			case "no push":
				instructions = "Do not push"
			case "no deploy":
				instructions = "Do not deploy"
			case "release branch":
				branch = "production"
			case "foreign repo":
				repo = "foreign/repo"
			}
			result, err := svc.PushNativeReleaseSource(context.Background(), p.ID, instructions, repo, t.TempDir(), branch, strings.Repeat("a", 40))
			expectedReads := 0
			if scenario == "missing connection" {
				expectedReads = 1
			}
			if err == nil || result != "" || reads != expectedReads {
				t.Fatal("unapproved native source reached saved access", err, reads)
			}
		})
	}
}
