package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/executionpolicy"
	cuangit "github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
)

func TestSupplementalVerifierUsesPrivateExactCommitWorkspace(t *testing.T) {
	ctx := context.Background()
	source := documentationFixture(t)
	head, err := fingerprintGit(ctx, source, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	commit := strings.TrimSpace(string(head))
	original, err := os.ReadFile(filepath.Join(source, "scripts/test.sh"))
	if err != nil {
		t.Fatal(err)
	}
	// Developer has already started another local edit; verification must use
	// the accepted source commit without resetting or including that edit.
	os.WriteFile(filepath.Join(source, "scripts/test.sh"), []byte("pending developer work\n"), 0700)
	t.Setenv("HOME", t.TempDir())
	var completion CompleteAttemptRequest
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/report" {
			w.Write([]byte(`{"summary":"Source checked","complete_note":"Actual source inspected","activities":[{"attempt_id":"verifier","activity_type":"agent_report","new_status":"review"}]}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/complete") {
			if err := json.NewDecoder(r.Body).Decode(&completion); err != nil {
				t.Error(err)
			}
			json.NewEncoder(w).Encode(map[string]any{
				"id":        "verifier",
				"task_id":   filepath.Base(source),
				"runner_id": "fixture",
				"status":    task.AttemptStatusSucceeded,
			})
			return
		}
		w.Write([]byte(`{}`))
	}))
	defer api.Close()
	called := false
	adapter := &reportingBudgetAdapter{readOnlyAuditAdapter: &readOnlyAuditAdapter{t: t}, execute: func(_ context.Context, req agent.ExecutionRequest) (*agent.ExecutionResult, error) {
		called = true
		if !strings.Contains(req.Prompt, "Role: independent verifier.") || strings.Contains(req.Prompt, "Role: developer.") {
			t.Fatal("native verifier received developer role")
		}
		if req.WorkingDir == source {
			t.Fatal("verifier shares developer checkout")
		}
		data, err := os.ReadFile(filepath.Join(req.WorkingDir, "scripts/test.sh"))
		if err != nil || string(data) != string(original) {
			t.Fatal("verifier did not inspect exact committed content", err)
		}
		os.WriteFile(filepath.Join(source, "new-dev-file.txt"), []byte("keep developer work"), 0600)
		if _, err := os.Stat(filepath.Join(req.WorkingDir, "new-dev-file.txt")); !os.IsNotExist(err) {
			t.Fatal("developer mutation changed verifier snapshot")
		}
		return &agent.ExecutionResult{Status: "SUCCEEDED", ExitCode: 0}, nil
	}}
	worker := NewRunner(&config.Config{RunnerID: "fixture", RunnerWorkerRegistryDir: t.TempDir(), RunnerServerURL: api.URL, WorkspacesDir: filepath.Dir(source)}, adapter, api.Client())
	claim := ClaimTaskResponse{Task: &task.Task{ID: filepath.Base(source), Instructions: "Verify original code", SourceRoomID: "room", ExecutionCategory: task.ExecutionCategoryCode}, Attempt: &task.TaskAttempt{ID: "verifier", TaskID: filepath.Base(source), CreatedAt: time.Now()}, Verification: &task.VerificationIntent{IsolatedWorkspace: true, SourceCommit: commit}, PublicationLimits: executionpolicy.PublicationPolicy{ReadOnlyRepository: true, DenyPush: true, DenyDeploy: true}, TaskAgent: &TaskAgentCapability{API: api.URL + "/report", Token: "fixture"}}
	worker.runAttempt(ctx, claim)
	if !called || completion.Status != task.AttemptStatusSucceeded || completion.VerifiedSourceCommit != commit || !completion.RepositoryVerification.Unchanged() {
		t.Fatalf("verification failed or lost native identity: %+v called=%v", completion, called)
	}
	data, _ := os.ReadFile(filepath.Join(source, "scripts/test.sh"))
	if string(data) != "pending developer work\n" {
		t.Fatal("verification reset developer changes")
	}
	if _, err := os.Stat(filepath.Join(source, "new-dev-file.txt")); err != nil {
		t.Fatal("developer file lost", err)
	}
	private := cuangit.VerificationWorkspacePath(filepath.Dir(source), filepath.Base(source), commit)
	if err := cuangit.NewClient().VerifySnapshotCommit(ctx, private, commit, commit); err != nil {
		t.Fatal(err)
	}
}
