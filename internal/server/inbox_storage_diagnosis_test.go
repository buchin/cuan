package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

func storageDiagnosisFixture(t *testing.T) (*Server, *task.TaskDetailResponse, db.InboxRequest, string) {
	t.Helper()
	s, d, _ := stalledReviewFixture(t, "completed", task.AttemptStatusFailed)
	ctx := context.Background()
	requests, _ := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, d.SourceRoomID)
	var origin db.InboxRequest
	for _, q := range requests {
		if q.MessageID == d.SourceMessageID {
			origin = q
		}
	}
	d.Status = task.TaskStatusBlocked
	d.CommitSHA = strings.Repeat("a", 40)
	started := time.Now().Add(-time.Hour).Truncate(time.Second)
	finished := started.Add(2 * time.Minute)
	exit := 2
	d.LatestAttempt.StartedAt = &started
	d.LatestAttempt.FinishedAt = &finished
	d.LatestAttempt.ExitCode = &exit
	d.Attempts = []*task.TaskAttempt{{ID: "older1", Status: task.AttemptStatusSucceeded}, {ID: "older2", Status: task.AttemptStatusFailed}, d.LatestAttempt}
	account, err := agent.CreateProviderAccount(origin.OwnerID, "Fixture")
	if err != nil {
		t.Fatal(err)
	}
	home, err := agent.ProviderAccountHome(origin.OwnerID, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	s.cfg.WorkspacesDir = t.TempDir()
	workspace := filepath.Join(s.cfg.WorkspacesDir, d.ID)
	directory := filepath.Join(home, ".gemini", "antigravity-cli", "log")
	os.MkdirAll(directory, 0700)
	path := filepath.Join(directory, started.In(time.Local).Format("cli-20060102_150405.log"))
	raw := "I1104 18:47:50.465845 1 server.go:313] Creating CLI server backend: product=antigravity workspaceDirs=[" + workspace + " " + workspace + "]\nE1104 18:49:38.610926 736 task_handle.go:150] failed mkdir " + home + "/.gemini/antigravity-cli/brain/task: no space left on device\n"
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(path, finished, finished)
	return s, d, origin, path
}

func TestStorageDiagnosisCorrectsBudgetWithoutRewritingFailedAttempt(t *testing.T) {
	s, d, origin, _ := storageDiagnosisFixture(t)
	ctx := context.Background()
	if taskRevisionAttemptCount(d) != 3 || interruptedInboxTaskCanResume(d) {
		t.Fatal("fixture must begin exhausted")
	}
	s.diagnoseInboxStorageFailure(ctx, d, origin)
	if !diagnosedStorageInterruption(d, d.LatestAttempt) || taskRevisionAttemptCount(d) != 2 || !interruptedInboxTaskCanResume(d) {
		t.Fatal("proven infrastructure failure not reconciled")
	}
	if d.LatestAttempt.Status != task.AttemptStatusFailed || *d.LatestAttempt.ExitCode != 2 || d.CommitSHA != strings.Repeat("a", 40) {
		t.Fatal("diagnosis rewrote history or saved commit")
	}
	if !strings.Contains(taskRecoveryContext(d), "Platform diagnosis:") {
		t.Fatal("continuation lost diagnosis")
	}
	count := len(d.Activities)
	s.diagnoseInboxStorageFailure(ctx, d, origin)
	if len(d.Activities) != count {
		t.Fatal("diagnostic scan duplicated evidence")
	}
	// The receipt is tied to scope, task, attempt, timing and the platform writer.
	original := d.Instructions
	d.Instructions += "new scope"
	if diagnosedStorageInterruption(d, d.LatestAttempt) {
		t.Fatal("diagnosis survived changed scope")
	}
	d.Instructions = original
	for _, a := range d.Activities {
		if a.ActivityType == storageDiagnosisActivity {
			a.Author = "agent"
		}
	}
	if diagnosedStorageInterruption(d, d.LatestAttempt) {
		t.Fatal("agent report became platform diagnosis")
	}
}

func TestStorageDiagnosisRequiresAuthorizationAndMatchingLocalEvidence(t *testing.T) {
	for _, scenario := range []string{"cancelled", "missing approval", "foreign owner", "no log", "tampered receipt"} {
		t.Run(scenario, func(t *testing.T) {
			s, d, origin, path := storageDiagnosisFixture(t)
			switch scenario {
			case "cancelled":
				d.LatestAttempt.CancelRequested = true
			case "missing approval":
				origin.MessageID = "missing"
			case "foreign owner":
				origin.OwnerID = "other"
			case "no log":
				os.Remove(path)
			case "tampered receipt":
				raw, _ := json.Marshal(storageDiagnosis{Kind: "storage_exhausted", InstructionsHash: "wrong", EvidenceHash: strings.Repeat("a", 64)})
				d.Activities = append(d.Activities, &task.TaskActivity{TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: storageDiagnosisActivity, Author: storageDiagnosisAuthor, Message: string(raw)})
				os.Remove(path)
			}
			s.diagnoseInboxStorageFailure(context.Background(), d, origin)
			if diagnosedStorageInterruption(d, d.LatestAttempt) || interruptedInboxTaskCanResume(d) {
				t.Fatal("unproven diagnosis bypassed budget/approval")
			}
		})
	}
}

func TestStorageDiagnosisRecoveryQueuesOnceBehindExistingProjectWork(t *testing.T) {
	s, d, origin, _ := storageDiagnosisFixture(t)
	ctx := context.Background()
	if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().UpdateAttempt(ctx, d.LatestAttempt); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "busy-project-task", ProjectID: d.ProjectID, Status: task.TaskStatusDoing}); err != nil {
		t.Fatal(err)
	}
	s.diagnoseInboxStorageFailure(ctx, d, origin)
	if !s.recoverInterruptedInboxTask(ctx, d, origin) {
		t.Fatal("proven storage interruption did not save a normal queued continuation")
	}
	id := task.LaunchID(d.ID, delegationMessageID("task-storage-recovery:"+d.LatestAttempt.ID))
	launch, err := s.taskSvc.GetLaunch(ctx, id)
	if err != nil || launch.State != "queued" || launch.SourceRequestID != origin.RequestID || launch.OwnerID != origin.OwnerID || launch.InstructionsHash != task.LaunchInstructionsHash(d.Instructions) {
		t.Fatalf("wrong durable continuation: %+v %v", launch, err)
	}
	if !strings.Contains(launch.Revision, "Reconcile uncertain external effects") || !strings.Contains(launch.Revision, "storage was exhausted") {
		t.Fatal("recovery lost storage diagnosis or effect reconciliation")
	}
	s.recoverInterruptedInboxTask(ctx, d, origin)
	after, err := s.taskSvc.GetLaunch(ctx, id)
	if err != nil || after.ID != launch.ID {
		t.Fatal("repeated scan created another continuation")
	}
	attempts, err := s.taskSvc.Store().ListAttempts(ctx, d.ID)
	if err != nil || len(attempts) != 1 {
		t.Fatal("diagnosis bypassed occupied project lane and reserved an attempt")
	}
}
