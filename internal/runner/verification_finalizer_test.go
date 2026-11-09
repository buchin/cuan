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

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
)

type forbiddenFinalizationAdapter struct{ t *testing.T }

func (a forbiddenFinalizationAdapter) Name() string             { return "fixture" }
func (a forbiddenFinalizationAdapter) Version() (string, error) { return "fixture", nil }
func (a forbiddenFinalizationAdapter) Execute(context.Context, agent.ExecutionRequest) (*agent.ExecutionResult, error) {
	a.t.Fatal("finalization invoked AI adapter")
	return nil, nil
}

func TestVerificationFinalizerChecksActualGitWithoutAIOrWorkspaceRepair(t *testing.T) {
	for _, scenario := range []string{"valid", "dirty", "different fingerprint", "different commit"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			root := privateCompletionFixture(t)
			head, err := fingerprintGit(ctx, root, "rev-parse", "HEAD")
			if err != nil {
				t.Fatal(err)
			}
			baseline, err := repositoryFingerprint(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			proof := &task.VerificationIntent{FinalizeAttemptID: "origin", SourceCommit: strings.TrimSpace(string(head)), RepositoryBaseline: baseline, ArtifactManifestHash: strings.Repeat("a", 64)}
			switch scenario {
			case "dirty":
				if err = os.WriteFile(filepath.Join(root, "saved.txt"), []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			case "different fingerprint":
				proof.RepositoryBaseline = strings.Repeat("b", 64)
			case "different commit":
				proof.SourceCommit = strings.Repeat("0", 40)
			}
			var completion CompleteAttemptRequest
			completed := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/complete") {
					if err := json.NewDecoder(r.Body).Decode(&completion); err != nil {
						t.Error(err)
					}
					completed++
					json.NewEncoder(w).Encode(task.TaskAttempt{ID: "finalization", TaskID: filepath.Base(root), RunnerID: "fixture", Status: completion.Status})
					return
				}
				w.Write([]byte(`{}`))
			}))
			defer api.Close()
			worker := NewRunner(&config.Config{RunnerID: "fixture", RunnerServerURL: api.URL, WorkspacesDir: filepath.Dir(root)}, forbiddenFinalizationAdapter{t})
			worker.runAttempt(ctx, ClaimTaskResponse{Task: &task.Task{ID: filepath.Base(root)}, Attempt: &task.TaskAttempt{ID: "finalization", TaskID: filepath.Base(root)}, Verification: proof})
			want := task.AttemptStatusFailed
			if scenario == "valid" {
				want = task.AttemptStatusSucceeded
			}
			if completed != 1 || completion.Status != want || completion.AgentReportConfirmed || completion.AgentReportRequired || completion.CommitSHA != "" || completion.PRURL != "" {
				t.Fatalf("invalid finalization completion %+v", completion)
			}
			if len(worker.pendingCompletionIDs()) != 0 {
				t.Fatal("fixture completion was not acknowledged")
			}
			if scenario == "valid" && (!completion.RepositoryVerification.Unchanged() || completion.FinalizationManifestHash != proof.ArtifactManifestHash || completion.VerifiedSourceCommit != proof.SourceCommit) {
				t.Fatal("missing independent source evidence")
			}
			if scenario == "dirty" {
				if data, err := os.ReadFile(filepath.Join(root, "saved.txt")); err != nil || string(data) != "keep" {
					t.Fatal("repaired saved work")
				}
			}
		})
	}
}
