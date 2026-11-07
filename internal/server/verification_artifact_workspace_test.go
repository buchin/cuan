package server

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cuangit "github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestVerifierArtifactRootUsesExactSnapshotWithoutDeveloperFallback(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "completed")
	ctx := context.Background()
	s.cfg.WorkspacesDir = t.TempDir()
	source := filepath.Join(s.cfg.WorkspacesDir, d.ID)
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	initTestGitRepo(t, source)
	if err := os.WriteFile(filepath.Join(source, "result.txt"), []byte("committed result"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "result.txt"}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "-m", "Result"}} {
		if out, err := exec.Command("git", append([]string{"-C", source}, args...)...).CombinedOutput(); err != nil {
			t.Fatal(err, string(out))
		}
	}
	raw, err := exec.Command("git", "-C", source, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	commit := strings.TrimSpace(string(raw))
	verificationFollowupQueueContract(t, s, d, q, commit)
	store := s.taskSvc.Store()
	store.DeleteTask(ctx, "busy")
	job, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, task.VerificationRequestID(d.ID, commit)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "verifier", job.ID); err != nil {
		t.Fatal(err)
	}
	att, err := s.taskSvc.CreateAttempt(ctx, d.ID, "owner", task.CreateAttemptRequest{RunnerID: "verifier", AgentName: d.AssignedAgent, RequestID: job.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	s.runnerSvc = runner.NewService(runner.NewMemoryStore(store), store, nil)
	claim, err := s.runnerSvc.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "verifier"}, time.Minute)
	if err != nil || !claim.Claimed {
		t.Fatal("native claim failed", err)
	}
	private := cuangit.VerificationWorkspacePath(s.cfg.WorkspacesDir, d.ID, commit)
	if _, err := cuangit.NewClient().PrepareVerificationSnapshot(ctx, source, private, commit, commit); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(source, "result.txt"), []byte("new developer content"), 0600)
	grantRoot, err := os.OpenRoot(source)
	if err != nil {
		t.Fatal(err)
	}
	defer grantRoot.Close()
	detail, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	grant := taskAgentGrant{TaskID: d.ID, AttemptID: att.ID, ArtifactRoot: grantRoot}
	root, closeRoot, err := s.taskExecutionRoot(ctx, grant, detail)
	if err != nil {
		t.Fatal(err)
	}
	data, err := root.ReadFile("result.txt")
	closeRoot()
	if err != nil || string(data) != "committed result" {
		t.Fatal("artifact used developer checkout", err, string(data))
	}
	os.WriteFile(filepath.Join(private, "result.txt"), []byte("tampered"), 0600)
	if _, _, err := s.taskExecutionRoot(ctx, grant, detail); err == nil {
		t.Fatal("changed verifier snapshot fell back to developer output")
	}
}
