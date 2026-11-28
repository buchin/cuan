package server

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestPublicationReviewCannotAcceptUnpublishedResult(t *testing.T) {
	for _, scenario := range []string{"verification revision", "implementation revision", "open PR", "report claim", "wrong receipt", "verified receipt", "unrelated running build", "exact project build", "foreign project build", "owner forbids deployment"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			s, _, _, _ := inboxFixture(t)
			projects := project.NewMemoryStore()
			repo := "https://github.com/buchin/cuan"
			if scenario == "foreign project build" {
				repo = "https://github.com/buchin/other"
			}
			if err := projects.CreateProject(ctx, &project.Project{ID: "publication-project", Name: "Project", RepoURL: repo, Metadata: registeredReleaseMetadata(t)}); err != nil {
				t.Fatal(err)
			}
			s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
			commit := strings.Repeat("c", 40)
			now := time.Now().UTC()
			d := &task.TaskDetailResponse{Task: task.Task{ID: "publication-task", ProjectID: "publication-project", Title: "Implement and deploy", CommitSHA: commit, ExecutionCategory: task.ExecutionCategoryCode, PRNumber: 24, Instructions: "Implement and deploy the approved feature.", CompleteNote: "Implementation tests passed."}, ResultKind: "implementation", LatestAttempt: &task.TaskAttempt{ID: "publication-attempt", TaskID: "publication-task", Status: task.AttemptStatusSucceeded, CreatedAt: now.Add(-time.Minute)}}
			if scenario == "report claim" {
				d.CompleteNote = "The deployment is live and verified. All criteria passed."
			}
			if scenario == "verified receipt" || scenario == "wrong receipt" {
				receiptCommit := commit
				if scenario == "wrong receipt" {
					receiptCommit = strings.Repeat("d", 40)
				}
				raw, _ := json.Marshal(task.DeploymentReceipt{Version: 1, TaskID: d.ID, AttemptID: d.LatestAttempt.ID, CommitSHA: receiptCommit, ReleaseSHA: receiptCommit, Environment: "production", Health: "healthy"})
				d.Activities = []*task.TaskActivity{{ID: task.DeploymentReceiptID(d.ID, d.LatestAttempt.ID, receiptCommit), TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: "deployment_verified", Author: "cuan-deploy", Message: string(raw), CreatedAt: now.Add(-time.Second)}}
			}
			if strings.Contains(scenario, "build") {
				running := commit
				if scenario == "unrelated running build" {
					running = strings.Repeat("d", 40)
				}
				s.cfg = &config.Config{Environment: "production", SourceRepository: "buchin/cuan", CommitSHA: running}
			}
			if scenario == "owner forbids deployment" {
				d.Instructions = "Do not deploy. Return verified local changes."
			}
			if err := s.taskSvc.Store().CreateTask(ctx, &d.Task); err != nil {
				t.Fatal(err)
			}
			if err := s.taskSvc.Store().CreateAttempt(ctx, d.LatestAttempt); err != nil {
				t.Fatal(err)
			}
			input := inboxReviewEvaluation{Decision: "pass", Finding: "Everything passed", RemainingWork: "none"}
			if strings.HasSuffix(scenario, "revision") {
				input = inboxReviewEvaluation{Decision: "revise", RemainingWork: strings.Fields(scenario)[0], Finding: "Deployment did not happen", Explanation: "Deploy again to obtain a receipt", Revision: "Check the approved feature", VisualInspection: "Observed existing view"}
			}
			result := s.requirePublicationReview(ctx, d, input)
			if observed := s.inboxReviewPublication(ctx, d); observed != nil && !observed.Verified && missingPublicationConnection(observed.ConnectionState) {
				if result.Decision != "blocked" || result.RemainingWork != "external" || result.ManagerAction != "wait" || result.Revision != "" {
					t.Fatal("missing connection launched another repair", result)
				}
				return
			}
			if scenario == "implementation revision" {
				if result.RemainingWork != "verification" || !strings.Contains(result.Revision, input.Finding) || !strings.Contains(result.Revision, input.Revision) || !strings.HasPrefix(result.Revision, "First reconcile") || strings.Contains(result.Explanation, "Deploy again") || !strings.Contains(result.Finding, "rilis") {
					t.Fatal("implementation gap or publication reconciliation lost")
				}
				return
			}
			if scenario == "verification revision" && (!strings.Contains(result.Revision, input.Revision) || result.VisualInspection != input.VisualInspection) {
				t.Fatal("verification obligations lost")
			}
			allowed := scenario == "verified receipt" || scenario == "exact project build" || scenario == "owner forbids deployment"
			if (result.Decision == "pass") != allowed {
				t.Fatalf("publication gate result %+v", result)
			}
			if !allowed && (result.RemainingWork != "verification" || !strings.Contains(result.Finding, "rilis")) {
				t.Fatal("publication gap was lost")
			}
			if d.CommitSHA != commit || d.LatestAttempt.Status != task.AttemptStatusSucceeded {
				t.Fatal("review changed historical result")
			}
		})
	}
}

