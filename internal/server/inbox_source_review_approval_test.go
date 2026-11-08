package server

import (
	"context"
	"encoding/json"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/publication"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestNativeSourceReviewRequiresBoundIndependentEvidence(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "completed")
	verificationFollowupQueueContract(t, s, d, q)
	ctx := context.Background()
	store := s.taskSvc.Store()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_ = store.DeleteTask(ctx, "phase-blocker")
	must(store.DeleteTask(ctx, "busy"))
	// The older review fixture lacks a real source execution interval; this
	// admission contract requires completed source work before verifier start.
	source, err := store.GetAttempt(ctx, d.LatestAttempt.ID)
	must(err)
	start, finish, exit := time.Now().Add(-2*time.Minute), time.Now().Add(-time.Minute), 0
	source.CreatedAt, source.StartedAt, source.FinishedAt, source.ExitCode = start.Add(-time.Minute), &start, &finish, &exit
	must(store.UpdateAttempt(ctx, source))
	d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
	must(err)
	job, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, task.VerificationRequestID(d.ID, d.CommitSHA, d.LatestAttempt.ID)))
	must(err)
	_, err = store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "source-review-launch", job.ID)
	must(err)
	att, err := s.taskSvc.CreateAttempt(ctx, d.ID, "owner", task.CreateAttemptRequest{RunnerID: "source-reviewer", AgentName: d.AssignedAgent, RequestID: job.RequestID})
	must(err)
	service := runner.NewService(runner.NewMemoryStore(store), store, nil)
	claim, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "source-reviewer"}, time.Minute)
	must(err)
	if !claim.Claimed {
		t.Fatal("verifier not claimed")
	}
	_, err = service.Complete(ctx, att.ID, runner.CompleteAttemptRequest{RunnerID: "source-reviewer", Status: task.AttemptStatusSucceeded, VerifiedSourceCommit: d.CommitSHA, AgentReportRequired: true, AgentReportConfirmed: true, RepositoryVerification: &runner.RepositoryVerification{Baseline: strings.Repeat("b", 64), Final: strings.Repeat("b", 64)}})
	must(err)
	d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
	must(err)
	assessment := inboxReviewEvaluation{Decision: "revise", SourceReview: "pass", RemainingWork: "publication", Explanation: "Source checked independently; publication pending.", Revision: "Publish the reviewed source only."}
	if s.saveNativeSourceReview(ctx, d, q.RequestID, assessment) == nil {
		t.Fatal("unsaved model verdict approved source")
	}
	report := &task.TaskActivity{ID: "independent-final-report", TaskID: d.ID, AttemptID: &att.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Author: "agent", Message: "Observed source and targeted regression checks passed; publication remains pending.", CreatedAt: d.LatestAttempt.FinishedAt.Add(-time.Nanosecond)}
	must(store.CreateActivity(ctx, report))
	d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
	must(err)
	_, err = s.saveReviewAssessment(ctx, d, q, assessment)
	must(err)
	approved, err := task.NativeSourceReviewForAttempt(ctx, store, &d.Task, att.ID)
	must(err)
	if approved == nil || approved.SourceCommit != d.CommitSHA || approved.ReviewAttemptID != att.ID {
		t.Fatal("review lost source binding", approved)
	}
	must(s.saveNativeSourceReview(ctx, d, q.RequestID, assessment))

	t.Run("historical live failure follows exact source across developer handoff", func(t *testing.T) {
		_, bound := s.verifiedSupplementalSource(ctx, d)
		if bound == nil {
			t.Fatal("missing source proof")
		}
		finished := d.LatestAttempt.CreatedAt.Add(-5 * time.Second)
		created := finished.Add(-15 * time.Second)
		live := task.LiveInspectionIntent{Purpose: task.CurrentOutcomeInspection, SourceAttemptID: "earlier-source-attempt", SourceCommit: bound.SourceCommit, TargetCommit: strings.Repeat("f", 40), Repository: "fixture/repo", OwnerID: bound.OwnerID, RoomID: bound.RoomID, SourceRequestID: bound.SourceRequestID, InstructionsHash: bound.InstructionsHash, ObservedAt: created}
		encoded, _ := json.Marshal(live)
		requestID := live.RequestID(d.ID)
		job, e := store.(task.LaunchStore).SaveLaunch(ctx, task.TaskLaunch{TaskID: d.ID, ProjectID: d.ProjectID, RequestID: requestID, OwnerID: bound.OwnerID, RoomID: bound.RoomID, SourceRequestID: bound.SourceRequestID, AgentID: d.AssignedAgent, InstructionsHash: bound.InstructionsHash, Revision: "Scope: read-only repository. Do not push. Do not deploy.\n" + task.LiveInspectionMarker + string(encoded), CreatedAt: created, AvailableAt: created, ExpiresAt: time.Now().Add(time.Hour)})
		must(e)
		if _, e := store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "historical-live", job.ID); e != nil {
			t.Fatal(e)
		}
		id := "att_" + strings.TrimPrefix(job.ID, "launch_")[:32]
		exit := 1
		old := &task.TaskAttempt{ID: id, TaskID: d.ID, RunnerID: "historical-runner", AgentName: d.AssignedAgent, CreatedAt: created, StartedAt: &created, FinishedAt: &finished, ExitCode: &exit, Status: task.AttemptStatusFailed, ErrorMessage: "Live result inspection project checks did not pass"}
		must(store.CreateAttempt(ctx, old))
		record, _ := json.Marshal(task.RunnerVerificationRecord{Summary: "Project verification: test.sh --fast (exit 1)\nConcrete native check failure"})
		must(store.CreateActivity(ctx, &task.TaskActivity{ID: "verification:" + id, TaskID: d.ID, AttemptID: &id, Author: old.RunnerID, ActivityType: task.ActivityTypeRunnerVerification, Message: string(record), CreatedAt: finished}))
		current, e := s.taskSvc.GetTaskDetail(ctx, d.ID)
		must(e)
		context := s.verifierCorrectionContext(ctx, current)
		if context == nil || context["source_attempt_id"] != bound.SourceAttemptID || context["historical_source_attempt_id"] != "earlier-source-attempt" {
			t.Fatal("same exact source lost its original native failure across handoff", context)
		}
		origin := db.InboxRequest{OwnerID: bound.OwnerID, RoomID: bound.RoomID, RequestID: bound.SourceRequestID}
		if !s.runningReleaseChecksAlreadyFailed(ctx, current, origin, live.TargetCommit, live.Repository) {
			t.Fatal("new source attempt erased unchanged running release check failure")
		}
		for _, mode := range []string{"changed target", "different repository", "foreign owner", "changed scope", "untrusted check"} {
			dd, request, target, repository := *current, origin, live.TargetCommit, live.Repository
			switch mode {
			case "changed target":
				target = strings.Repeat("e", 40)
			case "different repository":
				repository = "other/repo"
			case "foreign owner":
				request.OwnerID = "foreign"
			case "changed scope":
				dd.Instructions += " changed scope"
			case "untrusted check":
				dd.Activities = nil
			}
			if s.runningReleaseChecksAlreadyFailed(ctx, &dd, request, target, repository) {
				t.Fatal("unbound historical failure blocked a new inspection", mode)
			}
		}
		foreign := *current
		foreign.Instructions += " Changed original scope"
		if s.verifierCorrectionContext(ctx, &foreign) != nil {
			t.Fatal("changed scope reused historical native failure")
		}
	})

	t.Run("native findings refresh a stranded saved verification decision", func(t *testing.T) {
		must(s.taskSvc.FinishLaunch(ctx, job.ID, "source-review-launch", "stopped", time.Now(), "Execution finished"))
		d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
		must(err)
		p, e := s.projectSvc.Store().GetProject(ctx, d.ProjectID)
		must(e)
		p.Metadata = registeredReleaseMetadata(t)
		if p.RepoURL == "" {
			p.RepoURL = "https://github.com/fixture/repo"
		}
		must(s.projectSvc.Store().UpdateProject(ctx, p))
		s.cfg.Environment = "production"
		s.cfg.SourceRepository = "unrelated/config"
		s.publicationObserver = func(context.Context, publication.Config) (string, error) { return strings.Repeat("f", 40), nil }
		// A PM may correctly classify the work without reproducing native prose.
		// Exact source/failure/scope evidence must determine routing, not wording.
		rawCorrection := inboxReviewEvaluation{Decision: "revise", RequestedOutcome: "unfinished", RemainingWork: "implementation", Revision: "Reconcile the concrete remaining original work", ManagerAction: "replan", ManagerReason: "Independent checks identify a source integration gap"}
		normalized := s.requirePublicationReview(ctx, d, rawCorrection)
		if normalized.RemainingWork != "implementation" || !strings.HasPrefix(normalized.Revision, "Native publication reconciliation: native source integration finding:") {
			t.Fatalf("PM implementation decision fell back to repeated verification: %+v", normalized)
		}
		if twice := s.requirePublicationReview(ctx, d, normalized); twice != normalized {
			t.Fatal("native normalization changed the saved correction on reapplication")
		}
		// Publication observation may persist new native connection evidence.
		d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
		must(err)
		key := "task-review:" + d.LatestAttempt.ID
		review := db.InboxRequest{OwnerID: q.OwnerID, AgentID: q.AgentID, RoomID: d.SourceRoomID, RequestID: delegationMessageID(key), MessageID: delegationMessageID(key + ":input"), ReplyID: delegationMessageID(key + ":reply"), CreatedAt: time.Now().UTC(), Deadline: time.Now().Add(time.Hour), State: "queued", PayloadHash: delegationMessageID(key)}
		_, _, e = s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, review, &db.ProjectMessage{ID: review.MessageID, RoomID: review.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: inboxOwnershipPolicy + "\nReview [/tasks/" + d.ID + "].", CreatedAt: review.CreatedAt})
		must(e)
		stale := inboxReviewEvaluation{Decision: "revise", RemainingWork: "verification", Finding: "Existing evidence requires reconciliation", Explanation: "Inspect the original result", Revision: verificationReviewRevision("Inspect the existing release"), ManagerAction: "replan", ManagerReason: "Evaluate the saved result without repeating effects"}
		_, e = s.saveReviewAssessment(ctx, d, review, stale)
		must(e)
		review.State = "completed"
		must(s.projectStudioRepo.(db.InboxRequestRepository).FinishInboxRequest(ctx, review, nil))
		requests, e := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, d.SourceRoomID)
		must(e)
		var origin db.InboxRequest
		for _, r := range requests {
			if r.MessageID == d.SourceMessageID {
				origin = r
			}
		}
		current, e := s.taskSvc.GetTaskDetail(ctx, d.ID)
		must(e)
		if !s.recoverManagerVerificationHandoff(ctx, current, origin, requests) {
			t.Fatalf("native source finding did not schedule fresh PM assessment: %+v", s.verifierHandoffDiagnostic(ctx, current))
		}
		requests, e = s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, d.SourceRoomID)
		must(e)
		count := len(requests)
		if !s.recoverManagerVerificationHandoff(ctx, current, origin, requests) {
			t.Fatal("queued reassessment was not retained")
		}
		again, e := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, d.SourceRoomID)
		must(e)
		if len(again) != count {
			t.Fatal("fresh assessment duplicated")
		}
		saved, found := reviewAssessment(current, review.RequestID)
		if !found || saved.RemainingWork != "verification" {
			t.Fatal("historical review was rewritten")
		}
		if current.LatestAttempt.ID != d.LatestAttempt.ID {
			t.Fatal("historical source verifier was replayed")
		}
	})
	t.Run("separate developer correction", func(t *testing.T) {
		d, err = s.taskSvc.GetTaskDetail(ctx, d.ID)
		must(err)
		key := "task-review:" + d.LatestAttempt.ID + ":verifier-correction-v5:" + inboxReviewEvidenceDigest(d)
		review := db.InboxRequest{OwnerID: q.OwnerID, AgentID: q.AgentID, RoomID: d.SourceRoomID, RequestID: delegationMessageID(key), MessageID: delegationMessageID(key + ":input"), ReplyID: delegationMessageID(key + ":reply"), CreatedAt: time.Now().UTC(), Deadline: time.Now().Add(time.Hour), State: "queued", PayloadHash: delegationMessageID(key)}
		_, _, e := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, review, &db.ProjectMessage{ID: review.MessageID, RoomID: review.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: inboxOwnershipPolicy + "\nReview [/tasks/" + d.ID + "].", CreatedAt: review.CreatedAt})
		must(e)
		p, e := s.projectSvc.Store().GetProject(ctx, d.ProjectID)
		must(e)
		p.Metadata = json.RawMessage(`{"cuan_settings":{"auto_deploy":false}}`)
		must(s.projectSvc.Store().UpdateProject(ctx, p))
		correction := inboxReviewEvaluation{Decision: "revise", RequestedOutcome: "unfinished", RemainingWork: "implementation", Finding: "A scoped source correction remains", Explanation: "Correct the original result", Revision: "Reconcile only the identified original implementation gap against current project source; preserve unrelated work.", ManagerAction: "continue", ManagerReason: "Independent verification identified a concrete remaining gap"}
		if s.verifiedImplementationCorrectionAllowed(ctx, review, d, correction.Revision) {
			t.Fatal("unsaved reviewer text authorized developer work")
		}
		_, e = s.saveReviewAssessment(ctx, d, review, correction)
		must(e)
		current, e := s.taskSvc.GetTaskDetail(ctx, d.ID)
		must(e)
		if !s.verifiedImplementationCorrectionAllowed(ctx, review, current, correction.Revision) {
			t.Fatal("native verifier and saved assessment could not hand back to developer")
		}
		if !s.managerContinuationAllowed(ctx, review, current, correction.Revision) {
			t.Fatal("manager blocked separately authorized developer correction")
		}
		for _, mode := range []string{"foreign owner", "changed revision", "cancelled verifier", "changed instructions", "readonly owner"} {
			dd := *current
			aa := *current.LatestAttempt
			dd.LatestAttempt = &aa
			rr := review
			revision := correction.Revision
			switch mode {
			case "foreign owner":
				rr.OwnerID = "other"
			case "changed revision":
				revision += " unexpected scope"
			case "cancelled verifier":
				aa.CancelRequested = true
			case "changed instructions":
				dd.Instructions += " changed"
			case "readonly owner":
				dd.Instructions += "\nScope: read-only repository. Do not push. Do not deploy."
			}
			if s.verifiedImplementationCorrectionAllowed(ctx, rr, &dd, revision) {
				t.Fatal("invalid developer correction accepted", mode)
			}
		}
		must(store.CreateTask(ctx, &task.Task{ID: "correction-capacity", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}))
		raw, _ := json.Marshal(correction)
		review.State = "completed"
		must(s.projectStudioRepo.(db.InboxRequestRepository).FinishInboxRequest(ctx, review, nil))
		requests, e := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, d.SourceRoomID)
		must(e)
		var origin db.InboxRequest
		for _, request := range requests {
			if request.MessageID == d.SourceMessageID {
				origin = request
			}
		}
		if !s.recoverManagerVerificationHandoff(ctx, current, origin, requests) {
			t.Fatalf("saved implementation decision stranded: auth=%v origin=%+v manager=%v requests=%d", s.reviewSnapshotRecoveryAuthorized(ctx, current, origin), origin, s.managerContinuationAllowed(ctx, review, current, correction.Revision), len(requests))
		}
		output := "persisted handoff recovery"

		launch, e := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, review.RequestID))
		if e != nil {
			t.Fatalf("launch=%v output=%s", e, output)
		}
		if launch == nil || launch.State != "queued" || !strings.Contains(launch.Revision, "Separate developer correction after independent source verification") {
			t.Fatal("correction did not preserve native source provenance", output)
		}
		limits := executionpolicy.FromInstructions(launch.Revision)
		original, _ := s.inboxApprovedTaskScope(ctx, current)
		if original == nil || limits.ReadOnlyRepository != original.ReadOnlyRepository || limits.DenyPush != original.DenyPush || limits.DenyDeploy != original.DenyDeploy {
			t.Fatal("generated correction context manufactured new scope restrictions", limits, original)
		}
		latest, e := s.taskSvc.GetTaskDetail(ctx, d.ID)
		must(e)
		if latest.LatestAttempt.ID != current.LatestAttempt.ID || latest.Status == task.TaskStatusDone {
			t.Fatal("correction mutated verifier history or accepted unfinished result")
		}
		s.applyInboxReviewEvaluation(ctx, review, string(raw))
		again, e := s.taskSvc.GetTaskDetail(ctx, d.ID)
		must(e)
		if len(again.Attempts) != len(latest.Attempts) {
			t.Fatal("lost response duplicated execution")
		}
	})
	changed := d.Task
	changed.CommitSHA = strings.Repeat("c", 40)
	if _, err := task.NativeSourceReviewForAttempt(ctx, store, &changed, att.ID); err == nil {
		t.Fatal("approval accepted different source")
	}
	report.Message = "Replacement report claiming a different result"
	report.ID, report.CreatedAt = "replacement-report", time.Now()
	must(store.CreateActivity(ctx, report))
	if _, err := task.NativeSourceReviewForAttempt(ctx, store, &d.Task, att.ID); err == nil {
		t.Fatal("approval survived report replacement")
	}
	if d.Status == task.TaskStatusDone {
		t.Fatal("source approval accepted task")
	}
}
