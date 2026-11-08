package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/buchin/cuan/internal/git"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/publication"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestLiveResultInspectionLifecycle(t *testing.T) {
	for _, scenario := range []string{"valid", "wrong commit", "missing ancestry", "failed checks", "changed repository", "missing report", "cancelled", "foreign owner", "different repository", "checks forbidden", "release changed", "recover checks", "current outcome", "current outcome missing identity", "current outcome forged ancestry", "current outcome release changed", "current outcome reported checks", "current outcome checks missing report", "current outcome opaque local", "current outcome opaque foreign", "current outcome opaque unfinished", "current outcome opaque cancelled", "current outcome revalidation", "current outcome publication valid", "current outcome failed checks revalidation"} {
		t.Run(scenario, func(t *testing.T) {
			s, origin, d := phaseLineageSetup(t)
			liveInspectionContract(t, s, origin, d, scenario, nil)
		})
	}
}

func TestLocalOnlyFailureDoesNotInspectUnrelatedRelease(t *testing.T) {
	for _, scenario := range []string{"current outcome local only", "current outcome local branch only", "committed local branch only"} {
		t.Run(scenario, func(t *testing.T) {
			s, origin, d := phaseLineageSetup(t)
			liveInspectionContract(t, s, origin, d, scenario, nil)
		})
	}
}

func TestInspectionInfrastructureRecovery(t *testing.T) {
	for _, mode := range []string{"launch", "setup"} {
		t.Run(mode, func(t *testing.T) {
			s, origin, d := phaseLineageSetup(t)
			liveInspectionContract(t, s, origin, d, "current outcome unpublished valid infrastructure "+mode, nil)
		})
	}
}

func TestInspectionInfrastructureRecoveryPostgres(t *testing.T) {
	for _, mode := range []string{"launch", "setup"} {
		t.Run(mode, func(t *testing.T) {
			withPhasePostgres(t, func(database *sql.DB) {
				s, origin, d := phaseLineageSetup(t, database)
				liveInspectionContract(t, s, origin, d, "current outcome unpublished valid infrastructure "+mode, database)
			})
		})
	}
}

func TestUnrecordedLocalCurrentOutcomeLifecycle(t *testing.T) {
	for _, scenario := range []string{"local", "local missing report", "local failed checks", "local missing identity", "local changed repository", "local release changed", "foreign runner", "missing start", "zero start", "future start", "future creation", "cancelled", "cancelled at", "running", "foreign owner", "different repository", "checks forbidden"} {
		t.Run(scenario, func(t *testing.T) {
			s, origin, d := phaseLineageSetup(t)
			liveInspectionContract(t, s, origin, d, "current outcome unrecorded "+scenario, nil)
		})
	}
}

func TestUnpublishedCurrentOutcomeLifecycle(t *testing.T) {
	for _, scenario := range []string{"valid", "valid preparation", "valid scope preparation", "valid verifier repair", "valid registered review", "valid registered review handoff", "valid missing report", "valid failed checks", "valid changed repository", "valid missing identity", "valid forged ancestry", "valid release changed", "missing audit", "missing source report", "owner prohibition", "cancelled source", "failed source"} {
		t.Run(scenario, func(t *testing.T) {
			s, origin, d := phaseLineageSetup(t)
			liveInspectionContract(t, s, origin, d, "current outcome unpublished "+scenario, nil)
		})
	}
}

func TestSupplementalVerifierCurrentOutcomeHandoff(t *testing.T) {
	for _, scenario := range []string{"valid", "valid publication review", "stale preparation", "missing receipt", "cancelled verifier", "wrong source"} {
		t.Run(scenario, func(t *testing.T) {
			s, origin, d := phaseLineageSetup(t)
			liveInspectionContract(t, s, origin, d, "current outcome unpublished valid supplemental "+scenario, nil)
		})
	}
}

func TestOriginalInspectionPreparationGrant(t *testing.T) {
	for _, scenario := range []string{"valid", "cancelled source", "changed instructions", "foreign owner"} {
		t.Run(scenario, func(t *testing.T) {
			s, origin, d := phaseLineageSetup(t)
			liveInspectionContract(t, s, origin, d, "current outcome unpublished valid blocked preparation "+scenario, nil)
		})
	}
}

func TestCurrentOutcomeEvidenceRevalidation(t *testing.T) {
	for _, scenario := range []string{"current outcome unpublished valid evidence", "current outcome unpublished valid preparation evidence"} {
		t.Run(scenario, func(t *testing.T) {
			s, origin, d := phaseLineageSetup(t)
			liveInspectionContract(t, s, origin, d, scenario, nil)
		})
	}
}

func TestLiveResultInspectionPostgres(t *testing.T) {
	for _, scenario := range []string{"valid", "recover checks", "current outcome", "current outcome reported checks", "current outcome opaque local", "current outcome unpublished valid", "current outcome unpublished valid evidence", "current outcome unpublished valid preparation evidence", "current outcome unpublished valid preparation", "current outcome unpublished valid scope preparation", "current outcome unpublished valid blocked preparation valid", "current outcome unpublished valid supplemental valid", "current outcome unpublished valid supplemental stale preparation", "current outcome publication valid", "current outcome failed checks revalidation"} {
		t.Run(scenario, func(t *testing.T) {
			withPhasePostgres(t, func(database *sql.DB) {
				s, origin, d := phaseLineageSetup(t, database)
				liveInspectionContract(t, s, origin, d, scenario, database)
			})
		})
	}
}

