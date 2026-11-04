package server

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

func TestTaskEvidenceRetentionProtectsLiveAndUnmanagedDirectories(t *testing.T) {
	for _, scenario := range []string{"expired finished", "expired running", "fresh finished", "unmanaged directory"} {
		t.Run(scenario, func(t *testing.T) {
			s, _, call, _ := inboxFixture(t)
			ctx := context.Background()
			item := workspaceTask(t, call)
			attempt, err := s.taskSvc.CreateAttempt(ctx, item.ID, "owner", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: "agent_lifecycle"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning}); err != nil {
				t.Fatal(err)
			}
			workspace := t.TempDir()
			os.WriteFile(filepath.Join(workspace, "keep.txt"), []byte("REPOSITORY"), 0600)
			_, _, token, cleanup, err := s.issueTaskAgent(context.WithValue(ctx, taskAgentWorkspaceKey{}, workspace), item.ID, attempt.ID, "agent_lifecycle")
			if err != nil {
				t.Fatal(err)
			}
			defer cleanup()
			digest := sha256.Sum256([]byte(token))
			value, _ := s.taskAgentGrants.Load(digest)
			grant := value.(taskAgentGrant)
			evidenceDir := grant.EvidenceRoot.Name()
			if scenario == "unmanaged directory" {
				evidenceDir = filepath.Join(t.TempDir(), "artifacts")
				os.Mkdir(evidenceDir, 0700)
				grant.EvidenceRoot, err = os.OpenRoot(evidenceDir)
				if err != nil {
					t.Fatal(err)
				}
			}
			os.WriteFile(filepath.Join(evidenceDir, "mobile.png"), []byte("PRIVATE STAGED EVIDENCE"), 0600)
			if scenario != "fresh finished" {
				grant.Expires = time.Now().Add(-2 * time.Hour)
			}
			s.taskAgentGrants.Store(digest, grant)
			if err := os.Remove(taskGrantPath(digest)); err != nil {
				t.Fatal(err)
			}
			if err := s.persistTaskGrant(digest, grant, workspace); err != nil {
				t.Fatal(err)
			}
			if scenario != "expired running" {
				if _, err := s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed}); err != nil {
					t.Fatal(err)
				}
			}
			s.pruneExpiredTaskEvidence(ctx, time.Now())
			_, evidenceErr := os.Stat(evidenceDir)
			if scenario == "expired finished" {
				if !os.IsNotExist(evidenceErr) {
					t.Fatal("expired finished evidence retained", evidenceErr)
				}
				if _, err := os.Stat(taskGrantPath(digest)); !os.IsNotExist(err) {
					t.Fatal("expired grant index retained")
				}
			} else if evidenceErr != nil {
				t.Fatal("protected evidence removed", scenario, evidenceErr)
			}
			raw, err := os.ReadFile(filepath.Join(workspace, "keep.txt"))
			if err != nil || !strings.Contains(string(raw), "REPOSITORY") {
				t.Fatal("evidence retention touched repository")
			}
		})
	}
}
