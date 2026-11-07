package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeReleaseWorkspaceUsesFrozenContextAndExactDescriptor(t *testing.T) {
	for _, scenario := range []string{"valid", "wrong repository", "wrong branch", "wrong manifest", "no deploy", "no access"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			cache := t.TempDir()
			base := t.TempDir()
			if err := os.Mkdir(filepath.Join(cache, ".cuan"), 0700); err != nil {
				t.Fatal(err)
			}
			manifest := `{"version":1,"command":["release.sh"],"timeout_seconds":5}`
			for name, body := range map[string]string{".cuan/release.json": manifest, "release.sh": "#!/bin/sh\ntouch executed\n", "work.txt": "source"} {
				if err := os.WriteFile(filepath.Join(cache, name), []byte(body), 0700); err != nil {
					t.Fatal(err)
				}
			}
			run := func(args ...string) string {
				t.Helper()
				out, err := exec.Command("git", append([]string{"-C", cache}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatalf("git fixture failed: %s", out)
				}
				return strings.TrimSpace(string(out))
			}
			run("init", "--initial-branch=dev")
			run("add", ".")
			run("-c", "user.name=Fixture Human", "-c", "user.email=human@example.invalid", "commit", "-m", "Source")
			source := run("rev-parse", "HEAD")
			if err := os.WriteFile(filepath.Join(cache, "work.txt"), []byte("published result"), 0700); err != nil {
				t.Fatal(err)
			}
			run("add", ".")
			run("-c", "user.name=Fixture Human", "-c", "user.email=human@example.invalid", "commit", "-m", "Target")
			target := run("rev-parse", "HEAD")
			if err := os.WriteFile(filepath.Join(cache, "work.txt"), []byte("KEEP DEVELOPER WORK"), 0700); err != nil {
				t.Fatal(err)
			}
			state := run("status", "--porcelain=v1")
			refs := run("for-each-ref", "--format=%(refname):%(objectname)")
			store := NewMemoryStore()
			p := &Project{ID: "project", RepoURL: "https://github.com/owner/repo.git", LocalPath: cache, DefaultBranch: "dev", Metadata: json.RawMessage(`{"cuan_settings":{"workspace_revision":"initial","deploy_branch":"production","auto_deploy":true,"environment":"CUAN_MANAGED_RELEASE=1"}}`)}
			if err := store.CreateProject(ctx, p); err != nil {
				t.Fatal(err)
			}
			svc := NewService(store, nil, "fixture", base)
			accessReads := 0
			svc.SetSharedRepositoryAccess(func(context.Context) (string, error) { accessReads++; return "", nil })
			view, err := svc.Get(ctx, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			instructions := SnapshotProjectGuidance("Implement and deploy approved change", view)
			if scenario == "no deploy" {
				instructions = SnapshotProjectGuidance("Do not deploy", view)
			}
			p.Metadata = json.RawMessage(`{"cuan_settings":{"workspace_revision":"initial","deploy_branch":"changed","auto_deploy":false}}`)
			if err := store.UpdateProject(ctx, p); err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256([]byte(manifest))
			req := NativeReleaseWorkspaceRequest{ProjectID: p.ID, TaskID: "task", Instructions: instructions, Repository: "owner/repo", TargetBranch: "production", SourceCommit: source, TargetCommit: target, ManifestHash: hex.EncodeToString(hash[:])}
			switch scenario {
			case "wrong repository":
				req.Repository = "foreign/repo"
			case "wrong branch":
				req.TargetBranch = "changed"
			case "wrong manifest":
				req.ManifestHash = strings.Repeat("f", 64)
			case "no access":
				req.TargetCommit = strings.Repeat("f", 40)
			}
			result, err := svc.NativeReleaseWorkspace(ctx, req)
			if scenario == "valid" {
				if err != nil || result == nil || result.Path == cache || result.SourceCommit != source || result.TargetCommit != target || result.Branch != "production" || result.ManifestHash != req.ManifestHash {
					t.Fatal("prepared release lost frozen source chain", err)
				}
				again, reuseErr := svc.NativeReleaseWorkspace(ctx, req)
				if reuseErr != nil || again.Path != result.Path {
					t.Fatal("exact checkout not reused", reuseErr)
				}
				if _, err := os.Stat(filepath.Join(result.Path, "executed")); !os.IsNotExist(err) {
					t.Fatal("preparation executed script")
				}
			} else if err == nil || result != nil {
				t.Fatal("invalid release workspace returned")
			}
			expectedAccess := 0
			if scenario == "no access" {
				expectedAccess = 1
			}
			if accessReads != expectedAccess {
				t.Fatal("unexpected credential access", accessReads)
			}
			data, _ := os.ReadFile(filepath.Join(cache, "work.txt"))
			if string(data) != "KEEP DEVELOPER WORK" || run("status", "--porcelain=v1") != state || run("for-each-ref", "--format=%(refname):%(objectname)") != refs {
				t.Fatal("source cache modified")
			}
			if _, err := os.Stat(filepath.Join(cache, "executed")); !os.IsNotExist(err) {
				t.Fatal("cache script executed")
			}
		})
	}
}
