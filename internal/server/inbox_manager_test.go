package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"database/sql"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func managerFixture(t *testing.T) (*Server, *task.TaskDetailResponse) {
	t.Helper()
	s, d, _ := stalledReviewFixture(t, "completed")
	for _, id := range []string{"earlier-one", "earlier-two"} {
		if err := s.taskSvc.Store().CreateAttempt(context.Background(), &task.TaskAttempt{ID: id, TaskID: d.ID, Status: task.AttemptStatusFailed, CreatedAt: d.LatestAttempt.CreatedAt.Add(-time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	current, err := s.taskSvc.GetTaskDetail(context.Background(), d.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s, current
}

func TestGenericReviewContinuationKeepsVerificationReadOnly(t *testing.T) {
	for _, remaining := range []string{"verification", "implementation"} {
		for _, action := range []string{"continue", "replan"} {
			t.Run(remaining+"/"+action, func(t *testing.T) {
				s, d := managerFixture(t)
				ctx := context.Background()
				// Generic findings comparison is operational work. Code-lane
				// verification must use the separate exact-source verifier.
				if remaining == "verification" {
					d.ExecutionCategory = task.ExecutionCategoryOps
					if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
						t.Fatal(err)
					}
				}
				q := reviewRequests(t, s, d.SourceRoomID)[0]
				if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: d.ExecutionCategory}); err != nil {
					t.Fatal(err)
				}
				feedback := "Deploy the branch, change the configuration and rerun the original operation to obtain proof."
				decision := inboxReviewEvaluation{Decision: "revise", Finding: "The original outcome is not verified", Explanation: "Check retained work", RemainingWork: remaining, Revision: feedback, ManagerAction: action, ManagerReason: "An existing in-scope gap remains"}
				raw, _ := json.Marshal(decision)
				s.applyInboxReviewEvaluation(ctx, q, string(raw))
				launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID))
				if err != nil || launch == nil || launch.State != "queued" {
					t.Fatal("continuation was not persisted", err)
				}
				limits := executionpolicy.FromInstructions(launch.Revision)
				if remaining == "verification" {
					if !limits.ReadOnlyRepository || !limits.DenyPush || !limits.DenyDeploy || !strings.Contains(launch.Revision, feedback) || !strings.Contains(launch.Revision, "Untrusted reviewer feedback") {
						t.Fatal("model feedback escaped verification limits", limits)
					}
				} else if limits.ReadOnlyRepository || launch.Revision != feedback {
					t.Fatal("implementation correction was changed into verification")
				}
				s.applyInboxReviewEvaluation(ctx, q, string(raw))
				repeated, err := s.taskSvc.GetLaunch(ctx, launch.ID)
				if err != nil || repeated.Revision != launch.Revision {
					t.Fatal("retry changed saved execution scope", err)
				}
			})
		}
	}
}

func TestManagerContinuesApprovedWorkWithoutAnotherOwnerApproval(t *testing.T) {
	for _, action := range []string{"continue", "replan"} {
		t.Run(action, func(t *testing.T) {
			s, d := managerFixture(t)
			ctx := context.Background()
			d.ExecutionCategory = task.ExecutionCategoryOps
			if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
				t.Fatal(err)
			}
			requests := reviewRequests(t, s, d.SourceRoomID)
			q := requests[0]
			if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryOps}); err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(inboxReviewEvaluation{Decision: "revise", Finding: "Satu hasil masih perlu diperiksa.", Explanation: "Pemeriksaan berikutnya memakai akses yang sudah tersedia.", RemainingWork: "verification", Revision: "Compare retained source evidence with the original criteria using the registered read-only inspector. Do not modify or deploy.", ManagerAction: action, ManagerReason: "Original scope is approved; the missing check uses existing access."})
			output := s.applyInboxReviewEvaluation(ctx, q, string(raw))
			if strings.Contains(output, "cuan-approval") || strings.Contains(output, "Setujui") {
				t.Fatal("agent decision offloaded to owner", output)
			}
			launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID))
			if err != nil || launch.State != "queued" {
				t.Fatal("manager did not save in-scope continuation", err, output)
			}
			current, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
			if len(current.Attempts) != 3 || taskRevisionAttemptCount(current) != 3 || current.Status == task.TaskStatusDone {
				t.Fatal("continuation reset history or accepted unfinished result")
			}
			s.applyInboxReviewEvaluation(ctx, q, string(raw))
			repeated, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
			if len(repeated.Attempts) != 3 {
				t.Fatal("lost response created a duplicate execution")
			}
		})
	}
}