func TestLegacyPublicationAcceptanceReevaluatesWithoutExecution(t *testing.T) {
	for _, scenario := range []string{"missing publication", "verified receipt", "owner done", "owner prohibition", "cancelled approval", "current instructions", "clone scope"} {
		t.Run(scenario, func(t *testing.T) {
			s, d, oldReview := stalledReviewFixture(t, "completed")
			ctx := context.Background()
			projects := project.NewMemoryStore()
			if err := projects.CreateProject(ctx, &project.Project{ID: d.ProjectID, Name: "Project", RepoURL: "https://github.com/buchin/cuan"}); err != nil {
				t.Fatal(err)
			}
			s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
			scope := "Implement and deploy to production."
			if scenario == "clone scope" {
				scope = "Clone the registered repository and verify its structure."
			}
			if scenario == "owner prohibition" {
				scope = "Do not deploy. Return only local results."
			}
			d.Instructions = "Cuan managed delegation v1.\nApproved implementation.\nApproval message: " + d.SourceMessageID + "\nUser authorization: " + scope + "\nConversation: " + d.SourceRoomID + "\nReport actual progress and verification evidence. Stay within this scope; do not deploy unless authorized."
			if scenario == "current instructions" {
				d.Instructions = strings.Replace(d.Instructions, "; do not deploy unless authorized.", ".", 1)
			}
			d.CommitSHA = strings.Repeat("c", 40)
			if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
				t.Fatal(err)
			}
			origin, err := s.projectStudioRepo.GetMessageByID(ctx, d.SourceMessageID)
			if err != nil {
				t.Fatal(err)
			}
			origin.Content = scope
			if err := s.projectStudioRepo.UpdateMessage(ctx, origin); err != nil {
				t.Fatal(err)
			}
			if scenario == "cancelled approval" {
				q := db.InboxRequest{OwnerID: oldReview.OwnerID, AgentID: d.AssignedAgent, RoomID: d.SourceRoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", PayloadHash: "cancelled-approval", CreatedAt: time.Now().Add(-time.Hour), Deadline: time.Now().Add(time.Hour)}
				repo := s.projectStudioRepo.(db.InboxRequestRepository)
				if _, _, err := repo.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: scope, CreatedAt: q.CreatedAt}); err != nil {
					t.Fatal(err)
				}
				if changed, err := s.projectStudioRepo.(db.InboxLifecycleRepository).CancelInboxRequest(ctx, q.RoomID, q.RequestID); err != nil || !changed {
					t.Fatal("cancel approval", err)
				}
				d.Instructions = strings.Replace(d.Instructions, d.SourceMessageID, q.MessageID, 1)
				d.SourceMessageID = q.MessageID
				if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
					t.Fatal(err)
				}
			}
			d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
			if _, err := s.saveReviewAssessment(ctx, d, oldReview, inboxReviewEvaluation{Decision: "pass", RemainingWork: "none", Finding: "Local branch passed", Explanation: "Local branch passed"}); err != nil {
				t.Fatal(err)
			}
			if scenario == "verified receipt" {
				raw, _ := json.Marshal(task.DeploymentReceipt{Version: 1, TaskID: d.ID, AttemptID: d.LatestAttempt.ID, CommitSHA: d.CommitSHA, ReleaseSHA: d.CommitSHA, Environment: "production", Health: "healthy"})
				if err := s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{ID: task.DeploymentReceiptID(d.ID, d.LatestAttempt.ID, d.CommitSHA), TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: "deployment_verified", Author: "cuan-deploy", Message: string(raw), CreatedAt: time.Now().UTC()}); err != nil {
					t.Fatal(err)
				}
			}
			done := task.TaskStatusDone
			author := "coordinator"
			if scenario == "owner done" {
				author = "owner"
			}
			if _, err := s.taskSvc.UpdateTask(ctx, d.ID, author, task.UpdateTaskRequest{Status: &done}); err != nil {
				t.Fatal(err)
			}
			s.reconcileLegacyPublicationAcceptances(ctx)
			s.reconcileLegacyPublicationAcceptances(ctx)
			after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
			want := task.TaskStatusDone
			if scenario == "missing publication" {
				want = task.TaskStatusReview
			}
			if after.Status != want || len(after.Attempts) != 1 || after.LatestAttempt.Status != task.AttemptStatusSucceeded {
				t.Fatalf("audit replayed or changed result: %+v", after.LatestAttempt)
			}
			if after.Instructions != d.Instructions || after.CommitSHA != d.CommitSHA {
				t.Fatal("audit rewrote approved scope or result commit")
			}
			if scenario == "missing publication" {
				s.queueInboxTaskReview(ctx, after)
				s.queueInboxTaskReview(ctx, after)
				rows := reviewRequests(t, s, d.SourceRoomID)
				if len(rows) != 2 {
					t.Fatalf("publication review missing or duplicated: %d", len(rows))
				}
				var fresh db.InboxRequest
				for _, q := range rows {
					if q.RequestID != oldReview.RequestID {
						fresh = q
					}
				}
				if s.inboxReviewedTask(ctx, fresh) == nil {
					t.Fatal("new publication review does not bind task")
				}
				message, err := s.projectStudioRepo.GetMessageByID(ctx, fresh.MessageID)
				if err != nil || !strings.Contains(message.Content, inboxPublicationReviewPolicy) || strings.Contains(message.Content, "Stay within this scope; do not deploy unless authorized.") {
					t.Fatal("review lost corrected publication context")
				}
			}
		})
	}
}

