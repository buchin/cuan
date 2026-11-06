package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/publication"
	"github.com/buchin/cuan/internal/task"
)

func TestPublicationConfigurationChangesWakeOneBoundReviewWithoutExecution(t *testing.T) {
	ctx := context.Background()
	s, d, q := stalledReviewFixture(t, "completed")
	projects := project.NewMemoryStore()
	s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
	p := &project.Project{ID: d.ProjectID, RepoURL: "https://github.com/example/site", UpdatedAt: time.Now()}
	projects.CreateProject(ctx, p)
	requests, _ := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, d.SourceRoomID)
	var origin db.InboxRequest
	for _, r := range requests {
		if r.MessageID == d.SourceMessageID {
			origin = r
		}
	}
	before := inboxReviewEvidenceDigest(d)
	s.observeInboxPublicationConfiguration(ctx, d, origin)
	if publicationConfigurationObservation(d) == nil || inboxReviewEvidenceDigest(d) == before {
		t.Fatal("new prerequisite not review evidence")
	}
	var record publicationConfigurationRecord
	json.Unmarshal([]byte(publicationConfigurationObservation(d).Message), &record)
	if record.State != "not_configured" {
		t.Fatal("wrong diagnosis")
	}
	s.queueInboxTaskReview(ctx, d)
	s.queueInboxTaskReview(ctx, d)
	if len(reviewRequests(t, s, q.RoomID)) != 2 {
		t.Fatal("missing or duplicate prerequisite review")
	}
	oldDigest := inboxReviewEvidenceDigest(d)
	p.Metadata = registeredReleaseMetadata(t)
	p.UpdatedAt = p.UpdatedAt.Add(time.Second)
	projects.UpdateProject(ctx, p)
	s.queueInboxTaskReview(ctx, d)
	s.queueInboxTaskReview(ctx, d)
	if len(reviewRequests(t, s, q.RoomID)) != 3 || inboxReviewEvidenceDigest(d) == oldDigest {
		t.Fatal("connection change not observed exactly once")
	}
	if strings.Contains(publicationConfigurationObservation(d).Message, "fixture") || strings.Contains(publicationConfigurationObservation(d).Message, "SSH_PASSWORD") {
		t.Fatal("credential in review observation")
	}
	latest, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if latest.Status == task.TaskStatusDone || len(latest.Attempts) != len(d.Attempts) {
		t.Fatal("connection configuration executed or accepted task")
	}
}

func TestPublicationConfigurationDoesNotManufactureOwnerAuthorization(t *testing.T) {
	for _, scenario := range []string{"wrong owner", "cancelled approval", "wrong room", "cancelled task", "read-only original", "no-deploy original"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			s, d, _ := stalledReviewFixture(t, "completed")
			projects := project.NewMemoryStore()
			s.projectSvc = project.NewService(projects, nil, "", t.TempDir())
			projects.CreateProject(ctx, &project.Project{ID: d.ProjectID, RepoURL: "https://github.com/example/site"})
			requests, _ := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, d.SourceRoomID)
			var origin db.InboxRequest
			for _, r := range requests {
				if r.MessageID == d.SourceMessageID {
					origin = r
				}
			}
			switch scenario {
			case "wrong owner":
				origin.OwnerID = "foreign"
			case "cancelled approval":
				origin.State = "cancelled"
			case "wrong room":
				origin.RoomID = "foreign"
			case "cancelled task":
				d.Status = "cancelled"
			case "read-only original":
				d.Instructions = "Scope: read-only repository."
			case "no-deploy original":
				d.Instructions = "Do not deploy."
			}
			s.observeInboxPublicationConfiguration(ctx, d, origin)
			if publicationConfigurationObservation(d) != nil {
				t.Fatal("unauthorized observation")
			}
		})
	}
}