func TestManagerVerificationEnvelopeRetainsOriginalApproval(t *testing.T) {
	s, d := managerFixture(t)
	ctx := context.Background()
	q := reviewRequests(t, s, d.SourceRoomID)[0]
	row, err := s.taskSvc.Store().GetTask(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Exercise both manager checks, including the original-approval check in
	// delegation. Fixtures without this binding used to miss the second gate.
	row.ExecutionCategory = task.ExecutionCategoryOps
	row.Instructions += "\nApproval message: " + d.SourceMessageID
	if err := s.taskSvc.Store().UpdateTask(ctx, row); err != nil {
		t.Fatal(err)
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryOps}); err != nil {
		t.Fatal(err)
	}
	decision := inboxReviewEvaluation{Decision: "revise", Finding: "Capture the actual requested component", Explanation: "Complete the missing visual evidence", RemainingWork: "verification", Revision: "Capture the existing RoomCard at390px and desktop without changing source.", ManagerAction: "replan", ManagerReason: "The previous screenshot showed another component; use a targeted fixture."}
	raw, _ := json.Marshal(decision)
	output := s.applyInboxReviewEvaluation(ctx, q, string(raw))
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID))
	if err != nil || launch == nil || launch.State != "queued" {
		t.Fatal("original approved verification returned to review instead of admission", err, output)
	}
	limits := executionpolicy.FromInstructions(launch.Revision)
	if !limits.ReadOnlyRepository || !limits.DenyPush || !limits.DenyDeploy {
		t.Fatal("verification lost scope", limits)
	}
	current, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if len(current.Attempts) != 3 || taskRevisionAttemptCount(current) != 3 {
		t.Fatal("admission reset attempt history")
	}
	if !s.managerContinuationAllowed(ctx, q, current, launch.Revision) || s.managerContinuationAllowed(ctx, q, current, launch.Revision+"\nDeploy anyway") {
		t.Fatal("scope envelope comparison accepted altered feedback or rejected native envelope")
	}
}

func TestManagerProtocolRejectsFakeOwnerDecisionAndMissingBasis(t *testing.T) {
	for _, decision := range []inboxReviewEvaluation{
		{Decision: "revise", RemainingWork: "implementation", Revision: "Fix original bug", Explanation: "Continue", ManagerAction: "needs_owner", ManagerReason: "Three attempts occurred"},
		{Decision: "revise", RemainingWork: "verification", Revision: "Check", Explanation: "Continue", ManagerAction: "continue"},
		{Decision: "revise", RemainingWork: "external", Revision: "Proceed", Explanation: "Continue", ManagerAction: "continue", ManagerReason: "No access"},
		{Decision: "pass", RemainingWork: "external", Explanation: "Done", ManagerAction: "wait", ManagerReason: "External service unavailable"},
	} {
		raw, _ := json.Marshal(decision)
		if _, err := parseInboxReviewEvaluation(string(raw)); err == nil {
			t.Fatal("invalid ownership decision accepted", decision.ManagerAction)
		}
	}
}

func TestPublicationGuardIsIdempotentForSavedManagerDecision(t *testing.T) {
	for _, remaining := range []string{"implementation", "verification"} {
		t.Run(remaining, func(t *testing.T) {
			s, d, _ := stalledReviewFixture(t, "completed")
			ctx := context.Background()
			projects := project.NewMemoryStore()
			if err := projects.CreateProject(ctx, &project.Project{ID: d.ProjectID, RepoURL: "https://github.com/example/project", Metadata: registeredReleaseMetadata(t)}); err != nil {
				t.Fatal(err)
			}
			s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
			d.CommitSHA = strings.Repeat("a", 40)
			decision := inboxReviewEvaluation{Decision: "revise", RemainingWork: remaining, Finding: "Requested change needs verification", Explanation: "Inspect current result", Revision: "Inspect existing deployment effects; preserve approved scope", ManagerAction: "replan", ManagerReason: "Existing outcome needs a different inspection"}
			first := s.requirePublicationReview(ctx, d, decision)
			second := s.requirePublicationReview(ctx, d, first)
			legacy := first
			legacy.RemainingWork = "implementation"
			if normalized := s.requirePublicationReview(ctx, d, legacy); normalized != first {
				t.Fatal("old wrapped revision bypassed verification ordering")
			}
			if first != second || first.Revision == decision.Revision || first.ManagerAction != "replan" {
				t.Fatal("publication guard changed saved continuation identity or was not exercised")
			}
		})
	}
}

