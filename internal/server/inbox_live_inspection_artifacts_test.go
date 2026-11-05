package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestLiveInspectionRecoversStagedReportWithoutCompletion(t *testing.T) {
	for _, scenario := range []string{"saved", "restart", "cancelled", "changed scope", "symlink", "private file"} {
		t.Run(scenario, func(t *testing.T) {
			s, q, d := phaseLineageSetup(t)
			liveInspectionStagedReportContract(t, s, q, d, scenario, nil)
		})
	}
}
func TestLiveInspectionStagedReportPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		s, q, d := phaseLineageSetup(t, database)
		liveInspectionStagedReportContract(t, s, q, d, "restart", database)
	})
}

func liveInspectionStagedReportContract(t *testing.T, s *Server, q db.InboxRequest, d *task.TaskDetailResponse, scenario string, database *sql.DB) {
	t.Helper()
	ctx := context.Background()
	liveInspectionContract(t, s, q, d, "current outcome timeout files", database)
	d, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	attempt := d.LatestAttempt.ID
	_, _, token, cleanup, err := s.issueTaskAgent(context.WithValue(ctx, taskAgentWorkspaceKey{}, t.TempDir()), d.ID, attempt, d.AssignedAgent)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	digest := sha256.Sum256([]byte(token))
	raw, _ := s.taskAgentGrants.Load(digest)
	grant := raw.(taskAgentGrant)
	directory := grant.EvidenceRoot.Name()
	if err = os.WriteFile(filepath.Join(directory, "investigation_report.md"), []byte("Retained factual findings; execution timed out before final verification."), 0600); err != nil {
		t.Fatal(err)
	}
	switch scenario {
	case "restart":
		s.forgetTaskGrant(digest)
	case "cancelled":
		row, _ := s.taskSvc.Store().GetTask(ctx, d.ID)
		row.Status = "cancelled"
		if err = s.taskSvc.Store().UpdateTask(ctx, row); err != nil {
			t.Fatal(err)
		}
	case "changed scope":
		row, _ := s.taskSvc.Store().GetTask(ctx, d.ID)
		row.Instructions += "\nDifferent scope"
		if err = s.taskSvc.Store().UpdateTask(ctx, row); err != nil {
			t.Fatal(err)
		}
	case "symlink":
		if err = os.Symlink(filepath.Join(t.TempDir(), "private"), filepath.Join(directory, "escape.md")); err != nil {
			t.Fatal(err)
		}
	case "private file":
		if err = os.WriteFile(filepath.Join(directory, ".env"), []byte("DO_NOT_DELIVER"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	before := inboxReviewEvidenceDigest(d)
	attemptCount := len(d.Attempts)
	s.recoverTerminalVerificationArtifacts(ctx, d)
	s.recoverTerminalVerificationArtifacts(ctx, d)
	message, err := s.projectStudioRepo.GetMessageByID(ctx, delegationMessageID("task-delivery:"+attempt))
	allowed := scenario == "saved" || scenario == "restart"
	if !allowed {
		if err == nil && message != nil {
			t.Fatal("ineligible inspection files delivered")
		}
		return
	}
	if err != nil || message == nil {
		t.Fatal("saved inspection report lost", err)
	}
	_, ids := decodeAttachmentMessage(message.Content)
	if len(ids) != 1 {
		t.Fatal("report delivered more than once", ids)
	}
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status == task.TaskStatusDone || after.LatestAttempt.Status != task.AttemptStatusFailed || len(after.Attempts) != attemptCount {
		t.Fatal("artifact recovery changed outcome or created another execution")
	}
	count := 0
	for _, event := range after.Activities {
		if event.AttemptID == nil || *event.AttemptID != attempt {
			continue
		}
		if event.ActivityType == "agent_report" || event.ActivityType == task.ActivityLiveInspection {
			t.Fatal("recovery fabricated final report or live verification")
		}
		if event.ActivityType == recoveredVerificationArtifactsActivity {
			count++
			if event.NewStatus != "unreviewed" {
				t.Fatal("report treated as verified")
			}
		}
	}
	if count != 1 || inboxReviewEvidenceDigest(after) == before {
		t.Fatal("missing idempotent evidence revision", count)
	}
	data := s.inboxReviewData(ctx, after)
	if !strings.Contains(data, "Retained factual findings") || !strings.Contains(data, `"staged_outputs_recovered":true`) {
		t.Fatal("review cannot inspect saved report")
	}
	if s.liveInspectionReviewContext(ctx, after) != nil {
		t.Fatal("saved file became a successful inspection receipt")
	}
}
