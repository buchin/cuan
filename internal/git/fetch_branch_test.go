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

func TestCloneOrFetchRepairsMissingOriginAndRetainsFiles(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	seed := filepath.Join(root, "seed")
	cache := filepath.Join(root, "cache")
	actualGit, _ := exec.LookPath("git")
	run := func(args ...string) {
		t.Helper()
		if out, e := exec.Command(actualGit, args...).CombinedOutput(); e != nil {
			t.Fatal(string(out), e)
		}
	}
	run("init", "--bare", remote)
	run("init", "--initial-branch=main", seed)
	os.WriteFile(filepath.Join(seed, "README"), []byte("repository"), 0600)
	run("-C", seed, "add", "README")
	run("-C", seed, "commit", "-m", "Fixture")
	run("-C", seed, "push", remote, "main")
	run("init", cache)
	os.WriteFile(filepath.Join(cache, "pending.txt"), []byte("do not discard"), 0600)
	quote := func(s string) string { v, _ := json.Marshal(s); return string(v) }
	wrapper := filepath.Join(root, "git-wrapper")
	script := "#!/usr/bin/python3\nimport os,sys\na=[" + quote(remote) + " if x=='https://github.com/fixture/repo.git' else x for x in sys.argv[1:]]\nos.execv(" + quote(actualGit) + ",['git']+a)\n"
	os.WriteFile(wrapper, []byte(script), 0700)
	client := NewClient()
	client.gitBinary = wrapper
	if err := client.CloneOrFetch(context.Background(), "fixture", "repo", "", cache); err != nil {
		t.Fatal(err)
	}
	if err := client.CloneOrFetch(context.Background(), "fixture", "repo", "", cache); err != nil {
		t.Fatal("repeat fetch failed", err)
	}
	data, _ := os.ReadFile(filepath.Join(cache, "pending.txt"))
	if string(data) != "do not discard" {
		t.Fatal("pending edits lost")
	}
	run("-C", cache, "rev-parse", "--verify", "refs/remotes/origin/main^{commit}")
	config, _ := os.ReadFile(filepath.Join(cache, ".git/config"))
	if strings.Contains(string(config), "x-access-token") {
		t.Fatal("credentials persisted")
	}
}
