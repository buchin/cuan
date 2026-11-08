package task

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func nativeReleaseFixture(t *testing.T) (*MemoryStore, *Task, *TaskAttempt, NativeReleaseIntent, TaskLaunch) {
	t.Helper()
	ctx := context.Background()
	store := NewMemoryStore()
	now := time.Now().UTC()
	exit := 0
	item := &Task{ID: "release-task", ProjectID: "project", SourceRoomID: "room", SourceMessageID: "approved-message", AssignedAgent: "agent_cuan", Title: "Implement approved work", Instructions: "Cuan managed delegation v1.\nImplement and deploy approved work.", Status: TaskStatusReview, ExecutionCategory: ExecutionCategoryCode, CommitSHA: strings.Repeat("a", 40), Summary: "Source ready", CompleteNote: "Actual targeted checks passed; release pending", CreatedAt: now.Add(-3 * time.Minute), UpdatedAt: now}
	start, finish := now.Add(-2*time.Minute), now.Add(-time.Minute)
	source := &TaskAttempt{ID: "source", TaskID: item.ID, RunnerID: "source-runner", AgentName: "agy", Status: AttemptStatusSucceeded, ExitCode: &exit, CreatedAt: start.Add(-time.Second), StartedAt: &start, FinishedAt: &finish}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(store.CreateTask(ctx, item))
	must(store.CreateAttempt(ctx, source))
	summary := "Project verification: test.sh --fast (exit 0)"
	proof := NativeReleaseIntent{SourceAttemptID: source.ID, SourceCommit: item.CommitSHA, InstructionsHash: LaunchInstructionsHash(item.Instructions), SourceVerificationHash: VerificationSummaryHash(summary), OwnerID: "owner", RoomID: item.SourceRoomID, SourceRequestID: "approved-request", Repository: "owner/repo", TargetBranch: "dev", ManifestHash: strings.Repeat("b", 64)}
	checks, _ := json.Marshal(RunnerVerificationRecord{Summary: summary, Changes: NewRunnerChangeRecord(item.CommitSHA, "source.go | 1 +")})
	handoff, _ := json.Marshal(NativeReleaseHandoff{Version: 1, TaskID: item.ID, AttemptID: source.ID, SourceCommit: proof.SourceCommit, InstructionsHash: proof.InstructionsHash, VerificationHash: proof.SourceVerificationHash, Repository: proof.Repository, TargetBranch: proof.TargetBranch, ManifestHash: proof.ManifestHash})
	for _, a := range []*TaskActivity{{ID: "report", TaskID: item.ID, AttemptID: &source.ID, ActivityType: "agent_report", NewStatus: TaskStatusReview, Message: "Actual code handoff", CreatedAt: start}, {ID: "kind", TaskID: item.ID, AttemptID: &source.ID, ActivityType: "agent_result_kind", NewStatus: "implementation", CreatedAt: start}, {ID: "verification:" + source.ID, TaskID: item.ID, AttemptID: &source.ID, Author: source.RunnerID, ActivityType: ActivityTypeRunnerVerification, Message: string(checks), CreatedAt: finish}, {ID: "native-release-handoff:" + source.ID, TaskID: item.ID, AttemptID: &source.ID, Author: source.RunnerID, ActivityType: ActivityNativeReleaseHandoff, Message: string(handoff), CreatedAt: finish}} {
		must(store.CreateActivity(ctx, a))
	}
	reviewProof := VerificationIntent{IsolatedWorkspace: true, SourceAttemptID: source.ID, SourceCommit: item.CommitSHA, InstructionsHash: proof.InstructionsHash, OwnerID: proof.OwnerID, RoomID: proof.RoomID, SourceRequestID: proof.SourceRequestID, AssessmentRequestID: "review-request", SourceVerificationHash: proof.SourceVerificationHash}
	reviewRequest := VerificationRequestID(item.ID, item.CommitSHA)
	reviewRaw, _ := json.Marshal(reviewProof)
	reviewLaunch, err := store.SaveLaunch(ctx, TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, RequestID: reviewRequest, AgentID: item.AssignedAgent, OwnerID: proof.OwnerID, RoomID: proof.RoomID, SourceRequestID: proof.SourceRequestID, InstructionsHash: proof.InstructionsHash, Revision: VerificationIntentMarker + string(reviewRaw), CreatedAt: finish, AvailableAt: finish, ExpiresAt: now.Add(time.Hour)})
	must(err)
	_, err = store.ClaimLaunch(ctx, now, "review-fixture", reviewLaunch.ID)
	must(err)
	reviewStart, reviewFinish := finish.Add(100*time.Millisecond), finish.Add(500*time.Millisecond)
	reviewID := attemptRequestID(item.ID, reviewRequest)
	must(store.CreateAttempt(ctx, &TaskAttempt{ID: reviewID, TaskID: item.ID, RunnerID: "independent-reviewer", AgentName: item.AssignedAgent, Status: AttemptStatusSucceeded, ExitCode: &exit, CreatedAt: finish.Add(50 * time.Millisecond), StartedAt: &reviewStart, FinishedAt: &reviewFinish}))
	must(store.FinishLaunch(ctx, reviewLaunch.ID, "review-fixture", "stopped", now, ""))
	report := "Independent source checks passed; publication pending"
	assessment, _ := json.Marshal(map[string]any{"instructions_hash": proof.InstructionsHash, "assessment": map[string]string{"source_review": "pass"}})
	reviewReceipt := NativeSourceReview{Version: 1, TaskID: item.ID, ReviewAttemptID: reviewID, SourceAttemptID: source.ID, SourceCommit: item.CommitSHA, InstructionsHash: proof.InstructionsHash, ReportHash: VerificationSummaryHash(report), AssessmentHash: VerificationSummaryHash(string(assessment))}
	receiptRaw, _ := json.Marshal(reviewReceipt)
	for _, event := range []*TaskActivity{
		{ID: "review-report", TaskID: item.ID, AttemptID: &reviewID, ActivityType: "agent_report", NewStatus: TaskStatusReview, Message: report, CreatedAt: reviewFinish},
		{ID: "verification-source:" + reviewID, TaskID: item.ID, AttemptID: &reviewID, ActivityType: "verification_source_verified", Author: "independent-reviewer", Message: item.CommitSHA, CreatedAt: reviewFinish},
		{ID: "repository-scope:" + reviewID, TaskID: item.ID, AttemptID: &reviewID, ActivityType: "repository_scope_verified", Author: "independent-reviewer", NewStatus: "read_only", CreatedAt: reviewFinish},
		{ID: "review-assessment", TaskID: item.ID, AttemptID: &reviewID, ActivityType: "coordinator_review_assessment", Author: "coordinator", Message: string(assessment), CreatedAt: reviewFinish.Add(time.Millisecond)},
		{ID: "native-source-review:" + reviewID, TaskID: item.ID, AttemptID: &reviewID, ActivityType: ActivityNativeSourceReview, Author: "coordinator", Message: string(receiptRaw), CreatedAt: reviewFinish.Add(2 * time.Millisecond)},
	} {
		must(store.CreateActivity(ctx, event))
	}
	proof.SourceReviewAttemptID = reviewID
	raw, _ := json.Marshal(proof)
	j := TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, RequestID: NativeReleaseRequestID(item.ID, item.CommitSHA), OwnerID: proof.OwnerID, RoomID: proof.RoomID, SourceRequestID: proof.SourceRequestID, AgentID: item.AssignedAgent, InstructionsHash: proof.InstructionsHash, Revision: NativeReleaseIntentMarker + string(raw), CreatedAt: now, AvailableAt: now, ExpiresAt: now.Add(time.Hour)}
	return store, item, source, proof, j
}

