package server

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func TestLegacyScopeAcceptanceAuditMemory(t *testing.T) {
	for _, scenario := range []string{"affected phase", "own repository", "archived", "cancelled", "cancelled predecessor", "older active execution", "pending launch", "owner acceptance", "missing legacy receipt", "unverified approval", "changed snapshot"} {
		t.Run(scenario, func(t *testing.T) { scopeAcceptanceAuditContract(t, scenario, nil) })
	}
}

func TestLegacyScopeAcceptanceAuditPostgres(t *testing.T) {
	for _, scenario := range []string{"affected phase", "own repository", "cancelled", "older active execution", "pending launch", "missing legacy receipt", "changed snapshot"} {
		t.Run(scenario, func(t *testing.T) {
			withPhasePostgres(t, func(database *sql.DB) { scopeAcceptanceAuditContract(t, scenario, database) })
		})
	}
}

func scopeAcceptanceAuditContract(t *testing.T, scenario string, database *sql.DB) {
	var databases []*sql.DB
	if database != nil {
		databases = append(databases, database)
	}
	s, owner, parent, child, _ := phaseLineageFixture(t, databases...)
	s.inboxStopping.Store(true)
	ctx := context.Background()
	store := s.taskSvc.Store()
	now := time.Now().UTC()
	projects := project.NewMemoryStore()
	repo := "https://github.com/fixture/dojo"
	if scenario == "own repository" {
		repo = "https://github.com/fixture/cuan"
	}
	if err := projects.CreateProject(ctx, &project.Project{ID: child.ProjectID, RepoURL: repo, DefaultBranch: "main", IsArchived: scenario == "archived"}); err != nil {
		t.Fatal(err)
	}
	if database != nil {
		if _, err := database.ExecContext(ctx, `UPDATE projects SET repo_url=$1 WHERE id=$2`, repo, child.ProjectID); err != nil {
			t.Fatal(err)
		}
	}
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	s.cfg.Environment, s.cfg.SourceRepository = "production", "fixture/cuan"
	if err := s.taskSvc.FinishTaskLaunches(ctx, child.ID, "stopped"); err != nil {
		t.Fatal(err)
	}
	a := &task.TaskAttempt{ID: child.ID + "-source", TaskID: child.ID, AgentName: owner.AgentID, RunnerID: "fixture", Status: task.AttemptStatusSucceeded, CancelRequested: scenario == "cancelled", CreatedAt: now}
	if err := store.CreateAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	if scenario == "older active execution" {
		old := *a
		old.ID += "-old"
		old.CreatedAt = now.Add(-time.Hour)
		old.Status = task.AttemptStatusRunning
		if err := store.CreateAttempt(ctx, &old); err != nil {
			t.Fatal(err)
		}
	}
	if scenario == "pending launch" {
		_, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: child.ID, ProjectID: child.ProjectID, OwnerID: owner.OwnerID, RoomID: child.SourceRoomID, RequestID: db.GenerateUUID(), SourceRequestID: owner.RequestID, AgentID: owner.AgentID, InstructionsHash: task.LaunchInstructionsHash(child.Instructions), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
	}
	if scenario == "cancelled predecessor" {
		parent.Status = "cancelled"
		if err := store.UpdateTask(ctx, &parent.Task); err != nil {
			t.Fatal(err)
		}
	}
	child.Status, child.Summary, child.CompleteNote = task.TaskStatusDone, "Library ready", "Package tests passed"
	child.CommitSHA, child.DiffSummary = strings.Repeat("a", 40), task.LegacyForeignPagePath+" | 435 +"
	if err := store.UpdateTask(ctx, &child.Task); err != nil {
		t.Fatal(err)
	}
	author := "coordinator"
	if scenario == "owner acceptance" {
		author = "owner"
	}
	if err := store.CreateActivity(ctx, &task.TaskActivity{ID: child.ID + "-accepted", TaskID: child.ID, AttemptID: &a.ID, ActivityType: task.ActivityTypeStatusChange, Author: author, NewStatus: task.TaskStatusDone, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if scenario != "missing legacy receipt" {
		if err := store.CreateActivity(ctx, &task.TaskActivity{ID: child.ID + "-bad-requeue", TaskID: child.ID, ActivityType: "attempt_transition", Author: "cuan_dod_evaluator", NewStatus: task.TaskStatusQueued, Message: task.LegacyForeignPageRequirement, CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	if scenario == "unverified approval" {
		m, _ := s.projectStudioRepo.GetMessageByID(ctx, parent.SourceMessageID)
		m.Content = "The prior plan approval is no longer available."
		if err := s.projectStudioRepo.UpdateMessage(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	before, err := s.taskSvc.GetTaskDetail(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := scenario == "affected phase" || scenario == "changed snapshot"
	oldReviewID := ""
	if want {
		key := "task-review:" + a.ID
		oldReviewID = delegationMessageID(key)
		old := db.InboxRequest{OwnerID: owner.OwnerID, AgentID: owner.AgentID, RoomID: child.SourceRoomID, RequestID: oldReviewID, MessageID: delegationMessageID(key + ":input"), ReplyID: delegationMessageID(key + ":reply"), PayloadHash: oldReviewID, State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour)}
		content := "Review [/tasks/" + child.ID + "]\nReview evidence digest: " + inboxReviewEvidenceDigest(before) + "\n" + strings.Join([]string{inboxReportProvenancePolicy, inboxReviewAssessmentPolicy, inboxVerificationReviewPolicy, inboxVisualReviewPolicy, inboxOperationalReviewPolicy, inboxPublicationReviewPolicy, inboxOwnershipPolicy, inboxReviewProviderPolicy}, "\n")
		repo := s.projectStudioRepo.(db.InboxRequestRepository)
		if _, _, err := repo.AcceptInboxRequest(ctx, old, &db.ProjectMessage{ID: old.MessageID, RoomID: old.RoomID, AuthorType: "system", AuthorID: "cuan_work_review", Content: content, CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
		old.State = "completed"
		if err := repo.FinishInboxRequest(ctx, old, nil); err != nil {
			t.Fatal(err)
		}
	}
	if scenario == "changed snapshot" {
		changed, err := store.(task.ScopeAcceptanceAuditStore).RequeueScopeAcceptance(ctx, child.ID, a.ID, before.UpdatedAt.Add(-time.Second), "stale audit")
		if err != nil || changed {
			t.Fatal("stale scope audit changed accepted work", err)
		}
	}
	s.reconcileLegacyScopeAcceptances(ctx)
	after, err := s.taskSvc.GetTaskDetail(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if (after.Status == task.TaskStatusReview) != want {
		t.Fatalf("audit status %s; should reopen=%t", after.Status, want)
	}
	if after.LatestAttempt.ID != a.ID || after.LatestAttempt.Status != task.AttemptStatusSucceeded || len(after.Attempts) != len(before.Attempts) || after.Instructions != before.Instructions || after.CommitSHA != before.CommitSHA {
		t.Fatal("audit changed execution history or scope")
	}
	if want {
		if scopeAcceptanceAudit(after) == nil || inboxReviewEvidenceDigest(after) == inboxReviewEvidenceDigest(before) {
			t.Fatal("atomic audit receipt missing from fresh review evidence")
		}
		s.queueInboxTaskReview(ctx, after)
		fresh := 0
		for _, q := range reviewRequests(t, s, after.SourceRoomID) {
			if q.RequestID == oldReviewID {
				if q.State != "completed" {
					t.Fatal("old provider review was replayed")
				}
				continue
			}
			if d := s.inboxReviewedTask(ctx, q); d != nil && d.ID == after.ID {
				if q.State != "queued" {
					t.Fatal("fresh review not queued")
				}
				fresh++
			}
		}
		if fresh != 1 {
			t.Fatalf("phase recovery queued %d fresh snapshot reviews", fresh)
		}
		// A repeated discovery after acceptance must not reopen the same attempt.
		after.Status = task.TaskStatusDone
		if err := store.UpdateTask(ctx, &after.Task); err != nil {
			t.Fatal(err)
		}
		s.scopeAuditNext.Store(0)
		s.reconcileLegacyScopeAcceptances(ctx)
		again, _ := s.taskSvc.GetTaskDetail(ctx, child.ID)
		if again.Status != task.TaskStatusDone {
			t.Fatal("same accepted attempt audited twice")
		}
	}
	rows, err := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, child.SourceRoomID)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range rows {
		if q.MessageID != owner.MessageID && q.State == "running" {
			t.Fatal("audit executed a provider")
		}
	}
}
