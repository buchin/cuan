package server

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func TestTerminalVerificationRecoversStagedFilesWithoutInventingCompletion(t *testing.T) {
	for _, scenario := range []string{"saved", "restart", "private file", "symlink", "canceled", "newer attempt", "running snapshot"} {
		t.Run(scenario, func(t *testing.T) {
			s, d, q := stalledReviewFixture(t, "completed")
			stagedVerificationRecoveryContract(t, s, d, q, scenario)
		})
	}
}
func TestStagedVerificationRecoveryPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		s, q, d := phaseLineageSetup(t, database)
		stagedVerificationRecoveryContract(t, s, d, q, "restart")
	})
}
func stagedVerificationRecoveryContract(t *testing.T, s *Server, d *task.TaskDetailResponse, q db.InboxRequest, scenario string) {
	t.Helper()
	ctx := context.Background()
	verificationRuntimeRecoveryContract(t, s, d, q, "timeout")
	d, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	sourceAttempt := d.LatestAttempt.ID
	_, _, token, cleanup, err := s.issueTaskAgent(context.WithValue(ctx, taskAgentWorkspaceKey{}, t.TempDir()), d.ID, sourceAttempt, d.AssignedAgent)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	digest := sha256.Sum256([]byte(token))
	value, _ := s.taskAgentGrants.Load(digest)
	grant := value.(taskAgentGrant)
	directory := grant.EvidenceRoot.Name()
	file, err := os.Create(filepath.Join(directory, "mobile.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(file, image.NewRGBA(image.Rect(0, 0, 390, 844))); err != nil {
		t.Fatal(err)
	}
	file.Close()
	if err = os.WriteFile(filepath.Join(directory, "inspection.md"), []byte("Observations requiring independent review."), 0600); err != nil {
		t.Fatal(err)
	}
	switch scenario {
	case "restart":
		s.forgetTaskGrant(digest)
	case "private file":
		if err = os.WriteFile(filepath.Join(directory, ".env"), []byte("DO_NOT_DELIVER"), 0600); err != nil {
			t.Fatal(err)
		}
	case "symlink":
		if err = os.Symlink(filepath.Join(t.TempDir(), "private.txt"), filepath.Join(directory, "escape.txt")); err != nil {
			t.Fatal(err)
		}
	case "canceled":
		row, e := s.taskSvc.Store().GetTask(ctx, d.ID)
		if e != nil {
			t.Fatal(e)
		}
		row.Status = "cancelled"
		if e = s.taskSvc.Store().UpdateTask(ctx, row); e != nil {
			t.Fatal(e)
		}
	case "newer attempt":
		if _, err = s.taskSvc.CreateAttempt(ctx, d.ID, "owner", task.CreateAttemptRequest{RunnerID: "verifier", AgentName: d.AssignedAgent, RequestID: db.GenerateUUID()}); err != nil {
			t.Fatal(err)
		}
	case "running snapshot":
		copy := *d.LatestAttempt
		copy.Status = task.AttemptStatusRunning
		d.LatestAttempt = &copy
	}
	before := inboxReviewEvidenceDigest(d)
	s.recoverTerminalVerificationArtifacts(ctx, d)
	s.recoverTerminalVerificationArtifacts(ctx, d)
	message, err := s.projectStudioRepo.GetMessageByID(ctx, delegationMessageID("task-delivery:"+sourceAttempt))
	allowed := scenario == "saved" || scenario == "restart"
	if !allowed {
		if err == nil && message != nil {
			t.Fatal("delivered ineligible outputs")
		}
		return
	}
	if err != nil || message == nil {
		t.Fatal("lost staged outputs", err)
	}
	_, ids := decodeAttachmentMessage(message.Content)
	if len(ids) != 2 {
		t.Fatal("incomplete recovered delivery", ids)
	}
	after, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.LatestAttempt.Status != task.AttemptStatusFailed || after.Status == task.TaskStatusDone || len(after.Attempts) != 5 {
		t.Fatal("recovery invented execution or acceptance")
	}
	count := 0
	for _, a := range after.Activities {
		if a.AttemptID == nil || *a.AttemptID != sourceAttempt {
			continue
		}
		if a.ActivityType == recoveredVerificationArtifactsActivity {
			count++
			if a.NewStatus != "unreviewed" {
				t.Fatal("files treated as verified")
			}
		}
		if a.ActivityType == "agent_report" {
			t.Fatal("recovery fabricated agent report")
		}
	}
	data := s.inboxReviewData(ctx, after)
	if !strings.Contains(data, "Observations requiring independent review.") || !strings.Contains(data, `"staged_outputs_recovered":true`) {
		t.Fatal("review lost recovered report context")
	}
	if count != 1 || before == inboxReviewEvidenceDigest(after) {
		t.Fatal("missing durable evidence change", count)
	}
	if images := s.inboxReviewImages(ctx, after); len(images) != 1 || images[0].Width != 390 {
		t.Fatal("review omitted recovered image", images)
	}
}