func TestLegacyNativeReleaseEvidenceCannotAuthorizeFreshPublication(t *testing.T) {
	store, item, _, proof, _ := nativeReleaseFixture(t)
	proof.SourceReviewAttemptID = ""
	if ValidateNativeReleaseSource(context.Background(), store, item, &proof) == nil {
		t.Fatal("legacy unreviewed source admitted for release")
	}
	if err := validateNativeReleaseSource(context.Background(), store, item, &proof, false); err != nil {
		t.Fatal("legacy historical evidence became unreadable", err)
	}
}

func TestNativeDeveloperHandoffRecoversOnlyAfterBoundTerminalObservation(t *testing.T) {
	for _, scenario := range []string{"valid", "running", "cancelled", "wrong source", "wrong commit", "no source checks", "no deploy"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			store, item, source, release, _ := nativeReleaseFixture(t)
			release.SourceReviewAttemptID = ""
			now := time.Now().UTC()
			assessment, _ := json.Marshal(map[string]any{"request_id": "source-assessment", "instructions_hash": release.InstructionsHash, "evidence_digest": strings.Repeat("c", 64), "assessment": map[string]string{"decision": "revise", "remaining_work": "verification"}})
			if err := store.CreateActivity(ctx, &TaskActivity{ID: "source-assessment", TaskID: item.ID, AttemptID: &source.ID, ActivityType: "coordinator_review_assessment", Author: "coordinator", Message: string(assessment), CreatedAt: now.Add(-3 * time.Second)}); err != nil {
				t.Fatal(err)
			}
			inspection := LiveInspectionIntent{Purpose: CurrentOutcomeInspection, SourceAttemptID: source.ID, SourceCommit: item.CommitSHA, TargetCommit: strings.Repeat("d", 40), Repository: release.Repository, OwnerID: release.OwnerID, RoomID: release.RoomID, SourceRequestID: release.SourceRequestID, InstructionsHash: release.InstructionsHash, ObservedAt: now.Add(-2 * time.Second)}
			if scenario == "wrong source" {
				inspection.SourceAttemptID = "foreign-source"
			}
			if scenario == "wrong commit" {
				inspection.SourceCommit = strings.Repeat("e", 40)
			}
			raw, _ := json.Marshal(inspection)
			request := inspection.RequestID(item.ID)
			job, err := store.SaveLaunch(ctx, TaskLaunch{TaskID: item.ID, ProjectID: item.ProjectID, AgentID: item.AssignedAgent, RequestID: request, OwnerID: release.OwnerID, RoomID: release.RoomID, SourceRequestID: release.SourceRequestID, InstructionsHash: release.InstructionsHash, Revision: "Scope: read-only repository. Do not push. Do not deploy.\n" + LiveInspectionMarker + string(raw), CreatedAt: inspection.ObservedAt, AvailableAt: inspection.ObservedAt, ExpiresAt: now.Add(time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.ClaimLaunch(ctx, now, "inspection", job.ID); err != nil {
				t.Fatal(err)
			}
			start, finish, exit := now.Add(-time.Second), now.Add(-time.Millisecond), 1
			observer := &TaskAttempt{ID: attemptRequestID(item.ID, request), TaskID: item.ID, AgentName: item.AssignedAgent, RunnerID: "observer", Status: AttemptStatusFailed, ExitCode: &exit, CreatedAt: inspection.ObservedAt, StartedAt: &start, FinishedAt: &finish}
			if scenario == "running" {
				observer.Status, observer.FinishedAt = AttemptStatusRunning, nil
			}
			observer.CancelRequested = scenario == "cancelled"
			if err := store.CreateAttempt(ctx, observer); err != nil {
				t.Fatal(err)
			}
			if scenario == "no source checks" {
				source.CancelRequested = true
				if err := store.UpdateAttempt(ctx, source); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "no deploy" {
				item.Instructions += "\nDo not deploy."
			}
			if err := ValidateNativeDeveloperHandoff(ctx, store, item, &release); (err == nil) != (scenario == "valid") {
				t.Fatal("unexpected observation-to-source verification eligibility", scenario, err)
			}
			if ValidateNativeReleaseSource(ctx, store, item, &release) == nil {
				t.Fatal("observation recovery granted publication without independent source review")
			}
		})
	}
}

func TestNativeReleaseReviewUpgradeKeepsLegacyIntentHistorySeparate(t *testing.T) {
	_, item, _, proof, launch := nativeReleaseFixture(t)
	legacyID := verificationRequestID("native-release:v1:", item.ID, proof.SourceCommit)
	if legacyID == NativeReleaseRequestID(item.ID, proof.SourceCommit) {
		t.Fatal("reviewed source reused legacy execution identity")
	}
	launch.RequestID = legacyID
	if _, err := NativeReleaseIntentForLaunch(item, &launch); err == nil {
		t.Fatal("review approval reused a legacy launch key")
	}
	proof.SourceReviewAttemptID = ""
	raw, _ := json.Marshal(proof)
	launch.Revision = NativeReleaseIntentMarker + string(raw)
	if historical, err := NativeReleaseIntentForLaunch(item, &launch); err != nil || historical == nil {
		t.Fatal("legacy intent history unreadable", err)
	}
}

func TestNativeReleaseSourceRequiresExactCurrentBackendHandoff(t *testing.T) {
	for _, scenario := range []string{"valid", "unreviewed source", "self review", "cancelled review", "missing review approval", "wrong owner", "wrong approval", "failed source", "cancelled source", "future source", "missing handoff", "agent handoff", "wrong receipt identity", "changed commit", "changed scope", "wrong repository", "wrong target", "wrong manifest", "failed checks", "wrong check author", "newer execution", "no deploy"} {
		t.Run(scenario, func(t *testing.T) {
			store, item, source, p, _ := nativeReleaseFixture(t)
			ctx := context.Background()
			switch scenario {
			case "unreviewed source":
				p.SourceReviewAttemptID = ""
			case "wrong owner":
				p.OwnerID = "foreign-owner"
			case "wrong approval":
				p.SourceRequestID = "foreign-request"
			case "self review":
				p.SourceReviewAttemptID = source.ID
			case "cancelled review":
				review, err := store.GetAttempt(ctx, p.SourceReviewAttemptID)
				if err != nil {
					t.Fatal(err)
				}
				review.CancelRequested = true
				store.UpdateAttempt(ctx, review)
			case "failed source":
				source.Status = AttemptStatusFailed
				store.mu.Lock()
				copy := *source
				store.attempts[source.ID] = &copy
				store.mu.Unlock()
			case "cancelled source":
				source.CancelRequested = true
				store.UpdateAttempt(ctx, source)
			case "future source":
				future := time.Now().Add(time.Hour)
				source.FinishedAt = &future
				store.UpdateAttempt(ctx, source)
			case "changed commit":
				item.CommitSHA = strings.Repeat("c", 40)
				store.UpdateTask(ctx, item)
			case "changed scope":
				item.Instructions += "\nChanged approval"
				store.UpdateTask(ctx, item)
			case "wrong repository":
				p.Repository = "foreign/repo"
			case "wrong target":
				p.TargetBranch = "main"
			case "wrong manifest":
				p.ManifestHash = strings.Repeat("c", 64)
			case "newer execution":
				store.CreateAttempt(ctx, &TaskAttempt{ID: "newer", TaskID: item.ID, Status: AttemptStatusFailed, CreatedAt: time.Now()})
			case "no deploy":
				item.Instructions += "\nDo not deploy."
				p.InstructionsHash = LaunchInstructionsHash(item.Instructions)
				store.UpdateTask(ctx, item)
			}
			store.mu.Lock()
			for _, a := range store.activities {
				if a.TaskID != item.ID {
					continue
				}
				if scenario == "missing review approval" && a.ActivityType == ActivityNativeSourceReview {
					a.Author = "agent_cuan"
				}
				if a.ActivityType == ActivityNativeReleaseHandoff {
					if scenario == "no deploy" {
						var r NativeReleaseHandoff
						json.Unmarshal([]byte(a.Message), &r)
						r.InstructionsHash = p.InstructionsHash
						raw, _ := json.Marshal(r)
						a.Message = string(raw)
					}
					switch scenario {
					case "missing handoff":
						a.ActivityType = "comment"
					case "agent handoff":
						a.Author = "agent_cuan"
					case "wrong receipt identity":
						a.ID = "some-report"
					}
				}
				if a.ID == "verification:"+source.ID {
					switch scenario {
					case "failed checks":
						a.Message = `{"summary":"Project verification: test.sh --fast (exit 1)"}`
					case "wrong check author":
						a.Author = "agent_cuan"
					}
				}
			}
			store.mu.Unlock()
			err := ValidateNativeReleaseSource(ctx, store, item, &p)
			if (err == nil) != (scenario == "valid") {
				t.Fatalf("source validation=%v", err)
			}
		})
	}
}

func TestNativeReleaseIntentRejectsMalformedAndMismatchedLaunch(t *testing.T) {
	for _, scenario := range []string{"valid", "duplicate", "unknown version", "trailing json", "unknown field", "foreign owner", "foreign room", "wrong request", "wrong instructions", "mixed intents"} {
		t.Run(scenario, func(t *testing.T) {
			_, item, _, _, j := nativeReleaseFixture(t)
			switch scenario {
			case "duplicate":
				j.Revision += "\n" + j.Revision
			case "unknown version":
				j.Revision = strings.Replace(j.Revision, "intent v1:", "intent v2:", 1)
			case "trailing json":
				j.Revision += " {}"
			case "unknown field":
				j.Revision = strings.Replace(j.Revision, "{", "{\"bypass\":true,", 1)
			case "foreign owner":
				j.OwnerID = "other"
			case "foreign room":
				j.RoomID = "other"
			case "wrong request":
				j.RequestID = "other"
			case "wrong instructions":
				j.InstructionsHash = "other"
			case "mixed intents":
				j.Revision += "\n" + VerificationIntentMarker + "{}"
			}
			p, err := NativeReleaseIntentForLaunch(item, &j)
			if (err == nil && p != nil) != (scenario == "valid") {
				t.Fatalf("intent validation=%v", err)
			}
		})
	}
}

func TestNativeReleaseAdmissionRequiresClaimedLaunchAndExactPredecessor(t *testing.T) {
	ctx := context.Background()
	store, item, _, p, j := nativeReleaseFixture(t)
	svc := NewService(store, nil)
	saved, err := svc.SaveLaunch(ctx, j)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateAttempt(ctx, item.ID, "owner", CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: j.AgentID, RequestID: j.RequestID}); err == nil {
		t.Fatal("release admitted without predecessor binding")
	}
	if _, err := svc.CreateAttempt(ctx, item.ID, "owner", CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: j.AgentID, RequestID: j.RequestID, ExpectedPredecessorID: p.SourceReviewAttemptID}); err == nil {
		t.Fatal("unclaimed launch admitted")
	}
	if _, err := svc.ClaimLaunch(ctx, time.Now(), "dispatcher", saved.ID); err != nil {
		t.Fatal(err)
	}
	att, err := svc.CreateAttempt(ctx, item.ID, "owner", CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: j.AgentID, RequestID: j.RequestID, ExpectedPredecessorID: p.SourceReviewAttemptID})
	if err != nil {
		t.Fatal(err)
	}
	if att.Status != AttemptStatusQueued || ValidateNativeReleaseSource(ctx, store, item, &p) != nil {
		t.Fatal("exact native release admission lost source")
	}
	again, err := svc.CreateAttempt(ctx, item.ID, "owner", CreateAttemptRequest{RunnerID: "cuan-runner", AgentName: j.AgentID, RequestID: j.RequestID, ExpectedPredecessorID: p.SourceReviewAttemptID})
	if err != nil || again.ID != att.ID {
		t.Fatal("duplicate native release reservation", err)
	}
	info, err := svc.ExecutionInfo(ctx, item.ID)
	if err != nil || info.IntentKind != "native_release" || info.SourceCommit != p.SourceCommit {
		t.Fatal("native execution diagnostics lost source", err)
	}
	if item.Status == TaskStatusDone {
		t.Fatal("release admission accepted unfinished work")
	}
	att.CancelRequested = true
	if err := store.UpdateAttempt(ctx, att); err != nil {
		t.Fatal(err)
	}
	if ValidateNativeReleaseSource(ctx, store, item, &p) == nil {
		t.Fatal("cancelled native stage retained execution authority")
	}
}
