package project

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecutionWorkspaceFetchesMissingBaseWithoutChangingFiles(t *testing.T) {
	root := t.TempDir()
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		out, e := exec.Command(realGit, args...).CombinedOutput()
		if e != nil {
			t.Fatal(string(out), e)
		}
		return strings.TrimSpace(string(out))
	}
	remote := filepath.Join(root, "remote.git")
	seed := filepath.Join(root, "seed")
	source := filepath.Join(root, "source")
	run("init", "--bare", remote)
	run("init", "--initial-branch=main", seed)
	os.WriteFile(filepath.Join(seed, "README"), []byte("approved repository"), 0600)
	run("-C", seed, "add", "README")
	run("-C", seed, "commit", "-m", "Fixture base")
	run("-C", seed, "push", remote, "main")
	run("init", "--initial-branch=main", source)
	os.WriteFile(filepath.Join(source, "pending.txt"), []byte("retain untracked work"), 0600)
	bin := filepath.Join(root, "bin")
	os.Mkdir(bin, 0700)
	count := filepath.Join(root, "fetches")
	quote := func(v string) string { r, _ := json.Marshal(v); return string(r) }
	script := "#!/usr/bin/python3\nimport os,sys\na=sys.argv[1:]\nif 'fetch' in a:\n assert not any('fixture-token' in x for x in a)\n assert os.environ.get('GIT_CONFIG_KEY_0')=='http.https://github.com/.extraheader'\n with open(" + quote(count) + ",'a') as f:f.write('fetch\\n')\n a=[" + quote(remote) + " if x=='https://github.com/fixture/repo.git' else x for x in a]\nos.execv(" + quote(realGit) + ",['git']+a)\n"
	os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	store := NewMemoryStore()
	p := &Project{ID: "fixture", Name: "Fixture", RepoURL: "https://github.com/fixture/repo", LocalPath: source, DefaultBranch: "main"}
	store.CreateProject(context.Background(), p)
	svc := NewService(store, nil, "fixture-key", filepath.Join(root, "workspaces"))
	svc.SetSharedRepositoryAccess(func(context.Context) (string, error) { return "fixture-token", nil })
	response := p.ToResponse()
	first, err := svc.ExecutionWorkspace(context.Background(), "task-real", response)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(first, "README"))
	if string(data) != "approved repository" {
		t.Fatal("wrong repository base")
	}
	data, _ = os.ReadFile(filepath.Join(source, "pending.txt"))
	if string(data) != "retain untracked work" {
		t.Fatal("existing edits changed")
	}
	os.WriteFile(filepath.Join(first, "README"), []byte("retained task edits"), 0600)
	second, err := svc.ExecutionWorkspace(context.Background(), "task-real", response)
	if err != nil || first != second {
		t.Fatal("repeat duplicated worktree", err)
	}
	data, _ = os.ReadFile(count)
	if string(data) != "fetch\n" {
		t.Fatal("resumed work fetched or recreated", string(data))
	}
	response.DefaultBranch = "missing"
	if _, err = svc.ExecutionWorkspace(context.Background(), "task-missing", response); err == nil || !strings.Contains(err.Error(), "selected version no longer exists") {
		t.Fatal("missing branch misreported", err)
	}
	if got := run("-C", source, "symbolic-ref", "--short", "HEAD"); got != "main" {
		t.Fatal("source branch changed", got)
	}
}
