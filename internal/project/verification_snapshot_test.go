package project

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectVerificationSnapshotUsesOwnedConnectionAndPreservesWork(t *testing.T) {
	for _, denied := range []bool{false, true} {
		t.Run(map[bool]string{false: "connected", true: "denied"}[denied], func(t *testing.T) {
			root := t.TempDir()
			realGit, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			run := func(directory string, args ...string) string {
				t.Helper()
				out, err := exec.Command(realGit, append([]string{"-C", directory}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatal(err, string(out))
				}
				return strings.TrimSpace(string(out))
			}
			remote, seed, cache := filepath.Join(root, "remote.git"), filepath.Join(root, "seed"), filepath.Join(root, "cache")
			run(root, "init", "--bare", remote)
			run(root, "init", "--initial-branch=main", seed)
			run(seed, "config", "user.name", "Fixture")
			run(seed, "config", "user.email", "fixture@example.invalid")
			if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("SOURCE"), 0600); err != nil {
				t.Fatal(err)
			}
			run(seed, "add", ".")
			run(seed, "commit", "-m", "Source")
			original := run(seed, "rev-parse", "HEAD")
			run(seed, "push", remote, "main")
			run(root, "clone", "--branch", "main", remote, cache)
			if err := os.WriteFile(filepath.Join(cache, "README.md"), []byte("KEEP_PENDING"), 0600); err != nil {
				t.Fatal(err)
			}
			run(cache, "add", "README.md")
			indexBefore, err := os.ReadFile(filepath.Join(cache, ".git", "index"))
			if err != nil {
				t.Fatal(err)
			}
			refs := run(cache, "for-each-ref", "--format=%(refname):%(objectname)")
			if err := os.WriteFile(filepath.Join(seed, "feature.txt"), []byte("LIVE_RESULT"), 0600); err != nil {
				t.Fatal(err)
			}
			run(seed, "add", ".")
			run(seed, "commit", "-m", "Published result")
			live := run(seed, "rev-parse", "HEAD")
			run(seed, "push", remote, "main")
			quote := func(value string) string { raw, _ := json.Marshal(value); return string(raw) }
			bin, calls := filepath.Join(root, "bin"), filepath.Join(root, "fetches")
			if err := os.Mkdir(bin, 0700); err != nil {
				t.Fatal(err)
			}
			script := "#!/usr/bin/python3\nimport os,sys,base64\na=sys.argv[1:]\nif 'fetch' in a and 'https://github.com/fixture/repo.git' in a:\n assert not os.environ.get('GH_TOKEN') and not os.environ.get('GITHUB_TOKEN') and not os.environ.get('SSH_AUTH_SOCK')\n assert os.environ.get('GIT_CONFIG_KEY_0')=='http.https://github.com/.extraheader'\n assert base64.b64decode(os.environ['GIT_CONFIG_VALUE_0'].split('Basic ')[1]).decode()=='x-access-token:fixture-project-token'\n assert not any('fixture-project-token' in x for x in a)\n assert '--no-write-fetch-head' in a and '--no-auto-maintenance' in a and '--recurse-submodules=no' in a\n with open(" + quote(calls) + ",'a') as f:f.write('fetch\\n')\n"
			if denied {
				script += " sys.stderr.write('authentication failed\\n')\n sys.exit(128)\n"
			}
			script += " a=[" + quote(remote) + " if x=='https://github.com/fixture/repo.git' else x for x in a]\nos.execv(" + quote(realGit) + ",['git']+a)\n"
			if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
			for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN", "SSH_AUTH_SOCK"} {
				t.Setenv(key, "unrelated-host-identity")
			}
			store := NewMemoryStore()
			if err := store.CreateProject(context.Background(), &Project{ID: "fixture", Name: "Fixture", RepoURL: "https://github.com/fixture/repo", LocalPath: cache, DefaultBranch: "main"}); err != nil {
				t.Fatal(err)
			}
			service := NewService(store, nil, "fixture-key", filepath.Join(root, "projects"))
			service.SetSharedRepositoryAccess(func(context.Context) (string, error) { return "fixture-project-token", nil })
			if !denied {
				contained, err := service.PublishedCommitContained(context.Background(), "fixture", original, live)
				if err != nil || !contained {
					t.Fatal("owned connection did not verify contained publication", err)
				}
			}
			path, err := service.VerificationSnapshot(context.Background(), "fixture", "task", original, live)
			if denied {
				if err == nil || !strings.Contains(err.Error(), "Connect GitHub") || path != "" {
					t.Fatal("failed project connection became a ready snapshot", err)
				}
			} else {
				if err != nil || run(path, "rev-parse", "HEAD") != live {
					t.Fatal("project did not prepare the exact published revision", err)
				}
				service.SetSharedRepositoryAccess(func(context.Context) (string, error) { return "", errors.New("connection no longer available") })
				if contained, err := service.PublishedCommitContained(context.Background(), "fixture", original, live); err != nil || !contained {
					t.Fatal("cached immutable ancestry required another sign-in", err)
				}
				if again, err := service.VerificationSnapshot(context.Background(), "fixture", "task", original, live); err != nil || again != path {
					t.Fatal("existing snapshot unnecessarily required another sign-in/fetch", err)
				}
			}
			if data, err := os.ReadFile(filepath.Join(cache, "README.md")); err != nil || string(data) != "KEEP_PENDING" {
				t.Fatal("snapshot fetching overwrote pending work", err)
			}
			if index, err := os.ReadFile(filepath.Join(cache, ".git", "index")); err != nil || !bytes.Equal(indexBefore, index) || run(cache, "rev-parse", "HEAD") != original || run(cache, "for-each-ref", "--format=%(refname):%(objectname)") != refs {
				t.Fatal("snapshot fetch moved source HEAD/index/refs", err)
			}
			if data, err := os.ReadFile(calls); err != nil || string(data) != "fetch\n" {
				t.Fatal("snapshot duplicated the fetch", err)
			}
			if !denied {
				binding, e := os.ReadFile(filepath.Join(path, ".git", "cuan-verification.json"))
				if e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(filepath.Join(path, ".git", "cuan-verification-cold.json"), binding, 0600); e != nil {
					t.Fatal(e)
				}
				run(path, "sparse-checkout", "set", "--no-cone", "!*")
				// Retain the fixture's unique cache work elsewhere, making its old
				// disposable path unavailable. No repository connection remains.
				if e = os.Rename(cache, cache+"-retained"); e != nil {
					t.Fatal(e)
				}
				if restored, e := service.VerificationSnapshot(context.Background(), "fixture", "task", original, live); e != nil || restored != path {
					t.Fatal("project inspection depended on disposable cache", e)
				}
				if data, e := os.ReadFile(filepath.Join(path, "feature.txt")); e != nil || string(data) != "LIVE_RESULT" {
					t.Fatal("project inspection failed to restore exact source", e)
				}
				if data, e := os.ReadFile(filepath.Join(cache+"-retained", "README.md")); e != nil || string(data) != "KEEP_PENDING" {
					t.Fatal("disposable-cache move lost unique fixture work", e)
				}
				if data, e := os.ReadFile(calls); e != nil || string(data) != "fetch\n" {
					t.Fatal("restoration contacted unavailable connection", e)
				}
			}
		})
	}
}

