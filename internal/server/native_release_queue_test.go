package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestNativeReleaseQueueDoesNotUseAICapacity(t *testing.T) {
	nativeReleaseQueueContract(t, nil, "ready")
}
func TestNativeReleaseQueueRejectsRevokedSourceBeforeExecution(t *testing.T) {
	nativeReleaseQueueContract(t, nil, "revoked")
}
func TestNativeReleaseQueueRejectsForeignApproval(t *testing.T) {
	nativeReleaseQueueContract(t, nil, "foreign approval")
}
func TestNativeReleaseQueuePostgres(t *testing.T) {
	for _, scenario := range []string{"ready", "revoked", "foreign approval"} {
		t.Run(scenario, func(t *testing.T) {
			withPhasePostgres(t, func(database *sql.DB) { nativeReleaseQueueContract(t, database, scenario) })
		})
	}
}

func nativeReleaseQueueContract(t *testing.T, database *sql.DB, scenario string) {
	t.Helper()
	s, token, _, _ := inboxFixture(t)
	s.inboxStopping.Store(true)
	ctx := context.Background()
	owner, err := s.authSvc.ValidateToken(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	var store task.Store = task.NewMemoryStore()
	var runtimeStore runner.Store
	if database != nil {
		if _, err := database.ExecContext(ctx, `INSERT INTO projects(id,name,repo_url) VALUES('default','Default','') ON CONFLICT DO NOTHING`); err != nil {
			t.Fatal(err)
		}
		store = task.NewDBStore(database)
		runtimeStore = runner.NewDBStore(database)
		s.projectStudioRepo = db.NewSQLProjectStudioRepository(database)
	} else {
		runtimeStore = runner.NewMemoryStore(store)
	}
	s.taskSvc = task.NewService(store, nil)
	s.runnerSvc = runner.NewService(runtimeStore, store)
	room := s.resolveConversationContext(ctx, "agent_lifecycle", "")
	now := time.Now().UTC()
	q := db.InboxRequest{OwnerID: owner.ID, AgentID: "agent_lifecycle", RoomID: room.RoomID, RequestID: db.GenerateUUID(), MessageID: db.GenerateUUID(), ReplyID: db.GenerateUUID(), State: "queued", CreatedAt: now, Deadline: now.Add(time.Hour), PayloadHash: "approved-native-release"}
	repo := s.projectStudioRepo.(db.InboxRequestRepository)
	if _, _, err := repo.AcceptInboxRequest(ctx, q, &db.ProjectMessage{ID: q.MessageID, RoomID: q.RoomID, AuthorType: "user", Content: "Implement and deploy the approved feature.", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	q.State = "completed"
	if err := repo.FinishInboxRequest(ctx, q, nil); err != nil {
		t.Fatal(err)
	}
	item := &task.Task{ID: "native-release-task", ProjectID: "default", SourceRoomID: q.RoomID, SourceMessageID: q.MessageID, AssignedAgent: q.AgentID, Title: "Implement approved feature", Instructions: "Cuan managed delegation v1.\nImplement and deploy the approved feature.", Status: task.TaskStatusReview, ExecutionCategory: task.ExecutionCategoryCode, CommitSHA: strings.Repeat("a", 40), Summary: "Source handoff", CompleteNote: "Native checks passed; publication pending", CreatedAt: now, UpdatedAt: now}
	start, finish := now.Add(-time.Minute), now.Add(-time.Second)
	exit := 0
	source := &task.TaskAttempt{ID: "native-source", TaskID: item.ID, RunnerID: "source-runner", AgentName: "agy", Status: task.AttemptStatusSucceeded, ExitCode: &exit, CreatedAt: start.Add(-time.Second), StartedAt: &start, FinishedAt: &finish}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(store.CreateTask(ctx, item))
	must(store.CreateAttempt(ctx, source))
	summary := "Project verification: test.sh --fast (exit 0)"
	p := task.NativeReleaseIntent{SourceAttemptID: source.ID, SourceCommit: item.CommitSHA, SourceVerificationHash: task.VerificationSummaryHash(summary), InstructionsHash: task.LaunchInstructionsHash(item.Instructions), OwnerID: q.OwnerID, RoomID: q.RoomID, SourceRequestID: q.RequestID, Repository: "owner/repo", TargetBranch: "dev", ManifestHash: strings.Repeat("b", 64)}
	checks, _ := json.Marshal(task.RunnerVerificationRecord{Summary: summary, Changes: task.NewRunnerChangeRecord(item.CommitSHA, "source.go | 1 +")})
	handoff, _ := json.Marshal(task.NativeReleaseHandoff{Version: 1, TaskID: item.ID, AttemptID: source.ID, SourceCommit: p.SourceCommit, InstructionsHash: p.InstructionsHash, VerificationHash: p.SourceVerificationHash, Repository: p.Repository, TargetBranch: p.TargetBranch, ManifestHash: p.ManifestHash})
	for _, a := range []*task.TaskActivity{{ID: "report", TaskID: item.ID, AttemptID: &source.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Message: "Actual source report", CreatedAt: start}, {ID: "kind", TaskID: item.ID, AttemptID: &source.ID, ActivityType: "agent_result_kind", NewStatus: "implementation", CreatedAt: start}, {ID: "verification:" + source.ID, TaskID: item.ID, AttemptID: &source.ID, Author: source.RunnerID, ActivityType: task.ActivityTypeRunnerVerification, Message: string(checks), CreatedAt: finish}, {ID: "native-release-handoff:" + source.ID, TaskID: item.ID, AttemptID: &source.ID, Author: source.RunnerID, ActivityType: task.ActivityNativeReleaseHandoff, Message: string(handoff), CreatedAt: finish}} {
		must(store.CreateActivity(ctx, a))
	}
	detail, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	var capacityCalls atomic.Int32
	oldFetch := fetchInboxModelUsage
	defer func() { fetchInboxModelUsage = oldFetch }()
	fetchInboxModelUsage = func(context.Context, string, string) agent.AccountModelUsage {
		capacityCalls.Add(1)
		return agent.AccountModelUsage{}
	}
	if scenario == "foreign approval" {
		q.OwnerID = "another-owner"
		p.OwnerID = q.OwnerID
	}
	raw, _ := json.Marshal(p)
	requestID := task.NativeReleaseRequestID(item.ID, p.SourceCommit)
	output := s.startAuthorizedLaunch(ctx, q, detail, requestID, q.RequestID, task.NativeReleaseIntentMarker+string(raw))
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(item.ID, requestID))
	if scenario == "foreign approval" {
		if launch != nil {
			t.Fatal("foreign approval persisted native execution intent")
		}
		current, loadErr := s.taskSvc.GetTaskDetail(ctx, item.ID)
		must(loadErr)
		if current.Status != task.TaskStatusReview || len(current.Attempts) != 1 || capacityCalls.Load() != 0 {
			t.Fatal("foreign approval changed task state or consumed AI")
		}
		return
	}
	must(err)
	if launch == nil || launch.State != "started" {
		t.Fatal("native release was not durably admitted", output, launch)
	}
	current, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	if len(current.Attempts) != 2 || current.LatestAttempt.Status != task.AttemptStatusQueued || current.Status == task.TaskStatusDone || capacityCalls.Load() != 0 {
		t.Fatal("native admission used AI or accepted publication", capacityCalls.Load())
	}
	restarted := *s
	restarted.taskSvc = task.NewService(store, nil)
	restarted.startAuthorizedLaunch(ctx, q, detail, requestID, q.RequestID, task.NativeReleaseIntentMarker+string(raw))
	after, err := restarted.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	if len(after.Attempts) != 2 {
		t.Fatal("reconstructed server replayed native stage")
	}
	_, err = s.runnerSvc.Heartbeat(ctx, runner.HeartbeatRequest{RunnerID: "native-runner"}, time.Minute)
	must(err)
	old, err := s.runnerSvc.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "native-runner", SupportedAgents: []string{"other-provider"}}, time.Minute)
	must(err)
	if old.Claimed {
		t.Fatal("older runner acquired native stage")
	}
	if scenario == "revoked" {
		source.CancelRequested = true
		must(store.UpdateAttempt(ctx, source))
	}
	claim, err := s.runnerSvc.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "native-runner", SupportedAgents: []string{"other-provider"}, SupportsNativeRelease: true}, time.Minute)
	if scenario == "revoked" {
		if err == nil || claim != nil {
			t.Fatal("revoked source was claimed")
		}
		current, loadErr := s.taskSvc.GetTaskDetail(ctx, item.ID)
		must(loadErr)
		if current.LatestAttempt.Status != task.AttemptStatusFailed || current.Status == task.TaskStatusDone || capacityCalls.Load() != 0 {
			t.Fatal("revoked native claim remained live or was accepted")
		}
		return
	}
	must(err)
	if !claim.Claimed || claim.NativeRelease == nil || claim.NativeRelease.SourceCommit != p.SourceCommit || capacityCalls.Load() != 0 {
		t.Fatal("native claim lost source or depended on AI capacity")
	}
}
