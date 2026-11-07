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

func TestImmutableSourcePushUsesSavedAccessAndReconcilesRemote(t *testing.T) {
	for _, scenario := range []string{"new", "already pushed", "head moved during observation", "denied observation", "dirty", "wrong source", "wrong repo", "missing access", "protected branch"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			local, remote := filepath.Join(root, "local"), filepath.Join(root, "remote.git")
			realGit, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			run := func(args ...string) string {
				t.Helper()
				out, err := exec.Command(realGit, args...).CombinedOutput()
				if err != nil {
					t.Fatalf("fixture git: %s", out)
				}
				return strings.TrimSpace(string(out))
			}
			run("init", "--bare", remote)
			run("init", "--initial-branch=task/source", local)
			if err := os.WriteFile(filepath.Join(local, "result.txt"), []byte("approved"), 0600); err != nil {
				t.Fatal(err)
			}
			run("-C", local, "add", ".")
			run("-C", local, "-c", "user.name=Fixture Human", "-c", "user.email=human@example.invalid", "commit", "-m", "Source")
			source := run("-C", local, "rev-parse", "HEAD")
			run("-C", local, "-c", "user.name=Fixture Human", "-c", "user.email=human@example.invalid", "commit", "--allow-empty", "-m", "Later work", "--no-gpg-sign")
			later := run("-C", local, "rev-parse", "HEAD")
			run("-C", local, "reset", "--soft", source)
			run("-C", local, "remote", "add", "origin", "https://github.com/owner/repo.git")
			if scenario == "already pushed" {
				run("-C", local, "push", remote, source+":refs/heads/task/source")
			}
			calls := filepath.Join(root, "calls")
			wrapper := filepath.Join(root, "git-wrapper")
			quote := func(value string) string { raw, _ := json.Marshal(value); return string(raw) }
			script := "#!/usr/bin/python3\nimport sys,os,base64,subprocess\na=sys.argv[1:]\nif 'ls-remote' in a or 'push' in a:\n assert not os.environ.get('GH_TOKEN') and not os.environ.get('GITHUB_TOKEN') and not os.environ.get('SSH_AUTH_SOCK')\n assert base64.b64decode(os.environ['GIT_CONFIG_VALUE_0'].split('Basic ')[1]).decode()=='x-access-token:saved-owner'\n with open(" + quote(calls) + ",'a') as f:f.write(('observe' if 'ls-remote' in a else 'push')+'\\n')\n"
			if scenario == "denied observation" {
				script += " if 'ls-remote' in a:sys.exit(128)\n"
			}
			if scenario == "head moved during observation" {
				script += " if 'ls-remote' in a:subprocess.run([" + quote(realGit) + ",'-C'," + quote(local) + ",'update-ref','refs/heads/task/source'," + quote(later) + "],check=True)\n"
			}
			script += " a=[" + quote(remote) + " if x=='https://github.com/owner/repo.git' else x for x in a]\nos.execv(" + quote(realGit) + ",['git']+a)\n"
			if err := os.WriteFile(wrapper, []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			client := NewClient()
			client.gitBinary = wrapper
			branch, repoURL, expected, token := "task/source", "https://github.com/owner/repo.git", source, "saved-owner"
			switch scenario {
			case "dirty":
				if err := os.WriteFile(filepath.Join(local, "pending"), []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			case "wrong source":
				expected = later
			case "wrong repo":
				repoURL = "https://github.com/foreign/repo.git"
			case "missing access":
				token = ""
			case "protected branch":
				branch = "dev"
			}
			t.Setenv("GH_TOKEN", "wrong-host-token")
			t.Setenv("GITHUB_TOKEN", "other-host-token")
			t.Setenv("SSH_AUTH_SOCK", "wrong-host-agent")
			result, err := client.PushExecutionSource(context.Background(), local, repoURL, branch, "dev", expected, token)
			valid := scenario == "new" || scenario == "already pushed" || scenario == "head moved during observation"
			data, _ := os.ReadFile(calls)
			if valid {
				wantCalls := "observe\npush\n"
				if scenario == "already pushed" {
					wantCalls = "observe\n"
				}
				if err != nil || result != source || string(data) != wantCalls || run("--git-dir", remote, "rev-parse", "refs/heads/task/source") != source {
					t.Fatal("immutable remote source not confirmed", err, string(data))
				}
				if scenario == "head moved during observation" && run("-C", local, "rev-parse", "HEAD") != later {
					t.Fatal("source branch movement was reset")
				}
			} else {
				if err == nil || result != "" || strings.Contains(string(data), "push") {
					t.Fatal("invalid publication mutated remote", err)
				}
				if scenario != "denied observation" && len(data) != 0 {
					t.Fatal("invalid local scope accessed remote")
				}
			}
		})
	}
}
