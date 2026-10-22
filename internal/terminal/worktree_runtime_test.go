package terminal

import (
	"context"
	"github.com/buchin/cuan/internal/project"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorktreeTaskRuntimeCommitsAndResumes(t *testing.T) {
	ctx := context.Background()
	source := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, e := exec.Command("git", append([]string{"-C", source}, args...)...).CombinedOutput()
		if e != nil {
			t.Fatal(string(out), e)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "--initial-branch=main")
	os.WriteFile(filepath.Join(source, "README"), []byte("initial"), 0600)
	run("add", "README")
	run("commit", "-m", "Fixture initial")
	run("update-ref", "refs/remotes/origin/main", "HEAD")
	service := project.NewService(project.NewMemoryStore(), nil, "fixture-secret", t.TempDir())
	p := &project.ProjectResponse{RepoURL: "https://github.com/fixture/repo", LocalPath: source, DefaultBranch: "main"}
	workspace, e := service.ExecutionWorkspace(ctx, "runtime-task", p)
	if e != nil {
		t.Fatal(e)
	}
	bin := t.TempDir()
	// The fixture replaces model inference only. PTY, supervisor, worktree,
	// shell commands, commits, process exit and manager reconstruction are real.
	script := `#!/bin/sh
set -eu
git rev-parse --is-inside-work-tree >/dev/null
if [ ! -f result.txt ]; then
 printf 'one implementation\n' > result.txt
 git add -- result.txt
 git commit -m 'Implement fixture task'
else
 test "$(cat result.txt)" = 'one implementation'
fi
printf 'WORKTREE_RUNTIME_OK\n'
`
	os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	base := t.TempDir()
	execute := func() {
		t.Helper()
		manager := NewManager(ManagerConfig{WorkspacesDir: base})
		defer manager.CloseAll()
		finished := make(chan *TaskRunHistory, 1)
		_, err := manager.RunTaskAgent(ctx, TaskAgentRunConfig{TaskID: "runtime-task", Command: "agy", WorkDir: workspace, OnExit: func(h *TaskRunHistory) { finished <- h }})
		if err != nil {
			t.Fatal(err)
		}
		select {
		case result := <-finished:
			if result.ExitCode != 0 {
				t.Fatalf("runtime exit %d", result.ExitCode)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("runtime timeout")
		}
	}
	execute()
	execute()
	output, e := exec.Command("git", "-C", workspace, "log", "--format=%s").Output()
	if e != nil {
		t.Fatal(e)
	}
	if strings.Count(string(output), "Implement fixture task") != 1 {
		t.Fatal("resumed execution duplicated commit")
	}
	if _, e = os.Stat(filepath.Join(source, "result.txt")); !os.IsNotExist(e) {
		t.Fatal("task writes leaked into project checkout")
	}
}