func TestPublicationReviewBypassedWhenStagesOmitPublish(t *testing.T) {
	ctx := context.Background()
	s, _, _, _ := inboxFixture(t)
	projects := project.NewMemoryStore()
	if err := projects.CreateProject(ctx, &project.Project{ID: "no-publish-project", Name: "Project", RepoURL: "https://github.com/buchin/cuan", Metadata: registeredReleaseMetadata(t)}); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	commit := strings.Repeat("e", 40)
	now := time.Now().UTC()
	d := &task.TaskDetailResponse{
		Task: task.Task{
			ID:                "cleanup-task",
			ProjectID:         "no-publish-project",
			Title:             "Optimasi & Pembersihan Ruang Penyimpanan",
			CommitSHA:         commit,
			ExecutionCategory: task.ExecutionCategoryCode,
			RequiredStages:    []string{"implement"},
			Instructions:      "Pembersihan otomatis cache compiler dan file usang.",
			CompleteNote:      "Cleaned temporary artifacts and freed 24GB.",
		},
		ResultKind: "implementation",
		LatestAttempt: &task.TaskAttempt{
			ID:        "cleanup-attempt",
			TaskID:    "cleanup-task",
			Status:    task.AttemptStatusSucceeded,
			CreatedAt: now.Add(-time.Minute),
		},
	}
	if err := s.taskSvc.Store().CreateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().CreateAttempt(ctx, d.LatestAttempt); err != nil {
		t.Fatal(err)
	}

	observed := s.inboxReviewPublication(ctx, d)
	if observed == nil || observed.Required {
		t.Fatalf("expected publication to be not required when stages omit publish, got %+v", observed)
	}
	if observed.Provenance != "declared_stages_no_publication_required" {
		t.Fatalf("unexpected provenance: %s", observed.Provenance)
	}

	result := s.requirePublicationReview(ctx, d, inboxReviewEvaluation{Decision: "pass", Finding: "All clean", RemainingWork: "none"})
	if result.Decision != "pass" {
		t.Fatalf("expected pass decision, got %+v", result)
	}
}

func TestInboxReviewPublication_ProjectDeploymentSuccessAppliedTaskCommit(t *testing.T) {
	ctx := context.Background()
	s, _, _, _ := inboxFixture(t)
	repoDir := t.TempDir()
	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoDir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v (%s)", args, err, string(out))
		}
		return strings.TrimSpace(string(out))
	}
	runGit("init", "-b", "main")
	runGit("config", "user.email", "test@cuan.work")
	runGit("config", "user.name", "Cuan Test")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "Apply Task task-squash-1: Test squash apply")
	appliedSHA := runGit("rev-parse", "HEAD")

	projects := project.NewMemoryStore()
	if err := projects.CreateProject(ctx, &project.Project{
		ID:        "squash-project",
		Name:      "Squash Project",
		LocalPath: repoDir,
		RepoURL:   "https://github.com/buchin/cuan-landing",
		Metadata:  []byte(`{"auto_deploy":true,"deploy_script":"echo deploy"}`),
	}); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	depRepo := db.NewMemoryProjectDeploymentRepository()
	if err := depRepo.Create(ctx, &db.ProjectDeployment{
		ID:        "dep-1",
		ProjectID: "squash-project",
		CommitSHA: appliedSHA,
		Branch:    "main",
		Status:    db.DeploymentStatusSuccess,
	}); err != nil {
		t.Fatal(err)
	}
	s.projectDeploymentRepo = depRepo

	preSquashSHA := strings.Repeat("a", 40)
	d := &task.TaskDetailResponse{
		Task: task.Task{
			ID:        "task-squash-1",
			ProjectID: "squash-project",
			Title:     "Test squash apply",
			CommitSHA: preSquashSHA,
		},
		LatestAttempt: &task.TaskAttempt{
			ID:     "att-1",
			TaskID: "task-squash-1",
			Status: task.AttemptStatusSucceeded,
		},
	}
	if err := s.taskSvc.Store().CreateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().CreateAttempt(ctx, d.LatestAttempt); err != nil {
		t.Fatal(err)
	}

	observed := s.inboxReviewPublication(ctx, d)
	if observed == nil || !observed.Verified {
		t.Fatalf("expected squash-applied deployment commit to verify publication, got %+v", observed)
	}
	if observed.Provenance != "project_deployment_success_applied_task_commit" {
		t.Fatalf("unexpected provenance: %s", observed.Provenance)
	}
	if observed.ObservedRelease != appliedSHA {
		t.Fatalf("expected observed release %s, got %s", appliedSHA, observed.ObservedRelease)
	}
}

