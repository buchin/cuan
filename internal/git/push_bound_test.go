package git

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPushBoundBranchRealGit(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	local := filepath.Join(root, "checkout")
	remote := filepath.Join(root, "remote.git")
	run := func(args ...string) string {
		t.Helper()
		out, e := exec.Command("git", args...).CombinedOutput()
		if e != nil {
			t.Fatal(string(out), e)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "--bare", remote)
	run("init", "--initial-branch=main", local)
	os.WriteFile(filepath.Join(local, "result"), []byte("one"), 0600)
	run("-C", local, "add", "result")
	run("-C", local, "commit", "-m", "Fixture commit")
	initial := run("-C", local, "rev-parse", "HEAD")
	branch := "cuan/task-" + strings.Repeat("a", 64)
	run("-C", local, "checkout", "-b", branch)
	actualGit, _ := exec.LookPath("git")
	quote := func(s string) string { raw, _ := json.Marshal(s); return string(raw) }
	wrapper := filepath.Join(root, "git-fixture")
	script := "#!/usr/bin/env python3\nimport os,sys\na=sys.argv[1:]\nassert not any('fixture-access-token' in x for x in a)\nif 'push' in a:\n assert os.environ.get('GIT_CONFIG_KEY_0')=='http.https://github.com/.extraheader'\n a=[" + quote(remote) + " if x=='https://github.com/owner/repo.git' else x for x in a]\nos.execv(" + quote(actualGit) + ",['git']+a)\n"
	os.WriteFile(wrapper, []byte(script), 0700)
	client := NewClient()
	client.gitBinary = wrapper
	first, e := client.PushBoundBranch(ctx, local, "owner/repo", branch, "fixture-access-token")
	if e != nil || first != initial {
		t.Fatal(first, e)
	}
	second, e := client.PushBoundBranch(ctx, local, "owner/repo", branch, "fixture-access-token")
	if e != nil || second != first {
		t.Fatal("repeat push changed result", e)
	}
	if got := run("--git-dir", remote, "rev-parse", "refs/heads/"+branch); got != initial {
		t.Fatal("wrong remote commit")
	}
	if _, e = client.PushBoundBranch(ctx, local, "owner/repo", "main", "fixture-access-token"); e == nil {
		t.Fatal("default branch push accepted")
	}
	config, _ := os.ReadFile(filepath.Join(local, ".git/config"))
	if strings.Contains(string(config), "fixture-access-token") {
		t.Fatal("token persisted")
	}
	os.WriteFile(filepath.Join(local, "result"), []byte("remote forward"), 0600)
	run("-C", local, "add", "result")
	run("-C", local, "commit", "-m", "Remote ahead")
	_, e = client.PushBoundBranch(ctx, local, "owner/repo", branch, "fixture-access-token")
	if e != nil {
		t.Fatal(e)
	}
	run("-C", local, "reset", "--hard", initial)
	os.WriteFile(filepath.Join(local, "divergent"), []byte("local"), 0600)
	run("-C", local, "add", "divergent")
	run("-C", local, "commit", "-m", "Divergent local")
	if _, e = client.PushBoundBranch(ctx, local, "owner/repo", branch, "fixture-access-token"); e == nil {
		t.Fatal("divergent push overwrote remote")
	}
}

func TestPushProjectBranchAllowDefault(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	local := filepath.Join(root, "checkout")
	remote := filepath.Join(root, "remote.git")
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Tester",
			"GIT_AUTHOR_EMAIL=tester@example.com",
			"GIT_COMMITTER_NAME=Tester",
			"GIT_COMMITTER_EMAIL=tester@example.com",
		)
		out, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatal(string(out), e)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "--bare", remote)
	run("init", "--initial-branch=dev", local)
	run("-C", local, "remote", "add", "origin", "https://github.com/owner/repo.git")
	if err := os.WriteFile(filepath.Join(local, "README.md"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	run("-C", local, "add", "README.md")
	run("-C", local, "commit", "-m", "initial dev commit")
	initial := run("-C", local, "rev-parse", "HEAD")

	actualGit, _ := exec.LookPath("git")
	quote := func(s string) string { raw, _ := json.Marshal(s); return string(raw) }
	wrapper := filepath.Join(root, "git-fixture")
	script := "#!/usr/bin/env python3\nimport os,sys\na=sys.argv[1:]\nassert not any('fixture-access-token' in x for x in a)\nif 'push' in a:\n assert os.environ.get('GIT_CONFIG_KEY_0')=='http.https://github.com/.extraheader'\n a=[" + quote(remote) + " if x=='https://github.com/owner/repo.git' else x for x in a]\nos.execv(" + quote(actualGit) + ",['git']+a)\n"
	if err := os.WriteFile(wrapper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}

	client := NewClient()
	client.gitBinary = wrapper

	if _, err := client.PushProjectBranch(ctx, local, "owner/repo", "dev", "dev", "fixture-access-token", false); err != ErrDefaultBranchProtected {
		t.Fatalf("expected ErrDefaultBranchProtected when allowDefault=false, got %v", err)
	}

	gotCommit, err := client.PushProjectBranch(ctx, local, "owner/repo", "dev", "dev", "fixture-access-token", true)
	if err != nil {
		t.Fatalf("expected allowDefault=true push to succeed, got %v", err)
	}
	if gotCommit != initial {
		t.Fatalf("got commit %q, want %q", gotCommit, initial)
	}
	if remoteCommit := run("--git-dir", remote, "rev-parse", "refs/heads/dev"); remoteCommit != initial {
		t.Fatalf("remote refs/heads/dev = %q, want %q", remoteCommit, initial)
	}
}
