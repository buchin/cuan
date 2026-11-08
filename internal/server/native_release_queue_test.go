package server

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/publication"
	"github.com/buchin/cuan/internal/release"
	"golang.org/x/crypto/ssh"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
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
func TestNativeReleaseHTTPClaimDoesNotGrantModelExecution(t *testing.T) {
	nativeReleaseQueueContract(t, nil, "http")
}
func TestNativeReleaseQueueRejectsRevokedSourceBeforeExecution(t *testing.T) {
	nativeReleaseQueueContract(t, nil, "revoked")
}
func TestNativeReleaseQueueRejectsForeignApproval(t *testing.T) {
	nativeReleaseQueueContract(t, nil, "foreign approval")
}
func TestNativeReleasePublicationObservesPreparedTarget(t *testing.T) {
	for _, scenario := range []string{"publication", "publication activation receipt", "publication wrong target", "publication revoked", "publication missing process", "publication stale observation", "publication wrong process target", "publication mutated checkout", "publication no live proof"} {
		t.Run(scenario, func(t *testing.T) { nativeReleaseQueueContract(t, nil, scenario) })
	}
}
func TestNativeReleasePreparationPublishesActualBoundCheckout(t *testing.T) {
	nativeReleaseQueueContract(t, nil, "prepare")
}
func TestInitialNativeReleasePreparationPublishesApprovedSource(t *testing.T) {
	for _, scenario := range []string{"initial merge", "initial merge conflict", "initial verify", "initial verify changed release", "initial verify archived", "initial verify revoked", "initial contained", "initial no access", "initial missing verifier", "initial missing tool", "initial wrong source", "initial revoked"} {
		t.Run(scenario, func(t *testing.T) { nativeReleaseQueueContract(t, nil, scenario) })
	}
}
func TestNativeReleaseHandoffSchedulesWithoutPMModel(t *testing.T) {
	for _, scenario := range []string{"schedule", "schedule disabled", "schedule foreign", "schedule revoked"} {
		t.Run(scenario, func(t *testing.T) { nativeReleaseQueueContract(t, nil, scenario) })
	}
}
func TestNativeReleaseHandoffRecoversSourceVerificationAfterTerminalObservation(t *testing.T) {
	nativeReleaseQueueContract(t, nil, "schedule observed")
}
func TestReviewedSourceHandoffSchedulesNativeReleaseWithoutPMModel(t *testing.T) {
	nativeReleaseQueueContract(t, nil, "schedule reviewed")
}
func TestReviewedSourceHandoffDoesNotRepeatVerifiedPublication(t *testing.T) {
	nativeReleaseQueueContract(t, nil, "schedule reviewed delivered")
}
func TestNativeReleaseQueuePostgres(t *testing.T) {
	for _, scenario := range []string{"ready", "revoked", "foreign approval", "prepare", "initial merge", "initial verify", "schedule", "schedule observed", "schedule reviewed", "schedule reviewed delivered", "publication activation receipt"} {
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
	nativeSource, nativeManifest := strings.Repeat("a", 40), strings.Repeat("b", 64)
	preparedTarget := ""
	if scenario == "prepare" || strings.HasPrefix(scenario, "initial") || scenario == "schedule" || scenario == "schedule observed" {
		if scenario == "initial missing tool" {
			nativeSource, preparedTarget, nativeManifest = nativeReleaseProjectFixture(t, s, "cuan-fixture-unavailable-tool")
		} else {
			nativeSource, preparedTarget, nativeManifest = nativeReleaseProjectFixture(t, s)
		}
	}
	item := &task.Task{ID: "native-release-task", ProjectID: "default", SourceRoomID: q.RoomID, SourceMessageID: q.MessageID, AssignedAgent: q.AgentID, Title: "Implement approved feature", Instructions: "Cuan managed delegation v1.\nImplement and deploy the approved feature.", Status: task.TaskStatusReview, ExecutionCategory: task.ExecutionCategoryCode, CommitSHA: nativeSource, Summary: "Source handoff", CompleteNote: "Native checks passed; publication pending", CreatedAt: now, UpdatedAt: now}
	if scenario == "initial verify" {
		item.DoDChecklist = []task.DoDItem{{ID: "delivered-content", Text: "work.txt contains target in the delivered revision", Completed: false, Evidence: "Awaiting independent inspection"}}
	}
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
	p := task.NativeReleaseIntent{SourceAttemptID: source.ID, SourceCommit: item.CommitSHA, SourceVerificationHash: task.VerificationSummaryHash(summary), InstructionsHash: task.LaunchInstructionsHash(item.Instructions), OwnerID: q.OwnerID, RoomID: q.RoomID, SourceRequestID: q.RequestID, Repository: "owner/repo", TargetBranch: "dev", ManifestHash: nativeManifest}
	checks, _ := json.Marshal(task.RunnerVerificationRecord{Summary: summary, Changes: task.NewRunnerChangeRecord(item.CommitSHA, "source.go | 1 +")})
	handoff, _ := json.Marshal(task.NativeReleaseHandoff{Version: 1, TaskID: item.ID, AttemptID: source.ID, SourceCommit: p.SourceCommit, InstructionsHash: p.InstructionsHash, VerificationHash: p.SourceVerificationHash, Repository: p.Repository, TargetBranch: p.TargetBranch, ManifestHash: p.ManifestHash})
	for _, a := range []*task.TaskActivity{{ID: "report", TaskID: item.ID, AttemptID: &source.ID, ActivityType: "agent_report", NewStatus: task.TaskStatusReview, Message: "Actual source report", CreatedAt: start}, {ID: "kind", TaskID: item.ID, AttemptID: &source.ID, ActivityType: "agent_result_kind", NewStatus: "implementation", CreatedAt: start}, {ID: "verification:" + source.ID, TaskID: item.ID, AttemptID: &source.ID, Author: source.RunnerID, ActivityType: task.ActivityTypeRunnerVerification, Message: string(checks), CreatedAt: finish}, {ID: "native-release-handoff:" + source.ID, TaskID: item.ID, AttemptID: &source.ID, Author: source.RunnerID, ActivityType: task.ActivityNativeReleaseHandoff, Message: string(handoff), CreatedAt: finish}} {
		must(store.CreateActivity(ctx, a))
	}
	detail, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	if !strings.HasPrefix(scenario, "schedule") || strings.HasPrefix(scenario, "schedule reviewed") {
		detail = approveNativeReleaseFixtureSource(t, s, store, runtimeStore, item, source, q, &p)
	}
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
	output := ""
	if strings.HasPrefix(scenario, "schedule") {
		if scenario != "schedule" && scenario != "schedule observed" {
			projects := project.NewMemoryStore()
			environment := "CUAN_MANAGED_RELEASE=1"
			if scenario == "schedule disabled" {
				environment = ""
			}
			metadata, _ := json.Marshal(map[string]any{"cuan_settings": project.Settings{WorkspaceRevision: "initial", Environment: environment}})
			must(projects.CreateProject(ctx, &project.Project{ID: item.ProjectID, RepoURL: "https://github.com/owner/repo.git", DefaultBranch: "dev", Metadata: metadata}))
			s.projectSvc = project.NewService(projects, nil, "fixture", t.TempDir())
		}
		if scenario == "schedule foreign" {
			q.OwnerID = "foreign-owner"
		}
		if scenario == "schedule revoked" {
			source.CancelRequested = true
			must(store.UpdateAttempt(ctx, source))
		}
		if scenario == "schedule observed" {
			assessment := inboxReviewEvaluation{Decision: "revise", RemainingWork: "verification", Explanation: "Inspect existing release before replaying any publication.", Revision: "Reconcile existing production; original requirements remain unfinished."}
			_, err := s.saveReviewAssessment(ctx, detail, q, assessment)
			must(err)
			observed := time.Now().UTC()
			inspection := task.LiveInspectionIntent{Purpose: task.CurrentOutcomeInspection, SourceAttemptID: source.ID, SourceCommit: item.CommitSHA, TargetCommit: strings.Repeat("e", 40), Repository: p.Repository, OwnerID: q.OwnerID, RoomID: q.RoomID, SourceRequestID: q.RequestID, InstructionsHash: p.InstructionsHash, ObservedAt: observed}
			encoded, _ := json.Marshal(inspection)
			job, err := s.taskSvc.SaveLaunch(ctx, task.TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, RequestID: inspection.RequestID(item.ID), AgentID: item.AssignedAgent, OwnerID: q.OwnerID, RoomID: q.RoomID, SourceRequestID: q.RequestID, InstructionsHash: p.InstructionsHash, Revision: "Scope: read-only repository. Do not push. Do not deploy.\n" + task.LiveInspectionMarker + string(encoded), CreatedAt: observed, AvailableAt: observed, ExpiresAt: observed.Add(time.Hour)})
			must(err)
			claimed, err := s.taskSvc.ClaimLaunch(ctx, time.Now(), "observation-fixture", job.ID)
			must(err)
			if claimed == nil {
				t.Fatal("observation fixture was not claimed")
			}
			observer, err := s.taskSvc.CreateAttempt(ctx, item.ID, q.OwnerID, task.CreateAttemptRequest{RunnerID: "observer", AgentName: item.AssignedAgent, RequestID: job.RequestID})
			must(err)
			end, exit := time.Now(), 1
			observer.Status, observer.StartedAt, observer.FinishedAt, observer.ExitCode = task.AttemptStatusFailed, &observed, &end, &exit
			// Direct fixture creation may occur after the observation was saved.
			observer.StartedAt = &observer.CreatedAt
			must(store.UpdateAttempt(ctx, observer))
			must(s.taskSvc.FinishLaunch(ctx, job.ID, "observation-fixture", "stopped", time.Now(), "Observation finished; source not published"))
			row, err := store.GetTask(ctx, item.ID)
			must(err)
			row.Status = task.TaskStatusBlocked
			must(store.UpdateTask(ctx, row))
			detail, err = s.taskSvc.GetTaskDetail(ctx, item.ID)
			must(err)
			if !s.recoverNativeReleaseHandoff(ctx, detail, []db.InboxRequest{q}) {
				t.Fatal("terminal observation did not return to source verification")
			}
			verification, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(item.ID, task.VerificationRequestID(item.ID, item.CommitSHA, source.ID)))
			must(err)
			proof, err := task.VerificationIntentForLaunch(&detail.Task, verification)
			must(err)
			if proof == nil || proof.SourceAttemptID != source.ID || capacityCalls.Load() != 0 {
				t.Fatal("observation replaced developer source or used PM model", proof)
			}
			if release, _ := s.taskSvc.GetLaunch(ctx, task.LaunchID(item.ID, requestID)); release != nil {
				t.Fatal("observation granted publication without source review")
			}
			return
		}
		if scenario == "schedule reviewed delivered" {
			s.cfg.Environment, s.cfg.SourceRepository, s.cfg.CommitSHA = "production", "owner/repo", item.CommitSHA
			observed := s.inboxReviewPublication(ctx, detail)
			if observed == nil || !observed.Verified {
				t.Fatal("fixture publication was not observed", observed)
			}
			if s.recoverNativeReleaseHandoff(ctx, detail, []db.InboxRequest{q}) {
				t.Fatal("verified publication scheduled another release")
			}
			launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(item.ID, requestID))
			if launch != nil || !errors.Is(err, task.ErrTaskNotFound) {
				t.Fatal("verified publication created a new execution identity", launch, err)
			}
			current, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
			must(err)
			if len(current.Attempts) != 2 || current.Status != task.TaskStatusReview || capacityCalls.Load() != 0 {
				t.Fatal("publication observation started work or accepted the task")
			}
			return
		}
		if scenario == "schedule reviewed" {
			before, err := repo.ListInboxRequests(ctx, q.RoomID)
			must(err)
			s.queueInboxTaskReview(ctx, detail)
			launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(item.ID, requestID))
			must(err)
			if launch == nil || launch.State != "started" {
				t.Fatal("approved source did not schedule publication", launch)
			}
			parsed, err := task.NativeReleaseIntentForLaunch(&detail.Task, launch)
			must(err)
			if parsed == nil || parsed.SourceReviewAttemptID != detail.LatestAttempt.ID || parsed.SourceAttemptID != source.ID {
				t.Fatal("publication lost source/reviewer binding", parsed)
			}
			s.queueInboxTaskReview(ctx, detail)
			after, err := repo.ListInboxRequests(ctx, q.RoomID)
			must(err)
			current, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
			must(err)
			if len(after) != len(before) || len(current.Attempts) != 3 || current.Status == task.TaskStatusDone || capacityCalls.Load() != 0 {
				t.Fatal("reviewed publication duplicated work, used AI or accepted task")
			}
			return
		}
		scheduled := false
		if scenario == "schedule" {
			before, err := repo.ListInboxRequests(ctx, q.RoomID)
			must(err)
			s.queueInboxTaskReview(ctx, detail)
			admitted, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(item.ID, task.VerificationRequestID(item.ID, item.CommitSHA, source.ID)))
			must(err)
			scheduled = admitted != nil
			after, err := repo.ListInboxRequests(ctx, q.RoomID)
			must(err)
			if len(after) != len(before) {
				t.Fatal("native scheduling created a PM model request")
			}
		} else {
			scheduled = s.recoverNativeReleaseHandoff(ctx, detail, []db.InboxRequest{q})
		}
		if scenario != "schedule" {
			if scheduled {
				t.Fatal("unapproved source scheduled native work")
			}
			current, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
			must(err)
			if len(current.Attempts) != 1 || current.Status != task.TaskStatusReview || capacityCalls.Load() != 0 {
				t.Fatal("invalid scheduling changed task or used AI")
			}
			return
		}
		if !scheduled {
			t.Fatal("canonical source did not schedule independent source verification")
		}
		// A reconstructed scan with stale source evidence cannot duplicate it.
		restarted := *s
		restarted.queueInboxTaskReview(ctx, detail)
		if release, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(item.ID, requestID)); release != nil || (err != nil && !errors.Is(err, task.ErrTaskNotFound)) {
			t.Fatal("unreviewed source reached publication", release, err)
		}
		verification, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(item.ID, task.VerificationRequestID(item.ID, item.CommitSHA, source.ID)))
		must(err)
		if verification == nil {
			t.Fatal("source verification was not durable")
		}
		return
	} else {
		output = s.startAuthorizedLaunch(ctx, q, detail, requestID, q.RequestID, task.NativeReleaseIntentMarker+string(raw))
	}
	launch, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(item.ID, requestID))
	if scenario == "foreign approval" {
		if launch != nil {
			t.Fatal("foreign approval persisted native execution intent")
		}
		current, loadErr := s.taskSvc.GetTaskDetail(ctx, item.ID)
		must(loadErr)
		if current.Status != task.TaskStatusReview || len(current.Attempts) != 2 || capacityCalls.Load() != 0 {
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
	if len(current.Attempts) != 3 || current.LatestAttempt.Status != task.AttemptStatusQueued || current.Status == task.TaskStatusDone || capacityCalls.Load() != 0 {
		t.Fatal("native admission used AI or accepted publication", capacityCalls.Load())
	}
	restarted := *s
	restarted.taskSvc = task.NewService(store, nil)
	restarted.startAuthorizedLaunch(ctx, q, detail, requestID, q.RequestID, task.NativeReleaseIntentMarker+string(raw))
	after, err := restarted.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	if len(after.Attempts) != 3 {
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
	var claim *runner.ClaimTaskResponse
	if scenario == "http" {
		payload, _ := json.Marshal(runner.ClaimTaskRequest{RunnerID: "native-runner", SupportedAgents: []string{"other-provider"}, SupportsNativeRelease: true})
		request := httptest.NewRequest(http.MethodPost, "/api/v1/runner/claim", bytes.NewReader(payload))
		response := httptest.NewRecorder()
		s.handleRunnerClaim(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("native HTTP claim failed: %d", response.Code)
		}
		claim = &runner.ClaimTaskResponse{}
		must(json.Unmarshal(response.Body.Bytes(), claim))
		if claim.TaskAgent != nil || claim.AgentPrompt != "" || claim.Environment != "" || claim.WorkingDir != "" || claim.RequiresVision || claim.AutoDeploy || claim.OwnerID != q.OwnerID {
			t.Fatal("native stage received generic model preparation")
		}
	} else {
		claim, err = s.runnerSvc.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "native-runner", SupportedAgents: []string{"other-provider"}, SupportsNativeRelease: true}, time.Minute)
	}
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
	if strings.HasPrefix(scenario, "initial") {
		verifyInitialNativePreparation(t, s, token, item, claim, p, preparedTarget, scenario)
	}
	if scenario == "prepare" {
		type preparationResult struct {
			workspace *project.PreparedNativeReleaseWorkspace
			err       error
		}
		results := make(chan preparationResult, 2)
		for i := 0; i < 2; i++ {
			go func() {
				workspace, err := s.prepareNativeReleaseWorkspace(ctx, item.ID, claim.Attempt.ID, claim.Attempt.RunnerID, preparedTarget)
				results <- preparationResult{workspace, err}
			}()
		}
		first, second := <-results, <-results
		must(first.err)
		must(second.err)
		prepared := first.workspace
		if prepared == nil || second.workspace == nil || second.workspace.Path != prepared.Path {
			t.Fatal("concurrent native preparation diverged")
		}
		if prepared == nil || prepared.SourceCommit != p.SourceCommit || prepared.TargetCommit != preparedTarget || prepared.ManifestHash != p.ManifestHash {
			t.Fatal("native preparation lost source chain")
		}
		again, err := s.prepareNativeReleaseWorkspace(ctx, item.ID, claim.Attempt.ID, claim.Attempt.RunnerID, preparedTarget)
		must(err)
		if again.Path != prepared.Path {
			t.Fatal("preparation was not reused")
		}
		fresh, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
		must(err)
		record, err := task.NativeReleasePreparationForAttempt(ctx, store, &fresh.Task, claim.Attempt.ID)
		must(err)
		if record.TargetCommit != preparedTarget || fresh.CommitSHA != p.SourceCommit || fresh.Status == task.TaskStatusDone {
			t.Fatal("preparation rewrote source or accepted task")
		}
		if _, err := s.prepareNativeReleaseWorkspace(ctx, item.ID, claim.Attempt.ID, claim.Attempt.RunnerID, p.SourceCommit); err == nil {
			t.Fatal("existing preparation rebound to another target")
		}
		t.Setenv("NATIVE_TRANSPORT_AMBIENT", "must-not-transfer")
		for _, invalid := range []string{"valid", "injected target", "wrong runner", "unauthenticated"} {
			payload := map[string]string{"task_id": item.ID, "runner_id": claim.Attempt.RunnerID}
			if invalid == "injected target" {
				payload["target_commit"] = p.SourceCommit
			}
			if invalid == "wrong runner" {
				payload["runner_id"] = "foreign"
			}
			body, _ := json.Marshal(payload)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/runner/attempts/"+claim.Attempt.ID+"/release/prepare", bytes.NewReader(body))
			if invalid != "unauthenticated" {
				request.Header.Set("Authorization", "Bearer "+token)
			}
			response := httptest.NewRecorder()
			s.Router().ServeHTTP(response, request)
			expected := 200
			switch invalid {
			case "injected target":
				expected = 400
			case "wrong runner":
				expected = 409
			case "unauthenticated":
				expected = 401
			}
			if response.Code != expected {
				t.Fatalf("native transport %s got %d want %d", invalid, response.Code, expected)
			}
			if invalid == "valid" {
				var execution runner.NativeReleaseExecution
				must(json.Unmarshal(response.Body.Bytes(), &execution))
				if execution.Workspace == nil || execution.Workspace.Path != prepared.Path || execution.Environment == nil || response.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("native preparation transport lost exact workspace")
				}
				selected := map[string]string{}
				for _, entry := range execution.Environment {
					parts := strings.SplitN(entry, "=", 2)
					if _, duplicate := selected[parts[0]]; duplicate {
						t.Fatal("duplicate native environment key", parts[0])
					}
					selected[parts[0]] = parts[1]
				}
				if selected["CUAN_NATIVE_RELEASE_SYNC_SHA"] != preparedTarget || selected["CUAN_NATIVE_RELEASE_SYNC_BRANCH"] != prepared.Branch {
					t.Fatal("native sync context retained project-supplied stale values")
				}
				if selected["NATIVE_TRANSPORT_AMBIENT"] != "" || selected["HOME"] != filepath.Join(prepared.Path, ".git", "cuan-native-runtime", "home") || selected["CUAN_DEPLOY_EXCLUDE_TASK"] != item.ID || selected["CUAN_DEPLOY_EXCLUDE_ATTEMPT"] != claim.Attempt.ID {
					t.Fatal("native runtime borrowed ambient identity or lost own-attempt binding")
				}
			}
		}
		source.CancelRequested = true
		must(store.UpdateAttempt(ctx, source))
		if _, err := s.prepareNativeReleaseWorkspace(ctx, item.ID, claim.Attempt.ID, claim.Attempt.RunnerID, preparedTarget); err == nil {
			t.Fatal("revoked source reused preparation")
		}
	}
	if strings.HasPrefix(scenario, "publication") {
		verifyNativePreparedPublication(t, s, token, item, source, claim, p, scenario)
	}
}

