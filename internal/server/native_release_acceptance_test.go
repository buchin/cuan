package server

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

// Protocol fixture for the final acceptance gates. Model report/process facts
// are synthetic; actual snapshot contents and native source ancestry are read.
func verifyNativeInspectionCompletionAndAcceptance(t *testing.T, s *Server, item *task.Task, releaseAttemptID string, launch *task.TaskLaunch, proof *task.LiveInspectionIntent) {
	t.Helper()
	ctx := context.Background()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	current, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	if current.LatestAttempt.ID == releaseAttemptID {
		_, err := s.taskSvc.ClaimLaunch(ctx, time.Now(), "fixture-verifier", launch.ID)
		must(err)
		_, err = s.taskSvc.CreateAttempt(ctx, item.ID, launch.OwnerID, task.CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: launch.AgentID, RequestID: launch.RequestID, ExpectedPredecessorID: releaseAttemptID})
		must(err)
	}
	_, err = s.runnerSvc.Heartbeat(ctx, runner.HeartbeatRequest{RunnerID: "native-verifier"}, time.Minute)
	must(err)
	current, err = s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	claimed, err := s.runnerSvc.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "native-verifier", SupportedAgents: []string{current.LatestAttempt.AgentName}, SupportsLiveInspection: true, SupportsNativeVisualVerification: true}, time.Minute)
	must(err)
	if !claimed.Claimed || claimed.LiveInspection == nil || claimed.LiveInspection.Purpose != task.NativeReleaseInspection {
		t.Fatal("delivered verifier was not claimed")
	}
	snapshot, err := s.projectSvc.VerificationSnapshot(ctx, item.ProjectID, item.ID, proof.SourceCommit, proof.TargetCommit)
	must(err)
	must(git.NewClient().VerifySnapshotCommit(ctx, snapshot, proof.SourceCommit, proof.TargetCommit))
	content, err := os.ReadFile(filepath.Join(snapshot, "work.txt"))
	must(err)
	if string(content) != "target" {
		t.Fatal("verifier did not inspect delivered content")
	}
	reported, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	if len(reported.DoDChecklist) != 1 || reported.DoDChecklist[0].ID != "delivered-content" || reported.DoDChecklist[0].Completed {
		t.Fatal("earlier stages waived original independent criterion")
	}
	checklist := append([]task.DoDItem(nil), reported.DoDChecklist...)
	checklist[0].Completed = true
	checklist[0].Evidence = "Read work.txt in exact delivered snapshot " + proof.TargetCommit + ": target"
	_, err = s.taskSvc.UpdateTask(ctx, item.ID, "fixture-verifier-report", task.UpdateTaskRequest{DoDChecklist: &checklist})
	must(err)
	stamp := time.Now().UTC()
	attempt := claimed.Attempt
	must(s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: "fixture-native-verifier-report", TaskID: item.ID, AttemptID: &attempt.ID, Author: "agent", ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Message: "Observed work.txt contains target in the exact delivered snapshot", CreatedAt: stamp}))
	fingerprint := strings.Repeat("b", 64)
	completed, err := s.runnerSvc.Complete(ctx, attempt.ID, runner.CompleteAttemptRequest{RunnerID: "native-verifier", Status: task.AttemptStatusSucceeded, ExitCode: 0, AgentReportRequired: true, AgentReportConfirmed: true, LiveInspectionCommit: proof.TargetCommit, SourceAncestryVerified: true, RepositoryVerification: &runner.RepositoryVerification{Baseline: fingerprint, Final: fingerprint}, TestSummary: "Project verification: test.sh --fast (exit 0)"})
	must(err)
	if completed.Status != task.AttemptStatusSucceeded {
		t.Fatal("native verifier completion rejected", completed.ErrorMessage)
	}
	verified, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	evidence := s.liveInspectionReviewContext(ctx, verified)
	if evidence == nil || evidence["verified_result_commit"] != proof.TargetCommit || s.independentCodeReviewMissing(ctx, verified) {
		t.Fatal("verified native target lost independent review evidence")
	}
	key := "task-review:" + attempt.ID
	message := &db.ProjectMessage{ID: delegationMessageID(key + ":input"), RoomID: item.SourceRoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: "Review [/tasks/" + item.ID + "]", CreatedAt: time.Now().UTC()}
	existing, err := s.projectStudioRepo.GetMessageByID(ctx, message.ID)
	if err != nil && !errors.Is(err, db.ErrMessageNotFound) {
		t.Fatal(err)
	}
	if existing == nil {
		createErr := s.projectStudioRepo.CreateMessage(ctx, message)
		if createErr != nil && !errors.Is(createErr, db.ErrDuplicateMessage) {
			t.Fatal(createErr)
		}
		existing, err = s.projectStudioRepo.GetMessageByID(ctx, message.ID)
		must(err)
	}
	if existing == nil || existing.RoomID != message.RoomID || existing.AuthorType != "system" || existing.AuthorID != "cuan_work_review" || !strings.Contains(existing.Content, "/tasks/"+item.ID) {
		t.Fatal("existing native review message has different provenance")
	}

	q := db.InboxRequest{OwnerID: launch.OwnerID, AgentID: launch.AgentID, RoomID: item.SourceRoomID, RequestID: delegationMessageID(key), MessageID: message.ID, ReplyID: delegationMessageID(key + ":reply"), CreatedAt: time.Now().UTC(), Deadline: time.Now().Add(time.Hour)}
	output := s.applyInboxReviewEvaluation(ctx, q, `{"decision":"pass","finding":"Delivered fixture content verified.","explanation":"Exact delivered content checked","remaining_work":"none","revision":""}`)
	accepted, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	if accepted.Status != task.TaskStatusDone {
		raw, _ := json.Marshal(s.inboxReviewPublication(ctx, accepted))
		t.Fatalf("native verifier pass did not reach Done: %s publication=%s", output, raw)
	}
	if len(accepted.DoDChecklist) != 1 || accepted.DoDChecklist[0].ID != checklist[0].ID || accepted.DoDChecklist[0].Text != checklist[0].Text || !accepted.DoDChecklist[0].Completed || accepted.DoDChecklist[0].Evidence != checklist[0].Evidence {
		t.Fatal("acceptance replaced original criteria or evidence")
	}
	if accepted.CommitSHA != proof.SourceCommit {
		t.Fatal("acceptance replaced original developer source")
	}
}