func TestPublicationContextDistinguishesMissingConnectionFromFailedObservation(t *testing.T) {
	for _, scenario := range []string{"missing", "incomplete", "configured unreachable", "current process", "foreign process", "verified receipt"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			s, d, _ := stalledReviewFixture(t, "completed")
			store := project.NewMemoryStore()
			s.projectSvc = project.NewService(store, nil, "", t.TempDir())
			p := &project.Project{ID: d.ProjectID, RepoURL: "https://github.com/example/site"}
			if scenario == "configured unreachable" {
				p.Metadata = registeredReleaseMetadata(t)
			}
			if scenario == "incomplete" {
				p.Metadata = json.RawMessage(`{"cuan_settings":{"environment":"CUAN_PUBLICATION_VERIFIER=ssh-release"}}`)
			}
			store.CreateProject(ctx, p)
			d.CommitSHA = strings.Repeat("a", 40)
			s.cfg = &config.Config{Environment: "production", SourceRepository: "example/foreign", CommitSHA: d.CommitSHA}
			if scenario == "current process" {
				s.cfg.SourceRepository = "example/site"
			}
			calls := 0
			s.publicationObserver = func(context.Context, publication.Config) (string, error) {
				calls++
				return "", publication.ErrUnverified
			}
			if scenario == "verified receipt" {
				receipt := task.DeploymentReceipt{Version: 1, TaskID: d.ID, AttemptID: d.LatestAttempt.ID, CommitSHA: d.CommitSHA, ReleaseSHA: d.CommitSHA, Environment: "production", Health: "healthy"}
				raw, _ := json.Marshal(receipt)
				d.Activities = append(d.Activities, &task.TaskActivity{ID: task.DeploymentReceiptID(d.ID, d.LatestAttempt.ID, d.CommitSHA), TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: "deployment_verified", Author: "cuan-deploy", Message: string(raw), CreatedAt: time.Now()})
			}
			observed := s.inboxReviewPublication(ctx, d)
			want := "not_configured"
			if scenario == "incomplete" {
				want = "incomplete"
			}
			if scenario == "configured unreachable" {
				want = "configured"
			}
			if scenario == "current process" {
				want = "current_process"
			}
			if observed == nil || observed.ConnectionState != want || observed.Verified != (scenario == "current process" || scenario == "verified receipt") {
				t.Fatal("wrong publication diagnosis", observed)
			}
			if calls > 0 && scenario != "configured unreachable" {
				t.Fatal("unconfigured or foreign target was probed")
			}
			decision := s.requirePublicationReview(ctx, d, inboxReviewEvaluation{Decision: "pass", Explanation: "Reported done", RemainingWork: "none"})
			if missingPublicationConnection(want) && !observed.Verified && (decision.Decision != "blocked" || decision.ManagerAction != "wait" || !strings.Contains(decision.ManagerReason, missingPublicationConnectionGuidance)) {
				t.Fatal("specific prerequisite lost")
			}
			if s.requirePublicationReview(ctx, d, decision) != decision {
				t.Fatal("guard duplicated revision")
			}
		})
	}
}