func verifyNativePreparedPublication(t *testing.T, s *Server, token string, item *task.Task, source *task.TaskAttempt, claim *runner.ClaimTaskResponse, intent task.NativeReleaseIntent, scenario string) {
	t.Helper()
	ctx := context.Background()
	store := s.taskSvc.Store()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(key)
	environment := "CUAN_MANAGED_RELEASE=1\nCUAN_NATIVE_RELEASE_SYNC_SHA=stale\nCUAN_NATIVE_RELEASE_SYNC_BRANCH=wrong\nCUAN_PUBLICATION_VERIFIER=ssh-release\nCUAN_PUBLICATION_SSH_HOST=project.invalid\nCUAN_PUBLICATION_SSH_USER=project-user\nCUAN_PUBLICATION_SSH_PASSWORD=private-password\nCUAN_PUBLICATION_SSH_HOST_KEY=" + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))) + "\nCUAN_PUBLICATION_DEPLOY_PATH=/var/www/project\nCUAN_PUBLICATION_HEALTH_URL=https://project.invalid/up"
	metadata, _ := json.Marshal(map[string]any{"cuan_settings": project.Settings{WorkspaceRevision: "initial", Environment: environment}})
	projects := project.NewMemoryStore()
	must(projects.CreateProject(ctx, &project.Project{ID: item.ProjectID, Name: "Project", RepoURL: "https://github.com/owner/repo.git", DefaultBranch: "dev", Metadata: metadata}))
	s.projectSvc = project.NewService(projects, nil, "fixture", t.TempDir())
	target := strings.Repeat("c", 40)
	receipt := task.NativeReleasePreparation{Version: 1, TaskID: item.ID, AttemptID: claim.Attempt.ID, SourceCommit: intent.SourceCommit, TargetCommit: target, InstructionsHash: intent.InstructionsHash, Repository: intent.Repository, TargetBranch: intent.TargetBranch, ManifestHash: intent.ManifestHash}
	raw, _ := json.Marshal(receipt)
	must(store.CreateActivity(ctx, &task.TaskActivity{ID: "native-release-preparation:" + claim.Attempt.ID, TaskID: item.ID, AttemptID: &claim.Attempt.ID, Author: "cuan-native-release", ActivityType: task.ActivityNativeReleasePreparation, Message: string(raw), CreatedAt: time.Now().UTC()}))
	processStarted := time.Now().UTC()
	calls := 0
	s.publicationVerifier = func(_ context.Context, c publication.Config, expected string) error {
		calls++
		if scenario == "publication no live proof" {
			return errors.New("fixture production observation unavailable")
		}
		if expected != target || c.Password != "private-password" {
			t.Error("observer checked wrong release target or environment")
		}
		if scenario == "publication revoked" {
			source.CancelRequested = true
			must(store.UpdateAttempt(ctx, source))
		}
		return nil
	}
	activationTime := time.Now().UTC().Truncate(time.Microsecond)
	if scenario == "publication activation receipt" {
		raw, _ := json.Marshal(task.DeploymentReceipt{Version: 1, TaskID: item.ID, AttemptID: claim.Attempt.ID, CommitSHA: target, ReleaseSHA: target, Environment: "production", Health: "healthy"})
		must(store.CreateActivity(ctx, &task.TaskActivity{ID: task.DeploymentReceiptID(item.ID, claim.Attempt.ID, target), TaskID: item.ID, AttemptID: &claim.Attempt.ID, ActivityType: "deployment_verified", Author: "cuan-deploy", Message: string(raw), CreatedAt: activationTime}))
	}
	processFinished := time.Now().UTC()
	expected := target
	if scenario == "publication wrong target" {
		expected = item.CommitSHA
	}
	fields := map[string]string{"task_id": item.ID, "runner_id": claim.Attempt.RunnerID, "commit_sha": expected}
	if scenario == "publication activation receipt" {
		fields["process_finished_at"] = processFinished.UTC().Format(time.RFC3339Nano)
	}
	payload, _ := json.Marshal(fields)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/runner/attempts/"+claim.Attempt.ID+"/publication/verify", bytes.NewReader(payload))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	s.Router().ServeHTTP(response, request)
	want := 200
	wantCalls := 1
	if scenario == "publication wrong target" || scenario == "publication revoked" {
		want = 409
	}
	if scenario == "publication no live proof" {
		want = 422
	}
	if scenario == "publication wrong target" {
		wantCalls = 0
	}
	if response.Code != want || calls != wantCalls {
		t.Fatalf("native publication observation got %d calls=%d; want %d/%d", response.Code, calls, want, wantCalls)
	}
	detail, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	must(err)
	if scenario == "publication activation receipt" {
		if !task.VerifiedPostProcessDeployment(detail.Activities, detail.LatestAttempt, target, processFinished, time.Now()) {
			t.Fatal("activation receipt suppressed final observation")
		}
		for _, a := range detail.Activities {
			if a.ID == task.DeploymentReceiptID(item.ID, claim.Attempt.ID, target) && !a.CreatedAt.Equal(activationTime) {
				t.Fatal("activation receipt rewritten")
			}
		}
		replay := httptest.NewRequest(http.MethodPost, "/api/v1/runner/attempts/"+claim.Attempt.ID+"/publication/verify", bytes.NewReader(payload))
		replay.Header.Set("Authorization", "Bearer "+token)
		result := httptest.NewRecorder()
		s.Router().ServeHTTP(result, replay)
		if result.Code != 200 || calls != 1 {
			t.Fatal("final observation not idempotent", result.Code, calls)
		}
	}
	verified := task.VerifiedDeployment(detail.Activities, detail.LatestAttempt, target, time.Now())
	if verified != (want == 200) || detail.CommitSHA != intent.SourceCommit || detail.Status == task.TaskStatusDone {
		t.Fatal("release observation rewrote developer source or accepted unfinished work")
	}
	if want == 200 || scenario == "publication no live proof" {
		s.runnerSvc.SetProjectFinder(s.projectSvc)
		process := &release.Result{Commit: target, Branch: intent.TargetBranch, StartedAt: processStarted, FinishedAt: processFinished, ExitCode: 0, SourceUnchanged: true}
		switch scenario {
		case "publication missing process":
			process = nil
		case "publication stale observation":
			process.FinishedAt = time.Now().UTC()
		case "publication wrong process target":
			process.Commit = intent.SourceCommit
		case "publication mutated checkout":
			process.SourceUnchanged = false
		}
		completed, err := s.runnerSvc.Complete(ctx, claim.Attempt.ID, runner.CompleteAttemptRequest{RunnerID: claim.Attempt.RunnerID, Status: task.AttemptStatusSucceeded, ExitCode: 0, NativeReleaseProcess: process})
		must(err)
		current, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
		must(err)
		historical, historyErr := task.HistoricalNativeReleasePreparationForAttempt(ctx, store, &current.Task, claim.Attempt.ID)
		must(historyErr)
		if historical == nil || historical.SourceCommit != intent.SourceCommit || historical.TargetCommit != target {
			t.Fatal("terminal preparation history lost source/target")
		}
		if _, activeErr := task.NativeReleasePreparationForAttempt(ctx, store, &current.Task, claim.Attempt.ID); activeErr == nil {
			t.Fatal("historical read reopened native preparation authority")
		}
		inspectionProof := &task.LiveInspectionIntent{Purpose: task.CurrentOutcomeInspection, PreparedPublicationAttemptID: claim.Attempt.ID, SourceCommit: intent.SourceCommit, Repository: intent.Repository}
		proofErr := task.ValidatePreparedPublicationInspection(ctx, store, &current.Task, inspectionProof)
		if (proofErr == nil) != (want == 200) {
			t.Fatal("prepared publication history did not require actual deployment observation", proofErr)
		}
		if want == 200 {
			wrong := *inspectionProof
			wrong.Repository = "foreign/repo"
			if task.ValidatePreparedPublicationInspection(ctx, store, &current.Task, &wrong) == nil {
				t.Fatal("foreign preparation accepted")
			}
			wrong = *inspectionProof
			wrong.SourceCommit = strings.Repeat("f", 40)
			if task.ValidatePreparedPublicationInspection(ctx, store, &current.Task, &wrong) == nil {
				t.Fatal("wrong historical source accepted")
			}
		}
		projection := s.inboxNativePreparationHistory(ctx, current)
		encoded, _ := json.Marshal(projection)
		if projection == nil || !strings.Contains(string(encoded), "\"attempt_status\":\""+completed.Status+"\"") || !strings.Contains(string(encoded), "\"preparation_only\":true") {
			t.Fatal("history projection lost actual terminal result", string(encoded))
		}
		changed := current.Task
		changed.Instructions += " changed scope"
		if _, changedErr := task.HistoricalNativeReleasePreparationForAttempt(ctx, store, &changed, claim.Attempt.ID); changedErr == nil {
			t.Fatal("history accepted changed original scope")
		}
		deliveries := 0
		for _, event := range current.Activities {
			if event.ActivityType == "native_release_delivery" {
				deliveries++
				var delivery task.NativeReleaseDelivery
				must(json.Unmarshal([]byte(event.Message), &delivery))
				if delivery.SourceCommit != intent.SourceCommit || delivery.TargetCommit != target || delivery.SourceAttemptID != source.ID {
					t.Fatal("native delivery lost source lineage")
				}
			}
		}
		if scenario == "publication" || scenario == "publication activation receipt" {
			delivery, err := task.NativeReleaseDeliveryForAttempt(ctx, store, &current.Task, claim.Attempt.ID)
			must(err)
			if delivery == nil || delivery.SourceCommit != intent.SourceCommit || delivery.TargetCommit != target {
				t.Fatal("completed native delivery cannot be read for verification")
			}
			if completed.Status != task.AttemptStatusSucceeded || current.Status != task.TaskStatusReview || deliveries != 1 {
				t.Fatal("native delivery did not await independent verification")
			}
		} else if completed.Status == task.AttemptStatusSucceeded || deliveries != 0 {
			t.Fatal("invalid native process accepted")
		}
		if current.CommitSHA != intent.SourceCommit || current.Status == task.TaskStatusDone {
			t.Fatal("native completion rewrote source or accepted task")
		}
	}
}