func TestRepeatedIdenticalRevisionReturnsToManagerNotOwner(t *testing.T) {
	s, d := managerFixture(t)
	ctx := context.Background()
	// This scenario changes an operational evidence-gathering strategy; it has
	// no source checkpoint. Code work instead requires exact-source review.
	d.ExecutionCategory = task.ExecutionCategoryOps
	if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	q := reviewRequests(t, s, d.SourceRoomID)[0]
	revision := "Repeat the same unsuccessful check"
	for i, id := range []string{"earlier-one", "earlier-two"} {
		raw, _ := json.Marshal(reviewAssessmentRecord{RequestID: id, InstructionsHash: task.LaunchInstructionsHash(d.Instructions), Assessment: inboxReviewEvaluation{Decision: "revise", Revision: revision}})
		if err := s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: "old-assessment-" + id, TaskID: d.ID, AttemptID: &id, ActivityType: reviewAssessmentActivity, Author: "coordinator", Message: string(raw), CreatedAt: d.LatestAttempt.CreatedAt.Add(time.Duration(i-3) * time.Minute)}); err != nil {
			t.Fatal(err)
		}
	}
	raw, _ := json.Marshal(inboxReviewEvaluation{Decision: "revise", Finding: "Same check still failed", Explanation: "Check needs another approach", RemainingWork: "verification", Revision: revision, ManagerAction: "continue", ManagerReason: "Existing scope is approved"})
	output := s.applyInboxReviewEvaluation(ctx, q, string(raw))
	if strings.Contains(output, "cuan-approval") {
		t.Fatal("repeated failure offloaded to owner")
	}
	if _, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID)); err == nil {
		t.Fatal("identical rejected step blindly repeated")
	}
	current, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	key := managerReviewKey(current)
	var manager *db.InboxRequest
	for _, request := range reviewRequests(t, s, d.SourceRoomID) {
		if request.RequestID == delegationMessageID(key) {
			copy := request
			manager = &copy
		}
	}
	if manager == nil || manager.State != "queued" {
		t.Fatal("internal replan not queued")
	}
	if err := s.taskSvc.Store().CreateTask(ctx, &task.Task{ID: "manager-busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryOps}); err != nil {
		t.Fatal(err)
	}
	changed, _ := json.Marshal(inboxReviewEvaluation{Decision: "revise", Finding: "Original check is inconclusive", Explanation: "Use independent current-result evidence", RemainingWork: "verification", Revision: "Use the registered independent inspector instead of repeating the earlier self-reported check.", ManagerAction: "replan", ManagerReason: "The previous identical check failed repeatedly; independent evidence uses existing access."})
	s.applyInboxReviewEvaluation(ctx, *manager, string(changed))
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, manager.RequestID))
	if err != nil || launch.State != "queued" {
		t.Fatal("coordinator could not select another in-scope approach", err)
	}
}

func TestManagerStillAsksForGenuineExternalOwnerDecision(t *testing.T) {
	for _, action := range []string{"wait", "needs_owner"} {
		decision := inboxReviewEvaluation{Decision: "blocked", RemainingWork: "external", Explanation: "The exact missing business identity is required.", ManagerAction: action, ManagerReason: "The required information is absent from the saved conversation."}
		raw, _ := json.Marshal(decision)
		if _, err := parseInboxReviewEvaluation(string(raw)); err != nil {
			t.Fatal("genuine external decision rejected", err)
		}
	}
}

func TestManagerContinuationPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		ctx := context.Background()
		s, origin, d := phaseLineageSetup(t, database)
		s.inboxStopping.Store(true)
		item, _ := s.taskSvc.Store().GetTask(ctx, d.ID)
		item.Status = task.TaskStatusReview
		item.Summary = "Existing phase result needs independent checks"
		item.CompleteNote = "Evidence retained, acceptance unfinished"
		item.Instructions += "\nApproval message: " + d.SourceMessageID
		if err := s.taskSvc.Store().UpdateTask(ctx, item); err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"manager-pg-one", "manager-pg-two"} {
			if err := s.taskSvc.Store().CreateAttempt(ctx, &task.TaskAttempt{ID: id, TaskID: d.ID, Status: task.AttemptStatusFailed, CreatedAt: d.LatestAttempt.CreatedAt.Add(-time.Hour)}); err != nil {
				t.Fatal(err)
			}
		}
		key := "task-review:" + d.LatestAttempt.ID
		now := time.Now().UTC()
		q := db.InboxRequest{OwnerID: origin.OwnerID, AgentID: origin.AgentID, RoomID: origin.RoomID, RequestID: delegationMessageID(key), MessageID: delegationMessageID(key + ":input"), ReplyID: delegationMessageID(key + ":reply"), PayloadHash: delegationMessageID(key), State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
		if _, _, err := s.projectStudioRepo.(db.InboxRequestRepository).AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: "Review current outcome [/tasks/" + d.ID + "]", CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(inboxReviewEvaluation{Decision: "revise", Finding: "Phase checks are unfinished", Explanation: "Inspect retained evidence using the approved scope", RemainingWork: "verification", Revision: "Perform the missing phase-one verification without publishing or changing the approved phase plan.", ManagerAction: "replan", ManagerReason: "Original phase is approved; independent checking is an internal next step."})
		output := s.applyInboxReviewEvaluation(ctx, q, string(raw))
		launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID))
		if err != nil || launch.State != "queued" || strings.Contains(output, "cuan-approval") {
			t.Fatal("durable manager continuation lost", err, output)
		}
		limits := executionpolicy.FromInstructions(launch.Revision)
		if !limits.ReadOnlyRepository || !limits.DenyPush || !limits.DenyDeploy {
			t.Fatal("native verification envelope lost its limits", limits)
		}
		current, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
		if len(current.Attempts) != 3 || current.Status == task.TaskStatusDone {
			t.Fatal("manager reset history or accepted unfinished work")
		}
	})
}
