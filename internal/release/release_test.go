package release

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/buchin/cuan/internal/git"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func fixture(t *testing.T, body, manifest string) (string, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".cuan"), 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{ManifestPath: manifest, "release.sh": "#!/bin/sh\n" + body, "tracked.txt": "original\n"}
	for path, data := range files {
		if err := os.WriteFile(filepath.Join(root, path), []byte(data), 0700); err != nil {
			t.Fatal(err)
		}
	}
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatal("fixture git", string(out))
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "--initial-branch=dev")
	run("config", "user.name", "Fixture Human")
	run("config", "user.email", "human@example.invalid")
	run("add", ".")
	run("commit", "-m", "Release source")
	return root, run("rev-parse", "HEAD")
}

const goodManifest = `{"version":1,"command":["release.sh"],"timeout_seconds":5}`

func TestReleaseRunsExactSourceWithExplicitEnvironmentOnly(t *testing.T) {
	t.Setenv("RELEASE_AMBIENT_CREDENTIAL", "wrong-owner")
	root, commit := fixture(t, `printf '%s/%s' "$SELECTED" "${RELEASE_AMBIENT_CREDENTIAL:-not-inherited}" > "$MARKER"
printf 'fixture diagnostic\n'
`, goodManifest)
	plan, err := Load(context.Background(), root, commit, "dev")
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "delivered")
	result := Run(context.Background(), plan, []string{"PATH=" + os.Getenv("PATH"), "SELECTED=bound-owner", "MARKER=" + marker})
	if result.Error != "" || result.ExitCode != 0 || !result.SourceUnchanged || result.StartedAt.IsZero() || result.FinishedAt.Before(result.StartedAt) {
		t.Fatal("native execution failed", result)
	}
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "bound-owner/not-inherited" {
		t.Fatal("execution borrowed ambient environment", err)
	}
	raw, _ := json.Marshal(result)
	if strings.Contains(string(raw), "fixture diagnostic") || result.Diagnostic(nil) != "" || !strings.Contains(result.Diagnostic(func(s string) string { return s }), "fixture diagnostic") {
		t.Fatal("diagnostic boundary lost")
	}
	if Run(context.Background(), plan, nil).ExitCode != -1 {
		t.Fatal("nil environment inherited host credentials")
	}
}

func TestReleaseRejectsUnboundSourceAndDescriptor(t *testing.T) {
	for _, scenario := range []string{"wrong commit", "wrong branch", "dirty", "absolute command", "parent command", "unknown field", "unknown version", "timeout", "trailing json", "symlink"} {
		t.Run(scenario, func(t *testing.T) {
			manifest := goodManifest
			switch scenario {
			case "absolute command":
				manifest = `{"version":1,"command":["/bin/sh"],"timeout_seconds":5}`
			case "parent command":
				manifest = `{"version":1,"command":["../release.sh"],"timeout_seconds":5}`
			case "unknown field":
				manifest = `{"version":1,"command":["release.sh"],"timeout_seconds":5,"bypass":true}`
			case "unknown version":
				manifest = `{"version":2,"command":["release.sh"],"timeout_seconds":5}`
			case "timeout":
				manifest = `{"version":1,"command":["release.sh"],"timeout_seconds":999999}`
			case "trailing json":
				manifest += ` {}`
			}
			root, commit := fixture(t, "exit 0\n", manifest)
			branch := "dev"
			switch scenario {
			case "wrong commit":
				commit = strings.Repeat("a", 40)
			case "wrong branch":
				branch = "main"
			case "dirty":
				os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("retained change"), 0600)
			case "symlink":
				os.Remove(filepath.Join(root, "release.sh"))
				os.Symlink("tracked.txt", filepath.Join(root, "release.sh"))
				cmd := exec.Command("git", "-C", root, "commit", "-am", "Symlink source")
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatal(string(out))
				}
				out, _ := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
				commit = strings.TrimSpace(string(out))
			}
			if _, err := Load(context.Background(), root, commit, branch); err == nil {
				t.Fatal("unbound release accepted")
			}
		})
	}
}

func TestReleaseFailureAndSourceMutationNeverBecomeSuccess(t *testing.T) {
	for _, body := range []string{"exit 7\n", "printf changed > tracked.txt\n"} {
		root, commit := fixture(t, body, goodManifest)
		plan, err := Load(context.Background(), root, commit, "dev")
		if err != nil {
			t.Fatal(err)
		}
		result := Run(context.Background(), plan, []string{"PATH=" + os.Getenv("PATH")})
		if result.Error == "" {
			t.Fatal("failed release accepted", result)
		}
		if strings.Contains(body, "exit 7") && result.ExitCode != 7 {
			t.Fatal("real exit code lost")
		}
		if strings.Contains(body, "tracked.txt") && result.SourceUnchanged {
			t.Fatal("source mutation missed")
		}
	}
}