func liveInspectionContract(t *testing.T, s *Server, origin db.InboxRequest, d *task.TaskDetailResponse, scenario string, database *sql.DB) {
	t.Helper()
	staleSupplemental := scenario == "current outcome unpublished valid supplemental stale preparation"
	evidenceFollowup := strings.HasSuffix(scenario, " evidence")
	if evidenceFollowup {
		scenario = strings.TrimSuffix(scenario, " evidence")
	}
	s.inboxStopping.Store(true)
	ctx := context.Background()
	store := s.taskSvc.Store()
	projects := project.NewMemoryStore()
	if err := projects.CreateProject(ctx, &project.Project{ID: d.ProjectID, Name: "Fixture", RepoURL: "https://github.com/fixture/repo"}); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projects, nil, "fixture", t.TempDir())
	s.taskSvc = task.NewService(store, s.projectSvc)
	s.cfg.Environment, s.cfg.SourceRepository, s.cfg.CommitSHA = "production", "fixture/repo", strings.Repeat("b", 40)
	d.Status, d.CommitSHA, d.ResultKind = task.TaskStatusBlocked, strings.Repeat("a", 40), "implementation"
	d.Instructions = "Cuan managed delegation v1.\nImplement the approved feature, run tests and verify its live result."
	d.Summary, d.CompleteNote = "Prior implementation report", "Prior reported evidence; source project checks failed"
	d.DoDChecklist = []task.DoDItem{{Text: "Feature and checks available", Completed: true, Evidence: "Prior claim for independent rechecking"}}
	now, exit := time.Now().UTC(), 0
	var revalidationCommits []string
	if scenario == "current outcome revalidation" || scenario == "current outcome failed checks revalidation" {
		root := t.TempDir()
		run := func(args ...string) string {
			t.Helper()
			out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
			if err != nil {
				t.Fatal(err, string(out))
			}
			return strings.TrimSpace(string(out))
		}
		run("init", "--initial-branch=main")
		if err := os.MkdirAll(filepath.Join(root, "internal/server"), 0700); err != nil {
			t.Fatal(err)
		}
		sourcePath := filepath.Join(root, "internal/server/terminal_handlers.go")
		if scenario == "current outcome failed checks revalidation" {
			if err := os.MkdirAll(filepath.Join(root, "internal/runner"), 0700); err != nil {
				t.Fatal(err)
			}
			sourcePath = filepath.Join(root, "internal/runner/criterion_test.go")
		}
		for index, content := range []string{"original", "unrelated", "corrected"} {
			name := sourcePath
			if index == 1 {
				name = filepath.Join(root, "README.md")
			}
			if err := os.WriteFile(name, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			run("add", ".")
			run("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", content)
			revalidationCommits = append(revalidationCommits, run("rev-parse", "HEAD"))
		}
		p, err := projects.GetProject(ctx, d.ProjectID)
		if err != nil {
			t.Fatal(err)
		}
		p.LocalPath = root
		p.DefaultBranch = "main"
		if err := projects.UpdateProject(ctx, p); err != nil {
			t.Fatal(err)
		}
		s.projectSvc = project.NewService(projects, git.NewClient(), "fixture", t.TempDir())
		s.taskSvc = task.NewService(store, s.projectSvc)
		s.cfg.CommitSHA = revalidationCommits[0]
	}
	source := &task.TaskAttempt{ID: "live-source", TaskID: d.ID, RunnerID: "old-runner", AgentName: d.AssignedAgent, Status: task.AttemptStatusFailed, ExitCode: &exit, ErrorMessage: "Project verification failed", CreatedAt: now, FinishedAt: &now}
	currentOutcome := strings.HasPrefix(scenario, "current outcome")
	if currentOutcome {
		d.CommitSHA, d.ResultKind, d.Summary, d.CompleteNote = "", "", "", ""
		d.DoDChecklist = []task.DoDItem{{Text: "Original requested behavior works", Completed: false}}
		exit = 124
		source.ErrorMessage = "Execution timed out after adaptive timeout (45m0s)"
	}
	if scenario == "current outcome local only" {
		d.Instructions += "\nDo not push. Do not deploy."
		d.ResultKind = "implementation"
		source.ErrorMessage, exit = "Project verification failed", 0
	}
	if scenario == "current outcome local branch only" {
		d.Instructions += "\nPush the task branch for independent source review. Do not merge or deploy."
	}
	if scenario == "committed local branch only" {
		d.Instructions += "\nPush the task branch for independent source review. Do not merge or deploy."
	}
	if strings.HasPrefix(scenario, "current outcome opaque") {
		source.RunnerID, source.ErrorMessage = "local_terminal", ""
		started := now.Add(-20 * time.Minute)
		source.StartedAt = &started
		if scenario == "current outcome opaque foreign" {
			source.RunnerID = "other-runner"
		}
		if scenario == "current outcome opaque unfinished" {
			source.FinishedAt = nil
		}
		if scenario == "current outcome opaque cancelled" {
			source.CancelRequested = true
		}
	}
	if strings.HasPrefix(scenario, "current outcome unrecorded ") {
		source.RunnerID, source.ErrorMessage = "local_terminal", ""
		source.ExitCode, source.FinishedAt = nil, nil
		started := now.Add(-20 * time.Minute)
		source.StartedAt, source.CreatedAt = &started, now
		switch strings.TrimPrefix(scenario, "current outcome unrecorded ") {
		case "foreign runner":
			source.RunnerID = "other-runner"
		case "missing start":
			source.StartedAt = nil
		case "zero start":
			zeroTime := time.Time{}
			source.StartedAt = &zeroTime
		case "future start":
			future := now.Add(time.Hour)
			source.StartedAt = &future
		case "future creation":
			source.CreatedAt = now.Add(time.Hour)
		case "cancelled":
			source.CancelRequested = true
		case "cancelled at":
			source.CanceledAt = &now
		case "running":
			source.Status = task.AttemptStatusRunning
		case "foreign owner":
			origin.OwnerID = "other"
		case "different repository":
			s.cfg.SourceRepository = "other/repo"
		case "checks forbidden":
			d.Instructions += "\nDo not run tests."
		}
	}
	if scenario == "current outcome reported checks" || scenario == "current outcome checks missing report" {
		exit = 0
		source.ErrorMessage = "Project verification failed"
		d.ResultKind = "implementation"
	}
	publicationFailure := scenario == "current outcome publication valid" || scenario == "current outcome failed checks revalidation"
	if publicationFailure {
		exit = 0
		d.CommitSHA, d.ResultKind = strings.Repeat("a", 40), "implementation"
		source.ErrorMessage, source.StartedAt = task.PublicationVerificationUnavailableMessage, &now
	}
	unpublished := strings.HasPrefix(scenario, "current outcome unpublished ")
	if unpublished {
		d.CommitSHA, d.ResultKind, d.Summary, d.CompleteNote = strings.Repeat("a", 40), "implementation", "Unpublished branch complete", "Source checks reported"
		d.DoDChecklist = []task.DoDItem{{Text: "Original requested feature is live", Completed: true, Evidence: "Historical claim requiring fresh verification"}}
		d.Instructions = "Cuan managed delegation v1.\nImplement and deploy the feature.\nApproval message: " + origin.MessageID + "\nUser authorization: Implement and deploy the feature.\nConversation: " + origin.RoomID + "\nReport actual progress and verification evidence. Stay within this scope; do not deploy unless authorized."
		source.Status, source.ErrorMessage, source.StartedAt = task.AttemptStatusSucceeded, "", &now
		exit = 0
		switch strings.TrimPrefix(scenario, "current outcome unpublished ") {
		case "owner prohibition":
			d.Instructions = strings.ReplaceAll(d.Instructions, "Implement and deploy the feature.", "Do not deploy. Return local results.")
		case "cancelled source":
			source.CancelRequested = true
		case "failed source":
			source.Status = task.AttemptStatusFailed
		}
	}
	blockedPreparation := strings.HasPrefix(scenario, "current outcome unpublished valid blocked preparation ")
	if blockedPreparation {
		// No legacy footer shortcut: the original saved review grants the
		// read-only inspection even if a later model opinion asks for approval.
		d.Instructions = strings.ReplaceAll(d.Instructions, "; do not deploy unless authorized.", ".")
	}
	if strings.HasPrefix(scenario, "current outcome unpublished valid registered review") || scenario == "current outcome unpublished valid supplemental valid publication review" {
		d.Instructions = strings.ReplaceAll(d.Instructions, "; do not deploy unless authorized.", ".")
		if strings.HasSuffix(scenario, " handoff") {
			d.Status = task.TaskStatusReview
		}
		p, _ := projects.GetProject(ctx, d.ProjectID)
		p.Metadata = registeredReleaseMetadata(t)
		_ = projects.UpdateProject(ctx, p)
		s.cfg.SourceRepository = "example/cuan"
		s.publicationObserver = func(context.Context, publication.Config) (string, error) { return strings.Repeat("b", 40), nil }
	}
	if scenario == "cancelled" {
		source.CancelRequested = true
	}
	if scenario == "foreign owner" {
		origin.OwnerID = "other"
	}
	if scenario == "different repository" {
		s.cfg.SourceRepository = "other/repo"
	}
	if scenario == "checks forbidden" {
		d.Instructions += "\nDo not run the test suite."
	}
	if err := store.UpdateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAttempt(ctx, source); err != nil {
		t.Fatal(err)
	}
	if !currentOutcome || publicationFailure || scenario == "current outcome reported checks" || (unpublished && scenario != "current outcome unpublished missing source report") {
		if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "live-source-report", TaskID: d.ID, AttemptID: &source.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusDone, Message: "Implementation report", CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "live-source-kind", TaskID: d.ID, AttemptID: &source.ID, ActivityType: "agent_result_kind", NewStatus: "implementation", Message: "implementation", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if unpublished && scenario != "current outcome unpublished missing audit" && !strings.HasPrefix(scenario, "current outcome unpublished valid registered review") {
		if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "publication-audit:" + source.ID, TaskID: d.ID, AttemptID: &source.ID, ActivityType: task.ActivityPublicationAcceptanceAudit, Author: "coordinator", NewStatus: "review_required", CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	if publicationFailure {
		record, _ := json.Marshal(task.RunnerVerificationRecord{Summary: "Project verification: test.sh --fast (exit 0)\nNative source checks passed", Changes: task.NewRunnerChangeRecord(d.CommitSHA, "src/session.tsx | 2 +-")})
		if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "verification:" + source.ID, TaskID: d.ID, AttemptID: &source.ID, ActivityType: task.ActivityTypeRunnerVerification, Author: source.RunnerID, Message: string(record), CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
		d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
		if _, err := s.saveReviewAssessment(ctx, d, origin, inboxReviewEvaluation{Decision: "revise", RemainingWork: "verification", Finding: "Existing release requires inspection", Explanation: "Check the approved current outcome", Revision: "Inspect existing release without deployment"}); err != nil {
			t.Fatal(err)
		}
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	if blockedPreparation {
		assessment := inboxReviewEvaluation{Decision: "revise", RemainingWork: "verification", Finding: "Inspect existing release", Explanation: "Original outcome needs verification", Revision: "Read-only current-outcome inspection"}
		if _, err := s.saveReviewAssessment(ctx, d, origin, assessment); err != nil {
			t.Fatal(err)
		}
		d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	}
	if strings.HasPrefix(scenario, "current outcome unpublished valid supplemental ") {
		d = supplementalInspectionFixture(t, s, origin, d, database)
		switch strings.TrimPrefix(scenario, "current outcome unpublished valid supplemental ") {
		case "missing receipt":
			for _, event := range d.Activities {
				if event.AttemptID != nil && *event.AttemptID == d.LatestAttempt.ID && event.ActivityType == "verification_source_verified" {
					event.Author = "agent"
				}
			}
		case "cancelled verifier":
			d.LatestAttempt.CancelRequested = true
		case "wrong source":
			d.CommitSHA = strings.Repeat("e", 40)
		}
		if !strings.HasSuffix(scenario, "supplemental valid") && !staleSupplemental && !strings.HasSuffix(scenario, "valid publication review") {
			if s.recoverLiveResultInspection(ctx, d, origin) {
				t.Fatal("untrusted supplemental evidence granted inspection", scenario)
			}
			return
		}
		if strings.HasSuffix(scenario, "valid publication review") {
			if s.verificationReviewContext(ctx, d) == nil {
				t.Fatal("missing independent source context")
			}
			decision := s.requirePublicationReview(ctx, d, inboxReviewEvaluation{Decision: "revise", SourceReview: "pass", RemainingWork: "publication", Finding: "Account works; incidental tests not deployed", Explanation: "Check live result", Revision: "Publish incidental test repairs"})
			if decision.RemainingWork != "verification" {
				t.Fatal("legacy source approval manufactured publication stage", decision)
			}
			if _, err := s.saveReviewAssessment(ctx, d, origin, decision); err != nil {
				t.Fatal(err)
			}
			d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
		}
		scenario = "current outcome unpublished valid"
	}
	if strings.HasPrefix(scenario, "current outcome unpublished valid registered review") {
		decision := s.requirePublicationReview(ctx, d, inboxReviewEvaluation{Decision: "revise", RemainingWork: "implementation", Finding: "Existing release needs inspection", Explanation: "Inspect current outcome", Revision: "Verify existing release"})
		if decision.RemainingWork != "verification" {
			t.Fatal("unverified publication bypassed current-outcome inspection", decision)
		}
		_, err := s.saveReviewAssessment(ctx, d, origin, decision)
		if err != nil {
			t.Fatal(err)
		}
		d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
		if strings.HasSuffix(scenario, " handoff") {
			foreign := origin
			foreign.OwnerID = "another-owner"
			if s.startReviewedLiveInspection(ctx, foreign, d, decision) {
				t.Fatal("review handoff crossed owner boundary")
			}
			changed := decision
			changed.Revision += " changed"
			if s.startReviewedLiveInspection(ctx, origin, d, changed) {
				t.Fatal("review handoff ignored saved assessment")
			}
			if !s.startReviewedLiveInspection(ctx, origin, d, decision) {
				t.Fatal("review fast path bypassed native outcome inspection")
			}
			launch, err := s.taskSvc.LatestLaunch(ctx, d.ID)
			if err != nil || launch == nil {
				t.Fatal("review did not persist inspection", err)
			}
			proof, err := task.LiveInspectionForLaunch(&d.Task, launch)
			if err != nil || proof == nil || proof.Purpose != task.CurrentOutcomeInspection || proof.TargetCommit != strings.Repeat("b", 40) || proof.SourceCommit != d.CommitSHA {
				t.Fatal("review lost exact current-outcome intent", err)
			}
			if !s.startReviewedLiveInspection(ctx, origin, d, decision) {
				t.Fatal("review handoff did not deduplicate")
			}
			return
		}
	}
	eligible := scenario != "current outcome opaque foreign" && scenario != "current outcome opaque unfinished" && scenario != "current outcome opaque cancelled" && scenario != "current outcome checks missing report" && scenario != "cancelled" && scenario != "foreign owner" && scenario != "different repository" && scenario != "checks forbidden"
	if scenario == "current outcome local only" || scenario == "current outcome local branch only" || scenario == "committed local branch only" {
		eligible = false
	}
	if strings.HasPrefix(scenario, "current outcome unrecorded ") && !strings.HasPrefix(scenario, "current outcome unrecorded local") {
		eligible = false
	}
	if unpublished && !strings.HasPrefix(scenario, "current outcome unpublished valid") {
		eligible = false
	}
	if scenario == "current outcome unpublished valid" {
		before, _ := store.GetTask(ctx, d.ID)
		previousLaunch, _ := s.taskSvc.LatestLaunch(ctx, d.ID)
		attempts, _ := store.ListAttempts(ctx, d.ID)
		previousService := s.taskSvc
		if memory, ok := store.(*task.MemoryStore); ok {
			s.taskSvc = task.NewService(inspectionReadGuard{MemoryStore: memory}, s.projectSvc)
		}
		for range 2 {
			plan := s.liveInspectionPlanDiagnostic(ctx, d)
			if plan.Code != "ready" || plan.SourceAttemptID != source.ID || plan.TargetCommit != s.cfg.CommitSHA || plan.SupplementalSourceVerified != (d.LatestAttempt.ID != source.ID) {
				t.Fatal("read-only diagnostic diverged from native plan", plan)
			}
		}
		s.taskSvc = previousService
		after, _ := store.GetTask(ctx, d.ID)
		latest, _ := s.taskSvc.LatestLaunch(ctx, d.ID)
		afterAttempts, _ := store.ListAttempts(ctx, d.ID)
		if !after.UpdatedAt.Equal(before.UpdatedAt) || len(afterAttempts) != len(attempts) || ((latest == nil) != (previousLaunch == nil)) || (latest != nil && latest.ID != previousLaunch.ID) {
			t.Fatal("diagnostic admitted or changed work")
		}
	}
	if staleSupplemental {
		proof, code := s.planLiveResultInspection(ctx, d, origin, source)
		if code != "ready" || proof.SourceVerifierAttemptID == "" {
			t.Fatal("missing valid supplemental plan", code)
		}
		legacy := *proof
		legacy.SourceVerifierAttemptID = ""
		raw, _ := json.Marshal(legacy)
		parent, e := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: d.ID, ProjectID: d.ProjectID, RequestID: legacy.RequestID(d.ID), OwnerID: origin.OwnerID, RoomID: origin.RoomID, SourceRequestID: origin.RequestID, AgentID: d.AssignedAgent, Revision: task.LiveInspectionMarker + string(raw), InstructionsHash: legacy.InstructionsHash, CreatedAt: time.Now(), AvailableAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)})
		if e != nil {
			t.Fatal(e)
		}
		_ = store.DeleteTask(ctx, "phase-blocker")
		if claimed, e := store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "legacy-preparation", parent.ID); e != nil || claimed == nil {
			t.Fatal(e)
		}
		if e = s.taskSvc.FinishLaunch(ctx, parent.ID, "legacy-preparation", "stopped", time.Now(), staleInspectionLaunchReason); e != nil {
			t.Fatal(e)
		}
		if e := store.CreateTask(ctx, &task.Task{ID: "phase-blocker", ProjectID: d.ProjectID, Title: "Other work", Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); e != nil {
			t.Fatal(e)
		}
	}
	if boundSource, _ := s.verifiedSupplementalSource(ctx, d); boundSource != nil && eligible {
		// Exercise the coordinator's production entry, including all earlier
		// review/finalization/recovery gates, before testing deduplication.
		s.deliverInboxTaskUpdates(ctx)
		plannedID := (&task.LiveInspectionIntent{Purpose: task.CurrentOutcomeInspection, SourceAttemptID: source.ID, InstructionsHash: task.LaunchInstructionsHash(d.Instructions)}).RequestID(d.ID)
		if admitted, e := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, plannedID)); e != nil || admitted == nil {
			t.Fatal("coordinator did not admit planned supplemental inspection", e)
		}
	}
	if got := s.recoverLiveResultInspection(ctx, d, origin); got != eligible {
		t.Fatal("incorrect live inspection eligibility", got, scenario)
	}
	if !eligible {
		return
	}
	id := task.LiveInspectionRequestID(d.ID, d.CommitSHA, task.LaunchInstructionsHash(d.Instructions))
	if currentOutcome {
		id = (&task.LiveInspectionIntent{Purpose: task.CurrentOutcomeInspection, SourceAttemptID: source.ID, InstructionsHash: task.LaunchInstructionsHash(d.Instructions)}).RequestID(d.ID)
	}
	if staleSupplemental {
		latest, e := s.taskSvc.LatestLaunch(ctx, d.ID)
		if e != nil || latest == nil {
			t.Fatal("missing repaired admission", e)
		}
		id = latest.RequestID
		binding, e := task.LiveInspectionForLaunch(&d.Task, latest)
		if e != nil || binding == nil || binding.PreparationLaunchID == "" || binding.SourceVerifierAttemptID != d.LatestAttempt.ID {
			t.Fatal("lost legacy preparation/verifier binding", e)
		}
		parent, e := s.taskSvc.GetLaunch(ctx, binding.PreparationLaunchID)
		if e != nil || parent.State != "stopped" || !latest.ExpiresAt.Equal(parent.ExpiresAt) {
			t.Fatal("repair renewed or overwrote original grant", e)
		}
	}
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, id))
	if err != nil || launch.State != "queued" || launch.SourceRequestID != origin.RequestID {
		t.Fatal("missing scope-bound durable inspection", err)
	}
	if !s.recoverLiveResultInspection(ctx, d, origin) {
		t.Fatal("inspection did not deduplicate")
	}
	_ = store.DeleteTask(ctx, "phase-blocker")
	if scenario == "current outcome unpublished valid preparation" || scenario == "current outcome unpublished valid scope preparation" || blockedPreparation {
		parent := launch
		owned, err := store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "preparing", parent.ID)
		if err != nil || owned == nil {
			t.Fatal("could not reserve preparation", err)
		}
		reason := task.InspectionPublicationPreparationError
		if strings.Contains(scenario, "scope preparation") || blockedPreparation {
			reason = task.InspectionScopePreparationError
		}
		if err := store.(task.LaunchStore).FinishLaunch(ctx, parent.ID, owned.LeaseToken, "stopped", time.Now(), reason); err != nil {
			t.Fatal(err)
		}
		if blockedPreparation {
			opinion := origin
			opinion.RequestID = db.GenerateUUID()
			assessment := inboxReviewEvaluation{Decision: "blocked", RemainingWork: "external", Finding: "Worker cannot deploy", Explanation: "Model asks for owner deployment approval", ManagerAction: "needs_owner", ManagerReason: "worker deny_deploy"}
			if _, err := s.saveReviewAssessment(ctx, d, opinion, assessment); err != nil {
				t.Fatal(err)
			}
			switch strings.TrimPrefix(scenario, "current outcome unpublished valid blocked preparation ") {
			case "cancelled source":
				source.CancelRequested = true
				if err := store.UpdateAttempt(ctx, source); err != nil {
					t.Fatal(err)
				}
			case "changed instructions":
				row, _ := store.GetTask(ctx, d.ID)
				row.Instructions += "\nChanged approved scope"
				if err := store.UpdateTask(ctx, row); err != nil {
					t.Fatal(err)
				}
			case "foreign owner":
				origin.OwnerID = "another-owner"
			}
			d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
			if !strings.HasSuffix(scenario, "preparation valid") {
				if s.recoverLiveResultInspection(ctx, d, origin) {
					t.Fatal("original admission bypassed changed authority", scenario)
				}
				return
			}
		}
		if err := store.CreateTask(ctx, &task.Task{ID: "phase-blocker", ProjectID: d.ProjectID, Title: "Other work", Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
			t.Fatal(err)
		}
		s.cfg.CommitSHA = strings.Repeat("e", 40)
		candidate, parseErr := task.LiveInspectionForLaunch(&d.Task, parent)
		if parseErr != nil || candidate == nil {
			t.Fatal("parse parent", parseErr)
		}
		candidate.PreparationLaunchID = parent.ID
		candidate.TargetCommit = s.cfg.CommitSHA
		candidate.ObservedAt = time.Now().UTC()
		if err := task.ValidateLiveInspectionSource(ctx, store, &d.Task, candidate); err != nil {
			t.Fatal("candidate source", err)
		}
		if !s.recoverLiveResultInspection(ctx, d, origin) {
			latest, _ := s.taskSvc.LatestLaunch(ctx, d.ID)
			t.Fatalf("unstarted preparation was not recovered: status=%s latest=%+v", d.Status, latest)
		}
		replacement := &task.LiveInspectionIntent{PreparationLaunchID: parent.ID, InstructionsHash: parent.InstructionsHash}
		id = replacement.RequestID(d.ID)
		launch, err = s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, id))
		if err != nil || launch == nil || !launch.ExpiresAt.Equal(parent.ExpiresAt) {
			t.Fatal("preparation extended its authorization deadline", err)
		}
		proof, err := task.LiveInspectionForLaunch(&d.Task, launch)
		if err != nil || proof == nil || proof.PreparationLaunchID != parent.ID || proof.TargetCommit != s.cfg.CommitSHA {
			t.Fatal("replacement lost lineage/current target", err)
		}
		retained, _ := s.taskSvc.GetLaunch(ctx, parent.ID)
		if retained.State != "stopped" || retained.Revision != parent.Revision {
			t.Fatal("historical preparation was overwritten")
		}
		if !s.recoverLiveResultInspection(ctx, d, origin) && !blockedPreparation {
			t.Fatal("replacement did not deduplicate")
		}
		if blockedPreparation {
			latest, _ := s.taskSvc.LatestLaunch(ctx, d.ID)
			attempts, _ := store.ListAttempts(ctx, d.ID)
			if latest == nil || latest.ID != launch.ID || latest.State != "queued" || len(attempts) != len(d.Attempts) {
				t.Fatal("original grant repair duplicated admission or execution")
			}
		}
		_ = store.DeleteTask(ctx, "phase-blocker")
		scenario = "current outcome unpublished valid"
	}
	claimed, err := store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "live-launch", launch.ID)
	if err != nil || claimed == nil {
		t.Fatal("could not claim the existing launch", err)
	}
	bound, err := task.LiveInspectionForLaunch(&d.Task, launch)
	if err != nil || bound == nil {
		t.Fatal("inspection admission binding", err)
	}
	if bound.SourceVerifierAttemptID != "" {
		if bound.SourceAttemptID == bound.SourceVerifierAttemptID || bound.PredecessorAttemptID() != d.LatestAttempt.ID {
			t.Fatal("verifier replaced implementation identity or lost predecessor")
		}
		for _, mutate := range []func(*task.LiveInspectionIntent){
			func(p *task.LiveInspectionIntent) { p.SourceVerifierAttemptID = p.SourceAttemptID },
			func(p *task.LiveInspectionIntent) { p.SourceVerifierAttemptID = "missing-verifier" },
			func(p *task.LiveInspectionIntent) { p.OwnerID = "foreign-owner" },
			func(p *task.LiveInspectionIntent) { p.SourceCommit = strings.Repeat("e", 40) },
		} {
			bad := *bound
			mutate(&bad)
			if task.ValidateLiveInspectionSource(ctx, store, &d.Task, &bad) == nil {
				t.Fatal("forged source verifier accepted")
			}
		}
	}
	if d.LatestAttempt != nil && d.LatestAttempt.Status == task.AttemptStatusSucceeded {
		guarded := store.(interface {
			CreateAttemptAfter(context.Context, *task.TaskAttempt, string) error
		})
		if err := guarded.CreateAttemptAfter(ctx, &task.TaskAttempt{ID: "untrusted-success-retry", TaskID: d.ID, RunnerID: "cuan-runner", AgentName: d.AssignedAgent, Status: task.AttemptStatusQueued, CreatedAt: time.Now()}, d.LatestAttempt.ID); err == nil {
			t.Fatal("successful predecessor allowed an ordinary retry")
		}
	}
	inspectionAttempt, err := s.taskSvc.CreateAttempt(ctx, d.ID, origin.OwnerID, task.CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: d.AssignedAgent, RequestID: id, ExpectedPredecessorID: bound.PredecessorAttemptID()})
	if err != nil {
		t.Fatal(err)
	}
	var workerStore runner.Store = runner.NewMemoryStore(store)
	if database != nil {
		workerStore = runner.NewDBStore(database)
	}
	service := runner.NewService(workerStore, store, s.projectSvc)
	service.SetEventBus(runner.NewEventBus())
	oldWorker, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "live-runner"}, time.Minute)
	if err != nil || oldWorker.Claimed {
		t.Fatal("unsupported worker consumed inspection", err)
	}
	queued, _ := store.GetLatestAttempt(ctx, d.ID)
	if queued.Status != task.AttemptStatusQueued {
		t.Fatal("unsupported worker changed the queued attempt")
	}
	if currentOutcome {
		old, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "live-runner", SupportsNativeVisualVerification: true, SupportsLiveInspection: true, SupportsLiveInspectionRecovery: true}, time.Minute)
		if err != nil || old.Claimed {
			t.Fatal("older inspection worker reserved current outcome", err)
		}
	}
	unsupported, oldErr := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "live-runner", SupportsLiveInspection: true, SupportsLiveInspectionRecovery: true, SupportsCurrentOutcomeInspection: true}, time.Minute)
	if oldErr != nil || unsupported.Claimed {
		t.Fatal("worker without native visual transport reserved inspection", oldErr)
	}
	claim, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "live-runner", SupportsNativeVisualVerification: true, SupportsLiveInspection: true, SupportsCurrentOutcomeInspection: true}, time.Minute)
	if err != nil || !claim.Claimed || claim.LiveInspection == nil || !claim.PublicationLimits.ReadOnlyRepository || claim.Environment != "" || claim.AutoDeploy {
		t.Fatal("inspection lost native routing/scope/environment restrictions", err)
	}
	info, infoErr := s.taskSvc.ExecutionInfo(ctx, d.ID)
	if infoErr != nil || !info.IntentParsed || info.AttemptID != claim.Attempt.ID || info.SourceAttemptID != claim.LiveInspection.SourceAttemptID || info.TargetCommit != claim.LiveInspection.TargetCommit || !info.Scope.ReadOnlyRepository {
		t.Fatal("execution inspection lost exact saved intent", infoErr)
	}
	if strings.HasPrefix(scenario, "current outcome unpublished valid infrastructure ") {
		testInspectionInfrastructureRecovery(t, s, origin, d, claim, service, strings.TrimPrefix(scenario, "current outcome unpublished valid infrastructure "))
		return
	}
	if strings.HasPrefix(scenario, "current outcome unpublished valid capacity") {
		testInspectionCapacityContinuation(t, s, origin, d, claim, service, strings.HasSuffix(scenario, "long context"))
		return
	}
	if scenario == "current outcome unpublished valid registered review" && claim.PHPUnitTargets != `["tests/Feature/ExampleTest.php"]` {
		t.Fatal("registered nonsecret test targets lost", claim.PHPUnitTargets)
	}
	var nativeReceipt *task.VisualVerification
	nativeScenario := ""
	if strings.HasPrefix(scenario, "current outcome unpublished valid native ") {
		nativeScenario = strings.TrimPrefix(scenario, "current outcome unpublished valid native ")
		nativeReceipt = nativeVisualLifecycleRecord(t, s, d, claim, nativeScenario)
		if nativeScenario == "valid" || nativeScenario == "corrupted delivery" {
			scenario = "current outcome unpublished valid"
		} else {
			scenario = "native rejected"
		}
	}
	if claim.Attempt.ExecutionDeadline == nil || claim.Attempt.StartedAt == nil || claim.Attempt.ExecutionDeadline.Sub(*claim.Attempt.StartedAt) > 10*time.Minute {
		t.Fatal("stored inspection deadline diverges from native verification budget")
	}
	if scenario != "current outcome timeout files" {
		if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "live-current-report", TaskID: d.ID, AttemptID: &inspectionAttempt.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Message: "Current feature verified", CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	if scenario == "current outcome failed checks revalidation" {
		if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "live-current-kind", TaskID: d.ID, AttemptID: &inspectionAttempt.ID, ActivityType: "agent_result_kind", NewStatus: "implementation", CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	req := runner.CompleteAttemptRequest{RunnerID: "live-runner", Status: task.AttemptStatusSucceeded, ExitCode: 0, LiveInspectionCommit: s.cfg.CommitSHA, SourceAncestryVerified: true, AgentReportRequired: true, AgentReportConfirmed: true, TestSummary: "Project verification: test.sh --fast (exit 0)\nFresh checks", RepositoryVerification: &runner.RepositoryVerification{Baseline: strings.Repeat("c", 64), Final: strings.Repeat("c", 64)}}
	req.VisualVerification = nativeReceipt
	if currentOutcome {
		req.SourceAncestryVerified = false
		req.CurrentOutcomeVerified = true
	}
	switch scenario {
	case "current outcome failed checks revalidation":
		req.Status, req.ErrorMessage = task.AttemptStatusFailed, "Live result inspection project checks did not pass"
		req.TestSummary = "Project verification: test.sh --fast (exit 1)\n--- FAIL: TestOriginalCriterion (0.00s)\n    criterion_test.go:19: assertion failed\nFAIL\tgithub.com/fixture/repo/internal/runner\t1.2s\nFAIL\n"
	case "current outcome timeout files":
		req.Status, req.ExitCode, req.ErrorMessage = task.AttemptStatusFailed, 124, "Execution timed out after adaptive timeout (10m0s)"
		req.AgentReportConfirmed, req.CurrentOutcomeVerified = false, false
		req.LiveInspectionCommit, req.TestSummary, req.RepositoryVerification = "", "", nil
	case "current outcome missing identity", "current outcome unrecorded local missing identity", "current outcome unpublished valid missing identity":
		req.CurrentOutcomeVerified = false
	case "current outcome forged ancestry", "current outcome unpublished valid forged ancestry":
		req.SourceAncestryVerified = true
	case "current outcome unpublished valid verifier repair":
		req.Status, req.ErrorMessage = task.AttemptStatusFailed, "Live result inspection project checks did not pass"
		req.TestSummary = "Project verification: php vendor/bin/phpunit --colors=never tests/Feature/McpTest.php (exit 1)\nPest\\Exceptions\\InvalidPestCommand \n\n  Please run [./vendor/bin/pest] instead.\n"
		if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "current-kind", TaskID: d.ID, AttemptID: &inspectionAttempt.ID, ActivityType: "agent_result_kind", NewStatus: "implementation", CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	case "recover checks":
		req.Status, req.ErrorMessage = task.AttemptStatusFailed, "Live result inspection project checks did not pass"
		req.TestSummary = "Project verification: test.sh --fast (exit 1)\nCheck timed out"
	case "wrong commit":
		req.LiveInspectionCommit = strings.Repeat("d", 40)
	case "missing ancestry":
		req.SourceAncestryVerified = false
	case "failed checks", "current outcome unrecorded local failed checks", "current outcome unpublished valid failed checks":
		req.TestSummary = "Project verification: test.sh --fast (exit 1)\nFailed"
	case "changed repository", "current outcome unrecorded local changed repository", "current outcome unpublished valid changed repository":
		req.RepositoryVerification.Final = strings.Repeat("d", 64)
	case "missing report", "current outcome unrecorded local missing report", "current outcome unpublished valid missing report":
		req.AgentReportConfirmed = false
	}
	completed, err := service.Complete(ctx, inspectionAttempt.ID, req)
	valid := (publicationFailure && scenario != "current outcome failed checks revalidation") || scenario == "current outcome unpublished valid registered review" || scenario == "current outcome unpublished valid" || scenario == "current outcome unpublished valid release changed" || scenario == "valid" || scenario == "release changed" || scenario == "current outcome" || scenario == "current outcome release changed" || scenario == "current outcome reported checks" || scenario == "current outcome opaque local" || scenario == "current outcome unrecorded local" || scenario == "current outcome unrecorded local release changed" || scenario == "current outcome revalidation"
	if err != nil || (completed.Status == task.AttemptStatusSucceeded) != valid {
		t.Fatal("invalid live evidence was accepted", completed, err)
	}
	after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if evidenceFollowup {
		testEvidenceOutcomeRevalidation(t, s, origin, after, service)
		return
	}
	if scenario == "current outcome unpublished valid verifier repair" {
		_ = store.(task.LaunchStore).FinishLaunch(ctx, launch.ID, claimed.LeaseToken, "stopped", time.Now(), "Execution finished")
		status := task.TaskStatusBlocked
		_, _ = s.taskSvc.UpdateTask(ctx, d.ID, "coordinator", task.UpdateTaskRequest{Status: &status})
		after, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
		testNativeVerifierRepair(t, s, origin, after)
		return
	}
	old, _ := store.GetAttempt(ctx, source.ID)
	if old.Status != source.Status || after.Status == task.TaskStatusDone || after.CommitSHA != d.CommitSHA {
		t.Fatal("inspection rewrote source history or bypassed independent acceptance")
	}
	if strings.HasPrefix(scenario, "current outcome unrecorded ") {
		if old.ExitCode != nil || old.FinishedAt != nil || old.ErrorMessage != "" {
			t.Fatal("inspection invented historical termination metadata")
		}
	}
	if scenario == "current outcome unrecorded local" {
		observed := s.liveInspectionReviewContext(ctx, after)
		if observed == nil || observed["source_termination_metadata_missing"] != true || observed["source_ancestry_verified"] != nil {
			t.Fatal("legacy termination uncertainty was lost", observed)
		}
	}
	if scenario == "release changed" || scenario == "current outcome release changed" || scenario == "current outcome unrecorded local release changed" || scenario == "current outcome unpublished valid release changed" {
		s.cfg.CommitSHA = strings.Repeat("d", 40)
	}
	if got := s.liveInspectionReviewContext(ctx, after); (got != nil) != ((publicationFailure && scenario != "current outcome failed checks revalidation") || scenario == "current outcome unpublished valid registered review" || scenario == "current outcome unpublished valid" || scenario == "valid" || scenario == "current outcome" || scenario == "current outcome reported checks" || scenario == "current outcome opaque local" || scenario == "current outcome unrecorded local" || scenario == "current outcome revalidation") {
		t.Fatal("review used absent/stale/failed result evidence", got)
	}
	if nativeScenario != "" && nativeScenario != "corrupted delivery" {
		verdict := s.requireVisualReview(ctx, after, inboxReviewEvaluation{Decision: "pass", VisualInspection: "Protocol fixture inspected"})
		if (verdict.Decision == "pass") != (nativeScenario == "valid" || nativeScenario == "changed receipt") {
			t.Fatal("native evidence gate accepted missing/foreign proof", verdict)
		}
	}
	if scenario == "current outcome unpublished valid" {
		observed := s.liveInspectionReviewContext(ctx, after)
		if observed == nil || observed["source_unpublished_result"] != true || observed["unpublished_source_commit"] != d.CommitSHA || observed["source_execution_state"] != task.AttemptStatusSucceeded || observed["source_ancestry_verified"] != nil {
			t.Fatal("unpublished branch was credited with inspected publication", observed)
		}
		publication := s.inboxReviewPublication(ctx, after)
		if publication == nil || !publication.Verified || publication.Commit != s.cfg.CommitSHA {
			t.Fatal("publication observation confused source branch and inspected revision", publication)
		}
	}
	if scenario == "current outcome revalidation" || scenario == "current outcome failed checks revalidation" {
		testChangedOutcomeRevalidation(t, s, origin, after, service, revalidationCommits)
	}
	if scenario == "current outcome" {
		proof := s.liveInspectionReviewContext(ctx, after)
		if proof["source_ancestry_verified"] != nil || proof["source_commit"] != nil || proof["current_outcome_inspected"] != true {
			t.Fatal("current outcome invented historical source ancestry")
		}
	}
	if currentOutcome && !valid {
		stopped := *after
		stopped.Status = task.TaskStatusBlocked
		s.cfg.CommitSHA = strings.Repeat("e", 40)
		if s.recoverLiveResultInspection(ctx, &stopped, origin) {
			t.Fatal("failed current outcome inspection gained another attempt after deploy")
		}
	}
	if scenario == "valid" {
		limited := *after
		attempt := *after.LatestAttempt
		attempt.Status, attempt.ErrorMessage = task.AttemptStatusFailed, "The AI provider reported a usage limit. The run was stopped; saved work preserved"
		limited.Status, limited.LatestAttempt, limited.Attempts = task.TaskStatusBlocked, &attempt, nil
		if !providerLimitedInboxTaskCanResume(&limited) {
			t.Fatal("fixture did not reach generic provider recovery eligibility")
		}
		if s.recoverInterruptedInboxTask(ctx, &limited, origin) {
			t.Fatal("live inspection replayed as implementation after provider limit")
		}
		ordinaryID := delegationMessageID("task-provider-recovery:" + attempt.ID)
		if launch, _ := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, ordinaryID)); launch != nil {
			t.Fatal("live inspection created an ordinary execution launch")
		}
	}
	if scenario == "recover checks" {
		if err := store.(task.LaunchStore).FinishLaunch(ctx, launch.ID, claimed.LeaseToken, "stopped", time.Now(), "Execution finished"); err != nil {
			t.Fatal(err)
		}
		after, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
		if s.recoverLiveResultInspection(ctx, after, origin) {
			t.Fatal("same failed release was retried")
		}
		s.cfg.CommitSHA = strings.Repeat("c", 40)
		if err := store.CreateTask(ctx, &task.Task{ID: "recovery-blocker", ProjectID: d.ProjectID, Title: "Other work", Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
			t.Fatal(err)
		}
		if !s.recoverLiveResultInspection(ctx, after, origin) {
			t.Fatalf("changed running result did not recover failed checks: kind=%q gaps=%v report=%+v attempt=%+v", after.ResultKind, task.AcceptanceGaps(&after.Task), inboxReportEvidence(after), after.LatestAttempt)
		}
		proof := &task.LiveInspectionIntent{SourceCommit: d.CommitSHA, InstructionsHash: task.LaunchInstructionsHash(d.Instructions), PreviousAttemptID: inspectionAttempt.ID}
		recoveryID := proof.RequestID(d.ID)
		recovery, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, recoveryID))
		if err != nil || recovery == nil {
			t.Fatal("missing durable check recovery", err)
		}
		if err := store.DeleteTask(ctx, "recovery-blocker"); err != nil {
			t.Fatal(err)
		}
		binding, err := task.LiveInspectionForLaunch(&after.Task, recovery)
		if err != nil || binding.PreviousAttemptID != inspectionAttempt.ID || binding.PreviousCommit != strings.Repeat("b", 40) || binding.SourceAttemptID != source.ID {
			t.Fatal("recovery lost source and predecessor binding", err)
		}
		for _, mutate := range []func(*task.LiveInspectionIntent){func(p *task.LiveInspectionIntent) { p.PreviousCommit = strings.Repeat("d", 40) }, func(p *task.LiveInspectionIntent) { p.OwnerID = "foreign" }, func(p *task.LiveInspectionIntent) { p.PreviousAttemptID = source.ID }} {
			bad := *binding
			mutate(&bad)
			if task.ValidateLiveInspectionSource(ctx, store, &after.Task, &bad) == nil {
				t.Fatal("forged predecessor accepted")
			}
		}
		secondLaunch, err := store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "recovery-launch", recovery.ID)
		if err != nil {
			t.Fatal(err)
		}
		second, err := s.taskSvc.CreateAttempt(ctx, d.ID, origin.OwnerID, task.CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: d.AssignedAgent, RequestID: recoveryID, ExpectedPredecessorID: binding.PredecessorAttemptID()})
		if err != nil {
			t.Fatal(err)
		}
		old, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "live-runner", SupportsNativeVisualVerification: true, SupportsLiveInspection: true}, time.Minute)
		if err != nil || old.Claimed {
			t.Fatal("old inspection worker consumed recovery", err)
		}
		current, _ := store.GetLatestAttempt(ctx, d.ID)
		if current.Status != task.AttemptStatusQueued {
			t.Fatal("old worker reserved recovery")
		}
		claimed, err := service.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "live-runner", SupportsNativeVisualVerification: true, SupportsLiveInspection: true, SupportsLiveInspectionRecovery: true}, time.Minute)
		if err != nil || !claimed.Claimed || claimed.LiveInspection == nil {
			t.Fatal("recovery was not claimable", err)
		}
		if err := store.CreateActivity(ctx, &task.TaskActivity{ID: "recovery-report", TaskID: d.ID, AttemptID: &second.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Message: "Second inspection complete", CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Complete(ctx, second.ID, req); err != nil {
			t.Fatal(err)
		}
		if err := store.(task.LaunchStore).FinishLaunch(ctx, recovery.ID, secondLaunch.LeaseToken, "stopped", time.Now(), "Execution finished"); err != nil {
			t.Fatal(err)
		}
		exhausted, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
		s.cfg.CommitSHA = strings.Repeat("d", 40)
		if s.recoverLiveResultInspection(ctx, exhausted, origin) {
			t.Fatal("another deploy reset recovery budget")
		}
	}
	if scenario == "release changed" {
		key := "task-review:" + inspectionAttempt.ID
		message := &db.ProjectMessage{ID: delegationMessageID(key + ":input"), RoomID: origin.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: "Review [/tasks/" + d.ID + "]", CreatedAt: time.Now().UTC()}
		if err := s.projectStudioRepo.CreateMessage(ctx, message); err != nil {
			t.Fatal(err)
		}
		q := db.InboxRequest{OwnerID: origin.OwnerID, AgentID: origin.AgentID, RoomID: origin.RoomID, RequestID: delegationMessageID(key), MessageID: message.ID, ReplyID: delegationMessageID(key + ":reply"), CreatedAt: time.Now().UTC(), Deadline: time.Now().Add(time.Hour)}
		s.applyInboxReviewEvaluation(ctx, q, `{"decision":"pass","finding":"All criteria verified.","explanation":"Accept result","revision":""}`)
		current, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
		if err != nil || current.Status == task.TaskStatusDone {
			t.Fatal("model pass bypassed changed running release", err)
		}
	}
}
