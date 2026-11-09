package project

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/git"
)

func TestNativeTaskPushPreservesRemoteHistoryAfterReconciliation(t *testing.T) {
	for _, scenario := range []string{"rebased", "dirty", "policy rejection"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			gitBin, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			run := func(args ...string) string {
				t.Helper()
				out, e := exec.Command(gitBin, args...).CombinedOutput()
				if e != nil {
					t.Fatalf("fixture git: %v %s", e, out)
				}
				return strings.TrimSpace(string(out))
			}
			base := filepath.Join(root, "remotes")
			remote := filepath.Join(base, "owner", "repo.git")
			if err := os.MkdirAll(filepath.Dir(remote), 0700); err != nil {
				t.Fatal(err)
			}
			run("init", "--bare", remote)
			local := filepath.Join(root, "task")
			run("init", "--initial-branch=task/source", local)
			run("-C", local, "config", "user.name", "Fixture Human")
			run("-C", local, "config", "user.email", "human@example.invalid")
			if err := os.WriteFile(filepath.Join(local, "result.txt"), []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			run("-C", local, "add", ".")
			run("-C", local, "commit", "-m", "Original")
			original := run("-C", local, "rev-parse", "HEAD")
			run("-C", local, "push", remote, "HEAD:refs/heads/task/source")
			run("-C", local, "checkout", "--orphan", "reconciled")
			if err := os.WriteFile(filepath.Join(local, "result.txt"), []byte("reconciled"), 0600); err != nil {
				t.Fatal(err)
			}
			run("-C", local, "add", ".")
			run("-C", local, "commit", "-m", "Reconciled baseline")
			run("-C", local, "branch", "-M", "task/source")
			source := run("-C", local, "rev-parse", "HEAD")
			run("-C", local, "remote", "add", "origin", "https://github.com/owner/repo.git")
			if scenario == "dirty" {
				if err := os.WriteFile(filepath.Join(local, "pending.txt"), []byte("preserve"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "policy rejection" {
				if err := os.WriteFile(filepath.Join(remote, "hooks", "pre-receive"), []byte("#!/bin/sh\necho 'GH013: Repository rule violations' >&2\nexit 1\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			apiCalls := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { apiCalls++; w.WriteHeader(500) }))
			defer api.Close()
			client := git.NewClient()
			client.SetGitBaseURL(base)
			client.SetAPIBaseURL(api.URL)
			store := NewMemoryStore()
			if err := store.CreateProject(context.Background(), &Project{ID: "project", RepoURL: "https://github.com/owner/repo.git", DefaultBranch: "dev", LocalPath: local}); err != nil {
				t.Fatal(err)
			}
			svc := NewService(store, client, "fixture", root)
			svc.SetSharedRepositoryAccess(func(context.Context) (string, error) { return "saved-project-access", nil })
			result, pr, pushErr := svc.PushNativeTaskBranch(context.Background(), "task", "project", "Save the approved source without changing existing history", local, "task/source", true, "Reconciled task")
			if scenario == "rebased" {
				if pushErr != nil || result != source || pr != nil {
					t.Fatal("reconciled source was not retained independently", result, pr, pushErr)
				}
				if got := run("--git-dir", remote, "rev-parse", "refs/heads/cuan/verified-source/"+source); got != source {
					t.Fatal("wrong immutable source", got)
				}
				// A repeated completion observes the same immutable source rather than overwriting history.
				if again, _, err := svc.PushNativeTaskBranch(context.Background(), "task", "project", "Save approved source", local, "task/source", true, "Reconciled task"); err != nil || again != source {
					t.Fatal("source reconciliation is not repeatable", again, err)
				}
			} else if pushErr == nil || result != "" || pr != nil {
				t.Fatal("unverified or policy-rejected source published", result, pr, pushErr)
			}
			if got := run("--git-dir", remote, "rev-parse", "refs/heads/task/source"); got != original {
				t.Fatal("existing remote history overwritten", got)
			}
			if apiCalls != 0 {
				t.Fatal("created a PR for an older branch", apiCalls)
			}
			if scenario != "rebased" {
				if refs := run("--git-dir", remote, "for-each-ref", "--format=%(refname)", "refs/heads/cuan/verified-source/"); refs != "" {
					t.Fatal("unexpected immutable source mutation", refs)
				}
			}
			if scenario == "dirty" {
				if raw, err := os.ReadFile(filepath.Join(local, "pending.txt")); err != nil || string(raw) != "preserve" {
					t.Fatal("pending source lost", err)
				}
			}
		})
	}
}