func nativeReleaseProjectFixture(t *testing.T, s *Server, tools ...string) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.Mkdir(filepath.Join(root, ".cuan"), 0700))
	manifest := `{"version":1,"command":["release.sh"],"timeout_seconds":5}`
	if len(tools) > 0 {
		raw, _ := json.Marshal(map[string]any{"version": 1, "command": []string{"release.sh"}, "timeout_seconds": 5, "required_tools": tools})
		manifest = string(raw)
	}
	for name, body := range map[string]string{".cuan/release.json": manifest, "release.sh": "#!/bin/sh\ntouch executed\n", "work.txt": "source"} {
		must(os.WriteFile(filepath.Join(root, name), []byte(body), 0700))
	}
	run := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("fixture git: %s", out)
		}
		return strings.TrimSpace(string(out))
	}
	run("init", "--initial-branch=dev")
	run("add", ".")
	run("-c", "user.name=Fixture Human", "-c", "user.email=human@example.invalid", "commit", "-m", "Source")
	source := run("rev-parse", "HEAD")
	must(os.WriteFile(filepath.Join(root, "work.txt"), []byte("target"), 0700))
	run("add", ".")
	run("-c", "user.name=Fixture Human", "-c", "user.email=human@example.invalid", "commit", "-m", "Target")
	target := run("rev-parse", "HEAD")
	_, key, _ := ed25519.GenerateKey(rand.Reader)
	signer, _ := ssh.NewSignerFromKey(key)
	environment := "CUAN_MANAGED_RELEASE=1\nCUAN_NATIVE_RELEASE_SYNC_SHA=stale\nCUAN_NATIVE_RELEASE_SYNC_BRANCH=wrong\nCUAN_PUBLICATION_VERIFIER=ssh-release\nCUAN_PUBLICATION_SSH_HOST=project.invalid\nCUAN_PUBLICATION_SSH_USER=fixture\nCUAN_PUBLICATION_SSH_PASSWORD=fixture-password\nCUAN_PUBLICATION_SSH_HOST_KEY=" + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))) + "\nCUAN_PUBLICATION_DEPLOY_PATH=/var/www/project\nCUAN_PUBLICATION_HEALTH_URL=https://project.invalid/up"
	metadata, _ := json.Marshal(map[string]any{"cuan_settings": project.Settings{WorkspaceRevision: "initial", Environment: environment}})
	projects := project.NewMemoryStore()
	must(projects.CreateProject(context.Background(), &project.Project{ID: "default", Name: "Project", RepoURL: "https://github.com/owner/repo.git", DefaultBranch: "dev", LocalPath: root, Metadata: metadata}))
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.Contains(r.URL.Path, "/commits/") || r.Header.Get("Authorization") != "Bearer saved-owner" {
			t.Error("unexpected saved branch observation")
			w.WriteHeader(400)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"sha": target})
	}))
	t.Cleanup(api.Close)
	client := git.NewClient()
	client.SetAPIBaseURL(api.URL)
	s.projectSvc = project.NewService(projects, client, "fixture", t.TempDir())
	s.projectSvc.SetSharedRepositoryAccess(func(context.Context) (string, error) { return "saved-owner", nil })
	digest := sha256.Sum256([]byte(manifest))
	return source, target, hex.EncodeToString(digest[:])
}
