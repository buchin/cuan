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
func verifyNativeInspectionCompletionAndAcceptance(t *testing.T, s *Server, item *task.Task, releaseAttemptID string, launch *task.TaskLaunch, proof *task.LiveInspectionIntent, evidenceGap ...bool) {
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
	if len(evidenceGap) > 0 && evidenceGap[0] {
		assessment := inboxReviewEvaluation{Decision: "revise", RemainingWork: "verification", Finding: "Actual mobile and desktop component evidence is missing", Explanation: "Complete original visual evidence", Revision: "Inspect the actual requested component at mobile and desktop viewports; attach the real screenshots.", ManagerAction: "continue", ManagerReason: "The native regression captures do not cover the original component"}
		_, err = s.saveReviewAssessment(ctx, verified, q, assessment)
		must(err)
		blocked := task.TaskStatusBlocked
		_, err = s.taskSvc.UpdateTask(ctx, item.ID, "coordinator", task.UpdateTaskRequest{Status: &blocked})
		must(err)
		blocker := &task.Task{ID: "evidence-lane-busy", ProjectID: item.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}
		must(s.taskSvc.Store().CreateTask(ctx, blocker))
		verified, err = s.taskSvc.GetTaskDetail(ctx, item.ID)
		must(err)
		before := len(verified.Attempts)
		for i := 0; i < 3; i++ {
			s.queueInboxTaskReview(ctx, verified)
		}
		continued := *proof
		continued.PreviousAttemptID, continued.PreviousCommit = attempt.ID, proof.TargetCommit
		continued.ReviewRequestID, continued.RevalidationKind = q.RequestID, task.EvidenceRevalidation
		continued.ObservedAt = time.Now().UTC()
		if err := task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &verified.Task, &continued); err != nil {
			t.Fatalf("evidence continuation source invalid: %v", err)
		}
		job, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(item.ID, continued.RequestID(item.ID)))
		if err != nil {
			request, revision := task.OutcomeEvidenceReview(&verified.Task, verified.LatestAttempt, verified.Activities, "")
			t.Fatalf("continuation not saved: %v review=%q revision=%q scope=%v observed=%+v", err, request, revision, s.liveInspectionReadScopeAllowed(ctx, verified, proof.SourceAttemptID), s.runningProjectRelease(ctx, verified))
		}
		if job == nil || job.State != "queued" {
			t.Fatalf("missing bounded evidence continuation: %+v", job)
		}
		parsed, err := task.LiveInspectionForLaunch(&verified.Task, job)
		must(err)
		if parsed == nil || parsed.Purpose != task.NativeReleaseInspection || parsed.PredecessorAttemptID() != attempt.ID || parsed.SourceAttemptID != proof.SourceAttemptID {
			t.Fatal("lost native lineage", parsed)
		}
		must(task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &verified.Task, parsed))
		changed := *parsed
		changed.TargetCommit = strings.Repeat("e", 40)
		if changed.RequestID(item.ID) != parsed.RequestID(item.ID) {
			t.Fatal("deploy renewed evidence allowance")
		}
		forged := *parsed
		forged.OwnerID = "foreign"
		if task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &verified.Task, &forged) == nil {
			t.Fatal("changed authority accepted")
		}
		forged = *parsed
		forged.ReviewRequestID = "unrecorded-review"
		if task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &verified.Task, &forged) == nil {
			t.Fatal("unrecorded decision accepted")
		}
		after, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
		must(err)
		if len(after.Attempts) != before || after.Status == task.TaskStatusDone {
			t.Fatal("continuation duplicated work or accepted it")
		}
		if !strings.Contains(job.Revision, "actual requested components") {
			t.Fatal("targeted verification gap missing")
		}
		parent, err := s.taskSvc.Store().GetAttempt(ctx, attempt.ID)
		must(err)
		parent.CancelRequested = true
		must(s.taskSvc.Store().UpdateAttempt(ctx, parent))
		if task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &verified.Task, parsed) == nil {
			t.Fatal("revoked inspection granted evidence continuation")
		}
		parent.CancelRequested = false
		must(s.taskSvc.Store().UpdateAttempt(ctx, parent))
		must(s.taskSvc.Store().DeleteTask(ctx, blocker.ID))
		reserved, err := s.taskSvc.ClaimLaunch(ctx, time.Now(), "evidence-dispatcher", job.ID)
		must(err)
		if reserved == nil {
			t.Fatal("continuation not claimable")
		}
		child, err := s.taskSvc.CreateAttempt(ctx, item.ID, job.OwnerID, task.CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: job.AgentID, RequestID: job.RequestID, ExpectedPredecessorID: attempt.ID})
		must(err)
		child.Status = task.AttemptStatusSucceeded
		child.ExitCode = new(int)
		end := time.Now().UTC()
		child.FinishedAt = &end
		must(s.taskSvc.Store().UpdateAttempt(ctx, child))
		second := *parsed
		second.PreviousAttemptID = child.ID
		second.PreviousCommit = parsed.TargetCommit
		second.ObservedAt = time.Now().UTC()
		if task.ValidateLiveInspectionSource(ctx, s.taskSvc.Store(), &verified.Task, &second) == nil {
			t.Fatal("evidence completion renewed itself")
		}
		return
	}
	s.markAcceptedVerificationSnapshots(ctx)
	retentionReceipt := filepath.Join(snapshot, ".git", "cuan-verification-accepted.json")
	if _, err := os.Stat(retentionReceipt); !os.IsNotExist(err) {
		t.Fatal("unaccepted verifier snapshot received retention grant", err)
	}
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
	s.markAcceptedVerificationSnapshots(ctx)
	grant, err := os.ReadFile(retentionReceipt)
	must(err)
	var receipt struct {
		TaskID string `json:"task_id"`
		Source string `json:"source_commit"`
		Target string `json:"target_commit"`
	}
	must(json.Unmarshal(grant, &receipt))
	if receipt.TaskID != accepted.ID || receipt.Source != proof.SourceCommit || receipt.Target != proof.TargetCommit {
		t.Fatal("retention grant lost native source identity", receipt)
	}
	after, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	if after.Status != task.TaskStatusDone || len(after.Attempts) != len(accepted.Attempts) {
		t.Fatal("retention changed acceptance or attempts")
	}
}