func publicationConfigurationPostgresContract(t *testing.T, database *sql.DB, repo *db.SQLProjectStudioRepository, origin db.InboxRequest) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	store := task.NewDBStore(database)
	pstore := project.NewDBStore(database)
	s := &Server{taskSvc: task.NewService(store, nil), projectStudioRepo: repo, projectSvc: project.NewService(pstore, nil, "", t.TempDir())}
	item := &task.Task{ID: "publication-config-" + db.GenerateUUID(), ProjectID: "default", SourceRoomID: origin.RoomID, SourceMessageID: origin.MessageID, AssignedAgent: origin.AgentID, Title: "Verify existing approved result", Instructions: "Cuan managed delegation v1.\nVerify the approved website.", Status: task.TaskStatusReview, CreatedAt: now, UpdatedAt: now}
	if err := store.CreateTask(ctx, item); err != nil {
		t.Fatal(err)
	}
	attempt := &task.TaskAttempt{ID: db.GenerateUUID(), TaskID: item.ID, Status: task.AttemptStatusSucceeded, CreatedAt: now, ExitCode: new(int)}
	if err := store.CreateAttempt(ctx, attempt); err != nil {
		t.Fatal(err)
	}
	detail, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	before := inboxReviewEvidenceDigest(detail)
	s.observeInboxPublicationConfiguration(ctx, detail, origin)
	reloaded, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	event := publicationConfigurationObservation(reloaded)
	if event == nil || inboxReviewEvidenceDigest(reloaded) == before {
		t.Fatal("PostgreSQL prerequisite activity was not persisted")
	}
	s.observeInboxPublicationConfiguration(ctx, reloaded, origin)
	p, err := pstore.GetProject(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	p.Metadata = registeredReleaseMetadata(t)
	p.UpdatedAt = now.Add(time.Second)
	if err := pstore.UpdateProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	s.observeInboxPublicationConfiguration(ctx, reloaded, origin)
	final, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, a := range final.Activities {
		if a.ActivityType == publicationConfigurationActivity {
			count++
		}
	}
	var changed publicationConfigurationRecord
	json.Unmarshal([]byte(publicationConfigurationObservation(final).Message), &changed)
	if count != 2 || changed.State != "configured" || final.Status == task.TaskStatusDone || len(final.Attempts) != 1 {
		t.Fatal("PostgreSQL configuration lifecycle", count, changed.State)
	}
}

func TestMissingPublicationConnectionWaitsWithoutRepeatingWorkAndRechecksAfterSetup(t *testing.T) {
	ctx := context.Background()
	s, d := managerFixture(t)
	store := project.NewMemoryStore()
	s.projectSvc = project.NewService(store, nil, "", t.TempDir())
	p := &project.Project{ID: d.ProjectID, RepoURL: "https://github.com/example/site", UpdatedAt: time.Now()}
	if err := store.CreateProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	d.CommitSHA = strings.Repeat("a", 40)
	if err := s.taskSvc.Store().UpdateTask(ctx, &d.Task); err != nil {
		t.Fatal(err)
	}
	q := reviewRequests(t, s, d.SourceRoomID)[0]
	raw, _ := json.Marshal(inboxReviewEvaluation{Decision: "revise", RemainingWork: "implementation", Finding: "Release proof missing", Explanation: "Reconcile release", Revision: "Synchronize the release environment again", ManagerAction: "replan", ManagerReason: "No registered connection"})
	result := s.applyInboxReviewEvaluation(ctx, q, string(raw))
	current, err := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	assessment, ok := reviewAssessment(current, q.RequestID)
	if !ok || assessment.Decision != "blocked" || assessment.ManagerAction != "wait" || assessment.RemainingWork != "external" || current.Status != task.TaskStatusBlocked || len(current.Attempts) != len(d.Attempts) {
		t.Fatal("prerequisite became execution", assessment, result)
	}
	if launch, _ := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, q.RequestID)); launch != nil {
		t.Fatal("missing connection created another attempt")
	}
	p.Metadata = registeredReleaseMetadata(t)
	p.UpdatedAt = p.UpdatedAt.Add(time.Second)
	if err := store.UpdateProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	s.queueInboxTaskReview(ctx, current)
	s.queueInboxTaskReview(ctx, current)
	requests := reviewRequests(t, s, d.SourceRoomID)
	if len(requests) != 2 {
		t.Fatal("setup did not wake one review", len(requests))
	}
	var pending db.InboxRequest
	for _, r := range requests {
		if r.RequestID != q.RequestID {
			pending = r
		}
	}
	if pending.State != "queued" || s.inboxReviewedTask(ctx, pending) == nil {
		t.Fatal("setup did not bind review to saved task")
	}
	after, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if len(after.Attempts) != len(d.Attempts) || after.Status == task.TaskStatusDone {
		t.Fatal("setup itself executed or accepted task")
	}
}