func TestRequirePublicationReview_InterruptedVerifierStaysVerification(t *testing.T) {
	s, d, q := stalledReviewFixture(t, "completed")
	ctx := context.Background()
	now := time.Now().UTC()
	sourceCommit := strings.Repeat("b", 40)
	runningRelease := strings.Repeat("c", 40)

	projects := project.NewMemoryStore()
	if err := projects.CreateProject(ctx, &project.Project{
		ID:       d.ProjectID,
		Name:     "Test Project",
		RepoURL:  "https://github.com/buchin/cuan",
		Metadata: registeredReleaseMetadata(t),
	}); err != nil {
		t.Fatal(err)
	}
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	s.cfg = &config.Config{Environment: "production", SourceRepository: "buchin/cuan", CommitSHA: runningRelease}

	// Mark latest attempt as an interrupted read-only verification attempt.
	reqID := db.GenerateUUID()
	savedLaunch, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{
		TaskID:           d.ID,
		ProjectID:        d.ProjectID,
		RequestID:        reqID,
		OwnerID:          q.OwnerID,
		RoomID:           d.SourceRoomID,
		SourceRequestID:  q.RequestID,
		AgentID:          q.AgentID,
		Revision:         "Scope: read-only repository. Do not push. Do not deploy.",
		InstructionsHash: task.LaunchInstructionsHash(d.Instructions),
		CreatedAt:        now,
		AvailableAt:      now,
		ExpiresAt:        now.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.taskSvc.ClaimLaunch(ctx, now, "tok", savedLaunch.ID); err != nil {
		t.Fatal(err)
	}
	verifierAtt := &task.TaskAttempt{
		ID:            task.AttemptRequestID(d.ID, reqID),
		TaskID:        d.ID,
		AttemptNumber: 2,
		RunnerID:      "cuan-runner",
		AgentName:     q.AgentID,
		Status:        task.AttemptStatusSucceeded,
		CreatedAt:     now,
	}
	if err := s.taskSvc.Store().CreateAttempt(ctx, verifierAtt); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.Store().CreateActivity(ctx, &task.TaskActivity{
		ID:           "act-pub-obs",
		TaskID:       d.ID,
		AttemptID:    &verifierAtt.ID,
		ActivityType: publicationConfigurationActivity,
		Author:       "coordinator",
		Message:      `{"project_id":"` + d.ProjectID + `","state":"current_process"}`,
		CreatedAt:    now,
	}); err != nil {
		t.Fatal(err)
	}
	row, err := s.taskSvc.Store().GetTask(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	row.CommitSHA = sourceCommit
	if err := s.taskSvc.Store().UpdateTask(ctx, row); err != nil {
		t.Fatal(err)
	}
	fresh, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	fresh.ResultKind = "implementation"
	fresh.LatestAttempt.Status = task.AttemptStatusInterrupted
	fresh.LatestAttempt.ErrorMessage = "Runner heartbeat lost and lease expired; marked interrupted and recoverable without auto-rerun."
	if !s.isPriorVerificationAttempt(ctx, fresh) {
		t.Fatalf("expected isPriorVerificationAttempt=true for interrupted verifier launch")
	}

	got := s.requirePublicationReview(ctx, fresh, inboxReviewEvaluation{
		Decision:      "revise",
		RemainingWork: "implementation",
		ManagerAction: "continue",
		Revision:      "Finish independent verification",
	})
	if got.RemainingWork != "verification" {
		t.Fatalf("expected RemainingWork=verification for interrupted verifier, got %q (source_review=%q)", got.RemainingWork, got.SourceReview)
	}
	if got.SourceReview == "pass" {
		t.Fatalf("expected SourceReview != pass when verifier was interrupted, got %q", got.SourceReview)
	}
}