func TestReleaseRevalidatesPlanBeforeStarting(t *testing.T) {
	root, commit := fixture(t, `printf started > "$MARKER"`+"\n", goodManifest)
	plan, err := Load(context.Background(), root, commit, "dev")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("pending user work"), 0600)
	marker := filepath.Join(t.TempDir(), "must-not-exist")
	result := Run(context.Background(), plan, []string{"MARKER=" + marker})
	if result.Error == "" || result.ExitCode != -1 {
		t.Fatal("stale plan executed")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("release started after source changed")
	}
}

func TestReleaseCancellationAndChildOutputAreBounded(t *testing.T) {
	for _, orphan := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "orphan output"}[orphan], func(t *testing.T) {
			body := "sleep 30 &\nprintf '%s' $! > \"$CHILD_PID\"\nprintf started > \"$MARKER\"\nwait\n"
			if orphan {
				body = "sleep 30 &\nprintf '%s' $! > \"$CHILD_PID\"\nprintf started > \"$MARKER\"\nexit 0\n"
			}
			root, commit := fixture(t, body, goodManifest)
			plan, err := Load(context.Background(), root, commit, "dev")
			if err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(t.TempDir(), "started")
			pidFile := filepath.Join(t.TempDir(), "child-pid")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan Result, 1)
			go func() {
				done <- Run(ctx, plan, []string{"PATH=" + os.Getenv("PATH"), "MARKER=" + marker, "CHILD_PID=" + pidFile})
			}()
			if !orphan {
				deadline := time.Now().Add(2 * time.Second)
				for {
					if _, err := os.Stat(marker); err == nil {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("fixture never started")
					}
					time.Sleep(10 * time.Millisecond)
				}
				cancel()
			}
			select {
			case result := <-done:
				if result.Error == "" {
					t.Fatal("canceled/incomplete process accepted")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("release descendants outlived bounded execution")
			}
			raw, err := os.ReadFile(pidFile)
			if err != nil {
				t.Fatal("owned child receipt missing", err)
			}
			pid, err := strconv.Atoi(string(raw))
			if err != nil || pid <= 1 {
				t.Fatal("invalid owned child pid")
			}
			if syscall.Kill(pid, 0) == nil {
				state, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
				if err == nil {
					fields := strings.Fields(string(state))
					if len(fields) > 2 && fields[2] != "Z" {
						t.Fatal("owned release child still running")
					}
				}
			}
		})
	}
}

func TestReleaseRetainsBoundedDiagnosticTail(t *testing.T) {
	root, commit := fixture(t, "i=0; while [ $i -lt 5000 ]; do printf 'xxxxxxxxxxxxxxxx'; i=$((i+1)); done\n", goodManifest)
	plan, err := Load(context.Background(), root, commit, "dev")
	if err != nil {
		t.Fatal(err)
	}
	result := Run(context.Background(), plan, []string{"PATH=" + os.Getenv("PATH")})
	if result.Error != "" || len(result.Diagnostic(func(s string) string { return s })) != outputLimit {
		t.Fatal("diagnostic budget failed", result.Error)
	}
}

func TestPreparedReleaseWorkspaceLoadsWithoutExecutingOrChangingDeveloper(t *testing.T) {
	source, commit := fixture(t, "touch executed\n", goodManifest)
	destination := filepath.Join(t.TempDir(), "release")
	workspace, err := git.NewClient().PrepareReleaseWorkspace(context.Background(), source, destination, commit, commit, "production")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Load(context.Background(), workspace, commit, "production")
	if err != nil || plan.Description().Commit != commit || plan.Description().Branch != "production" || len(plan.Description().ManifestHash) != 64 {
		t.Fatal("prepared workspace incompatible with native release", err)
	}
	for _, root := range []string{source, workspace} {
		if _, err := os.Stat(filepath.Join(root, "executed")); !os.IsNotExist(err) {
			t.Fatal("workspace preparation executed release")
		}
	}
	out, err := exec.Command("git", "-C", source, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil || strings.TrimSpace(string(out)) != "dev" {
		t.Fatal("release checkout changed developer branch")
	}
}