func TestRelevantVerificationChangeCreatesNoSnapshotAndPreservesPendingWork(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cache := filepath.Join(root, "cache")
	if err := os.Mkdir(cache, 0700); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", cache}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "--initial-branch=main")
	if err := os.MkdirAll(filepath.Join(cache, "internal/server"), 0700); err != nil {
		t.Fatal(err)
	}
	sourceFile := filepath.Join(cache, "internal/server/terminal_handlers.go")
	if err := os.WriteFile(sourceFile, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "Source")
	source := run("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(cache, "README.md"), []byte("Unrelated change"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "Unrelated")
	unrelated := run("rev-parse", "HEAD")
	if err := os.WriteFile(sourceFile, []byte("corrected"), 0600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "Correction")
	corrected := run("rev-parse", "HEAD")
	if err := os.WriteFile(sourceFile, []byte("PRESERVE_PENDING_WORK"), 0600); err != nil {
		t.Fatal(err)
	}
	before := run("status", "--porcelain=v1")
	store := NewMemoryStore()
	if err := store.CreateProject(ctx, &Project{ID: "project", Name: "Fixture", RepoURL: "https://github.com/fixture/repo", LocalPath: cache}); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(root, "projects")
	service := NewService(store, nil, "fixture", base)
	for _, example := range []struct {
		target  string
		changed bool
	}{{unrelated, false}, {corrected, true}} {
		changed, err := service.VerificationPathsChanged(ctx, "project", "task", source, example.target, []string{"internal/server/terminal_handlers.go"})
		if err != nil || changed != example.changed {
			t.Fatal("relevant change query failed", changed, err)
		}
	}
	if run("status", "--porcelain=v1") != before {
		t.Fatal("project change query modified saved work")
	}
	if _, err := os.Stat(filepath.Join(base, "verification-snapshots")); !os.IsNotExist(err) {
		t.Fatal("irrelevant deployment created a snapshot")
	}
}

func TestVerificationTargetFallbackToLocalWorkspaces(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	cache := filepath.Join(root, "data", "projects", "proj1")
	if err := os.MkdirAll(cache, 0700); err != nil {
		t.Fatal(err)
	}
	run := func(dir string, args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}
		return strings.TrimSpace(string(out))
	}
	run(cache, "init", "--initial-branch=main")
	if err := os.WriteFile(filepath.Join(cache, "file.txt"), []byte("c1"), 0600); err != nil {
		t.Fatal(err)
	}
	run(cache, "add", ".")
	run(cache, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "Commit 1")
	c1 := run(cache, "rev-parse", "HEAD")

	// Create a separate task workspace under root/data/workspaces/task_abc
	taskWorkspace := filepath.Join(root, "data", "workspaces", "task_abc")
	if err := os.MkdirAll(taskWorkspace, 0700); err != nil {
		t.Fatal(err)
	}
	run(taskWorkspace, "init", "--initial-branch=cuan/task-abc")
	run(taskWorkspace, "remote", "add", "origin", cache)
	run(taskWorkspace, "pull", "origin", "main")
	if err := os.WriteFile(filepath.Join(taskWorkspace, "file.txt"), []byte("c2-local-work"), 0600); err != nil {
		t.Fatal(err)
	}
	run(taskWorkspace, "add", ".")
	run(taskWorkspace, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "Commit 2 in local workspace")
	c2 := run(taskWorkspace, "rev-parse", "HEAD")

	// Verify c2 does NOT exist in cache
	cmd := exec.Command("git", "-C", cache, "cat-file", "-e", c2+"^{commit}")
	if cmd.Run() == nil {
		t.Fatal("c2 should not exist in cache initially")
	}

	store := NewMemoryStore()
	if err := store.CreateProject(ctx, &Project{
		ID:            "proj1",
		Name:          "Project 1",
		RepoURL:       "https://github.com/fixture/nonexistent-remote.git",
		LocalPath:     cache,
		DefaultBranch: "main",
	}); err != nil {
		t.Fatal(err)
	}

	baseDir := filepath.Join(root, "data", "projects")
	service := NewService(store, nil, "fixture-key", baseDir)

	// Call VerificationSnapshot requesting c1 and c2.
	// Remote fetch will fail (nonexistent remote), but it must find c2 in root/data/workspaces/task_abc!
	snapPath, err := service.VerificationSnapshot(ctx, "proj1", "task_abc", c1, c2)
	if err != nil {
		t.Fatalf("expected VerificationSnapshot to succeed via local workspace fallback, got error: %v", err)
	}
	if snapPath == "" {
		t.Fatal("expected non-empty snapshot path")
	}
	headCommit := run(snapPath, "rev-parse", "HEAD")
	if headCommit != c2 {
		t.Fatalf("expected snapshot HEAD to be c2 (%s), got: %s", c2, headCommit)
	}
}
