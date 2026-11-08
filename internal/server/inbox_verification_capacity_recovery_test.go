package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/task"
)

func TestVerificationCapacityRecovery(t *testing.T) {
	for _, scenario := range []string{"ready", "recovery quota", "no account", "busy account", "cancelled", "wrong owner", "changed source", "offline runner"} {
		t.Run(scenario, func(t *testing.T) {
			s, d, q := stalledReviewFixture(t, "completed")
			verificationCapacityRecoveryContract(t, s, d, q, scenario)
		})
	}
}

func TestVerificationCapacityRecoveryPostgres(t *testing.T) {
	withPhasePostgres(t, func(database *sql.DB) {
		s, q, d := phaseLineageSetup(t, database)
		verificationCapacityRecoveryContract(t, s, d, q, "ready")
	})
}

func verificationCapacityRecoveryContract(t *testing.T, s *Server, d *task.TaskDetailResponse, q db.InboxRequest, scenario string) {
	t.Helper()
	ctx := context.Background()
	verificationFollowupQueueContract(t, s, d, q)
	store := s.taskSvc.Store()
	_ = store.DeleteTask(ctx, "busy")
	_ = store.DeleteTask(ctx, "phase-blocker")
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	job, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, task.VerificationRequestID(d.ID, d.CommitSHA, d.LatestAttempt.ID)))
	if err != nil {
		t.Fatal(err)
	}
	claimedJob, err := store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "capacity-original", job.ID)
	if err != nil {
		t.Fatal(err)
	}
	a, err := s.taskSvc.CreateAttempt(ctx, d.ID, "owner", task.CreateAttemptRequest{RunnerID: "verifier", AgentName: d.AssignedAgent, RequestID: job.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	runnerStore := runner.NewMemoryStore(store)
	s.runnerSvc = runner.NewService(runnerStore, store, nil)
	if _, err := s.runnerSvc.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "verifier"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := s.runnerSvc.Complete(ctx, a.ID, runner.CompleteAttemptRequest{RunnerID: "verifier", Status: task.AttemptStatusFailed, ExitCode: 75, ErrorMessage: "The AI provider reported a usage limit. The run was stopped; saved work is preserved."}); err != nil {
		t.Fatal(err)
	}
	if err := s.taskSvc.FinishLaunch(ctx, job.ID, claimedJob.LeaseToken, "stopped", time.Now().UTC(), ""); err != nil {
		t.Fatal(err)
	}
	d, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	parent, commit := d.LatestAttempt.ID, d.CommitSHA
	prior, err := task.VerificationIntentForAttempt(ctx, store, &d.Task, parent)
	if err != nil || prior == nil {
		t.Fatal("missing verifier", err)
	}
	source := prior.SourceAttemptID
	var origin db.InboxRequest
	rows, _ := s.projectStudioRepo.(db.InboxRequestRepository).ListInboxRequests(ctx, d.SourceRoomID)
	for _, row := range rows {
		if row.MessageID == d.SourceMessageID {
			origin = row
		}
	}
	if origin.RequestID == "" {
		t.Fatal("missing source authorization")
	}
	if scenario != "offline runner" {
		if _, err := s.runnerSvc.Heartbeat(ctx, runner.HeartbeatRequest{RunnerID: "verifier"}, time.Minute); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := runnerStore.RegisterOrUpdateHeartbeat(ctx, &runner.RunnerInfo{ID: "verifier", Status: runner.StatusOffline}); err != nil {
			t.Fatal(err)
		}
	}
	if scenario != "no account" {
		account, err := agent.CreateProviderAccount(origin.OwnerID, "fixture")
		if err != nil {
			t.Fatal(err)
		}
		home, _ := agent.ProviderAccountHome(origin.OwnerID, account.ID)
		if err := os.MkdirAll(agent.AntigravityConfigDirPath(home), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(agent.AntigravityCredentialsFilePath(home), []byte("fixture-only-credentials"), 0600); err != nil {
			t.Fatal(err)
		}
		metaPath := filepath.Join(agent.ConfigHome(), ".local", "share", "cuan", "accounts", account.ID, "account.json")
		raw, _ := os.ReadFile(metaPath)
		var meta map[string]any
		json.Unmarshal(raw, &meta)
		meta["state"] = "verified"
		raw, _ = json.Marshal(meta)
		os.WriteFile(metaPath, raw, 0600)
		if scenario == "busy account" {
			t.Setenv("CUAN_ACCOUNT_MAX_SESSIONS", "1")
			release, err := agent.TryLeaseProviderAccount(origin.OwnerID, account.ID)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
		}
	}
	if scenario == "cancelled" {
		d.Status = "cancelled"
	}
	if scenario == "wrong owner" {
		origin.OwnerID = "another-owner"
	}
	if scenario == "changed source" {
		d.CommitSHA = strings.Repeat("c", 40)
	}
	if err := store.CreateTask(ctx, &task.Task{ID: "capacity-busy", ProjectID: d.ProjectID, Status: task.TaskStatusDoing, ExecutionCategory: task.ExecutionCategoryCode}); err != nil {
		t.Fatal(err)
	}
	before := len(d.Attempts)
	want := scenario == "ready" || scenario == "recovery quota"
	if got := s.recoverCapacityVerification(ctx, d, origin); got != want {
		t.Fatalf("recovery=%v scenario=%s", got, scenario)
	}
	if !want {
		return
	}
	for i := 0; i < 2; i++ {
		if !s.recoverCapacityVerification(ctx, d, origin) {
			t.Fatal("lost idempotent pending recovery")
		}
	}
	queued, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, task.VerificationRecoveryRequestID(d.ID, commit)))
	if err != nil || queued.State != "queued" {
		t.Fatal("missing durable recovery", err)
	}
	proof, err := task.VerificationIntentForLaunch(&d.Task, queued)
	if err != nil || proof.RecoveryCause != "capacity" || proof.RecoveryAttemptID != parent || proof.SourceAttemptID != source || proof.SourceCommit != commit || proof.RuntimeFingerprint != "" {
		t.Fatal("recovery changed source or cause", err)
	}
	current, _ := s.taskSvc.GetTaskDetail(ctx, d.ID)
	if err := task.ValidateVerificationCapacityRecovery(ctx, store, &d.Task, proof, current.Activities); err != nil {
		t.Fatal("native admission receipt lost", err)
	}
	for _, fault := range []string{"agent receipt", "wrong owner receipt", "future receipt", "old receipt", "missing receipt"} {
		var events []*task.TaskActivity
		for _, event := range current.Activities {
			copyEvent := *event
			if event.ActivityType == task.ActivityVerificationCapacityRecovery {
				if fault == "missing receipt" {
					continue
				}
				var receipt task.VerificationCapacity
				json.Unmarshal([]byte(event.Message), &receipt)
				switch fault {
				case "agent receipt":
					copyEvent.Author = "agent"
				case "wrong owner receipt":
					receipt.OwnerID = "different"
				case "future receipt":
					receipt.CheckedAt = time.Now().Add(time.Hour)
				case "old receipt":
					receipt.CheckedAt = d.LatestAttempt.FinishedAt.Add(-time.Second)
				}
				raw, _ := json.Marshal(receipt)
				copyEvent.Message = string(raw)
			}
			events = append(events, &copyEvent)
		}
		if task.ValidateVerificationCapacityRecovery(ctx, store, &d.Task, proof, events) == nil {
			t.Fatal("invalid admission evidence accepted", fault)
		}
	}
	if len(current.Attempts) != before || current.CommitSHA != commit {
		t.Fatal("queue bypassed lane or changed result")
	}
	if err := store.DeleteTask(ctx, "capacity-busy"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.(task.LaunchStore).ClaimLaunch(ctx, time.Now(), "capacity-recovery", queued.ID); err != nil {
		t.Fatal(err)
	}
	recovered, err := s.taskSvc.CreateAttempt(ctx, d.ID, "owner", task.CreateAttemptRequest{RunnerID: "verifier", AgentName: d.AssignedAgent, RequestID: queued.RequestID})
	if err != nil {
		t.Fatal(err)
	}
	claim, err := s.runnerSvc.Claim(ctx, runner.ClaimTaskRequest{RunnerID: "verifier"}, time.Minute)
	if err != nil || !claim.Claimed || claim.Verification == nil || !claim.PublicationLimits.ReadOnlyRepository || !claim.PublicationLimits.DenyDeploy || !claim.PublicationLimits.DenyPush {
		t.Fatal("claim lost original verifier scope", err)
	}
	completion := runner.CompleteAttemptRequest{RunnerID: "verifier", Status: task.AttemptStatusSucceeded, ExitCode: 0, VerifiedSourceCommit: commit, AgentReportRequired: true, AgentReportConfirmed: true, RepositoryVerification: &runner.RepositoryVerification{Baseline: strings.Repeat("b", 64), Final: strings.Repeat("b", 64)}}
	if scenario == "recovery quota" {
		completion.Status = task.AttemptStatusFailed
		completion.ExitCode = 75
		completion.ErrorMessage = "The AI provider reported a usage limit. The run was stopped; saved work is preserved."
	}
	_, err = s.runnerSvc.Complete(ctx, recovered.ID, completion)
	if err != nil {
		t.Fatal(err)
	}
	current, _ = s.taskSvc.GetTaskDetail(ctx, d.ID)
	old, _ := store.GetAttempt(ctx, parent)
	expectedStatus := task.TaskStatusReview
	if scenario == "recovery quota" {
		expectedStatus = task.TaskStatusBlocked
	}
	if old.Status != task.AttemptStatusFailed || current.Status != expectedStatus || current.LatestAttempt.ID != recovered.ID {
		t.Fatal("recovery rewrote history or bypassed review")
	}
	if s.recoverCapacityVerification(ctx, current, origin) {
		t.Fatal("chained capacity recovery")
	}
}
