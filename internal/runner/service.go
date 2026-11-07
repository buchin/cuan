package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/taskfsm"
)

type ProjectFinder interface {
	Get(ctx context.Context, id string) (*project.ProjectResponse, error)
	TaskContext(ctx context.Context, id, instructions string) (*project.ProjectResponse, error)
}

type RunnerService struct {
	browserRuntimes sync.Map
	store           Store
	taskStore       task.Store
	projectFinder   ProjectFinder
	dodEvaluator    *DoDEvaluator
	chatNotifier    BusinessChatNotifier
	eventBus        *EventBus
	steerMu         sync.Mutex
	pendingSteering map[string][]SteeringDirective
	steerHandlers   map[string]func(attemptID, prompt string) error
}

func NewService(store Store, taskStore task.Store, projectFinder ...ProjectFinder) *RunnerService {
	var pf ProjectFinder
	if len(projectFinder) > 0 {
		pf = projectFinder[0]
	}
	return &RunnerService{
		store:           store,
		taskStore:       taskStore,
		projectFinder:   pf,
		dodEvaluator:    NewDoDEvaluator(),
		eventBus:        GlobalEventBus,
		pendingSteering: make(map[string][]SteeringDirective),
		steerHandlers:   make(map[string]func(attemptID, prompt string) error),
	}
}

func (s *RunnerService) SetProjectFinder(f ProjectFinder) {
	s.projectFinder = f
}

func (s *RunnerService) SetDoDEvaluator(eval *DoDEvaluator) {
	s.dodEvaluator = eval
}

func (s *RunnerService) SetChatNotifier(notifier BusinessChatNotifier) {
	s.chatNotifier = notifier
}

func (s *RunnerService) SetEventBus(bus *EventBus) {
	s.eventBus = bus
}

// EventBus returns the registered EventBus, or nil if none is set.
func (s *RunnerService) EventBus() *EventBus {
	return s.eventBus
}

// ValidateTaskTransition checks if a task status transition is permitted by the state machine.
func (s *RunnerService) ValidateTaskTransition(fromStatus, toStatus string) error {
	fromState, err := taskfsm.FromCuanTaskStatus(fromStatus)
	if err != nil {
		return err
	}
	toState, err := taskfsm.FromCuanTaskStatus(toStatus)
	if err != nil {
		return err
	}
	return taskfsm.ValidateTransition(fromState, toState)
}

// Heartbeat processes heartbeat and lease renewal from a runner daemon.
func (s *RunnerService) Heartbeat(ctx context.Context, req HeartbeatRequest, leaseDuration time.Duration) (*HeartbeatResponse, error) {
	if req.RunnerID == "" {
		return nil, fmt.Errorf("runner_id is required")
	}

	info := &RunnerInfo{
		ID:               req.RunnerID,
		Name:             req.RunnerID,
		Hostname:         req.Hostname,
		SupportedAgents:  req.SupportedAgents,
		CurrentAttemptID: req.CurrentAttemptID,
		CPUPercent:       req.CPUPercent,
		MemoryUsedBytes:  req.MemoryUsedBytes,
		MemoryTotalBytes: req.MemoryTotalBytes,
		DiskUsedBytes:    req.DiskUsedBytes,
		DiskTotalBytes:   req.DiskTotalBytes,
	}
	if req.CurrentAttemptID != "" {
		info.Status = StatusBusy
	} else {
		info.Status = StatusIdle
	}

	if err := s.store.RegisterOrUpdateHeartbeat(ctx, info); err != nil {
		return nil, err
	}

	if req.BrowserRuntime.Fresh(time.Now()) {
		s.browserRuntimes.Store(req.RunnerID, *req.BrowserRuntime)
	} else {
		s.browserRuntimes.Delete(req.RunnerID)
	}

	shouldCancel := false
	var canceledIDs []string
	attemptIDsToCheck := req.ActiveAttemptIDs
	if len(attemptIDsToCheck) == 0 && req.CurrentAttemptID != "" {
		attemptIDsToCheck = []string{req.CurrentAttemptID}
	}
	for _, attID := range attemptIDsToCheck {
		if attID == "" {
			continue
		}
		canceled, err := s.store.RenewLease(ctx, attID, req.RunnerID, leaseDuration)
		if err != nil || canceled {
			shouldCancel = true
			canceledIDs = append(canceledIDs, attID)
		}
	}

	s.steerMu.Lock()
	var directives []SteeringDirective
	for _, attID := range attemptIDsToCheck {
		if pending, ok := s.pendingSteering[attID]; ok && len(pending) > 0 {
			directives = append(directives, pending...)
			delete(s.pendingSteering, attID)
		}
	}
	s.steerMu.Unlock()

	return &HeartbeatResponse{
		Status:             "ok",
		LeaseDurationSec:   int(leaseDuration.Seconds()),
		ShouldCancel:       shouldCancel,
		CanceledAttemptIDs: canceledIDs,
		SteeringDirectives: directives,
	}, nil
}

// RegisterSteerHandler connects an in-process or local runner's steering dispatcher.
func (s *RunnerService) RegisterSteerHandler(runnerID string, handler func(attemptID, prompt string) error) {
	s.steerMu.Lock()
	defer s.steerMu.Unlock()
	s.steerHandlers[runnerID] = handler
}

// Steer injects an interactive steering prompt into the active attempt's agent process stdin,
// or queues it for runner heartbeat dispatch, or performs a graceful fallback to a follow-up comment.
func (s *RunnerService) Steer(ctx context.Context, attemptID string, req SteerAttemptRequest) (*SteerAttemptResponse, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, errors.New("prompt is required")
	}

	att, err := s.taskStore.GetAttempt(ctx, attemptID)
	if err != nil {
		return nil, fmt.Errorf("attempt %s: %w", attemptID, err)
	}
	if att == nil {
		return nil, fmt.Errorf("attempt not found: %s", attemptID)
	}

	author := req.Author
	if author == "" {
		author = "user"
	}

	// If attempt is not running, gracefully fallback by recording a queued follow-up activity on the task
	if att.Status != task.AttemptStatusRunning {
		_ = s.taskStore.CreateActivity(ctx, &task.TaskActivity{
			TaskID:       att.TaskID,
			AttemptID:    &attemptID,
			ActivityType: task.ActivityTypeComment,
			Author:       author,
			Message:      fmt.Sprintf("[Follow-up Prompt] %s", req.Prompt),
			CreatedAt:    time.Now().UTC(),
		})
		return &SteerAttemptResponse{
			Status:   "queued",
			Fallback: true,
			Message:  "Attempt is not running; prompt recorded as queued follow-up on task.",
		}, nil
	}

	// Try immediate delivery if a direct steer handler is registered for this runner
	s.steerMu.Lock()
	handler, hasHandler := s.steerHandlers[att.RunnerID]
	s.steerMu.Unlock()

	if hasHandler && handler != nil {
		if hErr := handler(attemptID, req.Prompt); hErr == nil {
			_ = s.taskStore.CreateActivity(ctx, &task.TaskActivity{
				TaskID:       att.TaskID,
				AttemptID:    &attemptID,
				ActivityType: task.ActivityTypeComment,
				Author:       author,
				Message:      fmt.Sprintf("[Interactive Steering] %s", req.Prompt),
				CreatedAt:    time.Now().UTC(),
			})
			return &SteerAttemptResponse{
				Status:   "delivered",
				Fallback: false,
				Message:  "Steering instruction injected to active agent process stdin.",
			}, nil
		}
	}

	// Queue for next heartbeat dispatch to runner
	s.steerMu.Lock()
	s.pendingSteering[attemptID] = append(s.pendingSteering[attemptID], SteeringDirective{
		AttemptID: attemptID,
		Prompt:    req.Prompt,
		Author:    author,
	})
	s.steerMu.Unlock()

	_ = s.taskStore.CreateActivity(ctx, &task.TaskActivity{
		TaskID:       att.TaskID,
		AttemptID:    &attemptID,
		ActivityType: task.ActivityTypeComment,
		Author:       author,
		Message:      fmt.Sprintf("[Interactive Steering Queued] %s", req.Prompt),
		CreatedAt:    time.Now().UTC(),
	})

	return &SteerAttemptResponse{
		Status:   "delivered",
		Fallback: false,
		Message:  "Steering instruction queued for active runner.",
	}, nil
}

// Claim attempts to acquire the next queued task attempt for the runner.
func (s *RunnerService) Claim(ctx context.Context, req ClaimTaskRequest, leaseDuration time.Duration) (*ClaimTaskResponse, error) {
	if req.RunnerID == "" {
		return nil, fmt.Errorf("runner_id is required")
	}

	claimCtx := context.WithValue(ctx, verificationFinalizationClaimKey{}, req.SupportsVerificationFinalization)
	claimCtx = context.WithValue(claimCtx, liveInspectionClaimKey{}, req.SupportsLiveInspection && req.SupportsNativeVisualVerification)
	claimCtx = context.WithValue(claimCtx, liveInspectionRecoveryClaimKey{}, req.SupportsLiveInspectionRecovery)
	claimCtx = context.WithValue(claimCtx, currentOutcomeClaimKey{}, req.SupportsCurrentOutcomeInspection)
	claimCtx = context.WithValue(claimCtx, evidenceRevalidationClaimKey{}, req.SupportsEvidenceRevalidation)
	att, t, err := s.store.ClaimNextQueuedAttempt(claimCtx, req.RunnerID, req.SupportedAgents, leaseDuration, req.AvailableLanes)
	if err != nil {
		return nil, fmt.Errorf("claim attempt: %w", err)
	}

	if att == nil || t == nil {
		return &ClaimTaskResponse{Claimed: false}, nil
	}

	fresh, err := s.taskStore.GetTask(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	t = fresh

	var leaseExpires time.Time
	if att.LeaseExpiresAt != nil {
		leaseExpires = *att.LeaseExpiresAt
	}

	limits, err := task.AttemptPublicationPolicy(ctx, s.taskStore, t, att.ID)
	if err != nil {
		return nil, err
	}
	verification, err := task.ValidateVerificationSource(ctx, s.taskStore, t, att.ID)
	if err != nil {
		return nil, err
	}
	if verification != nil && verification.FinalizeAttemptID != "" && !req.SupportsVerificationFinalization {
		return nil, fmt.Errorf("runner does not support verification finalization")
	}
	inspection, err := task.ValidateLiveInspection(ctx, s.taskStore, t, att.ID)
	if err != nil {
		return nil, fmt.Errorf("live result inspection cannot be claimed: %w", err)
	}
	if inspection != nil && (!req.SupportsLiveInspection || !req.SupportsNativeVisualVerification) {
		return nil, fmt.Errorf("runner does not support live result inspection")
	}
	if inspection != nil && inspection.PreviousAttemptID != "" && !req.SupportsLiveInspectionRecovery {
		return nil, fmt.Errorf("runner does not support live result inspection recovery")
	}
	if inspection != nil && inspection.Purpose == task.CurrentOutcomeInspection && !req.SupportsCurrentOutcomeInspection {
		return nil, fmt.Errorf("runner does not support current outcome inspection")
	}
	if inspection != nil && inspection.RevalidationKind == task.EvidenceRevalidation && !req.SupportsEvidenceRevalidation {
		return nil, fmt.Errorf("runner does not support evidence revalidation")
	}
	if runtime := s.BrowserRuntime(ctx, req.RunnerID); runtime != nil {
		raw, _ := json.Marshal(runtime)
		if err = s.taskStore.CreateActivity(ctx, &task.TaskActivity{ID: "verification-runtime:" + att.ID, TaskID: t.ID, AttemptID: &att.ID, ActivityType: task.ActivityVerificationRuntime, Author: req.RunnerID, Message: string(raw), CreatedAt: time.Now().UTC()}); err != nil {
			return nil, err
		}
	}
	resp := &ClaimTaskResponse{
		LiveInspection:    inspection,
		Verification:      verification,
		Claimed:           true,
		PublicationLimits: limits,
		Attempt:           att,
		Task:              t,
		LeaseExpires:      leaseExpires,
	}

	if s.projectFinder != nil && t.ProjectID != "" {
		p, err := s.projectFinder.TaskContext(ctx, t.ProjectID, t.Instructions)
		if err != nil || p == nil {
			// No provider has started. Keep a terminal, attributable failure
			// rather than executing against unrelated current defaults.
			_, finishErr := s.Complete(ctx, att.ID, CompleteAttemptRequest{RunnerID: req.RunnerID, Status: task.AttemptStatusFailed, ExitCode: 1, ErrorMessage: "Saved task project context is unavailable; no provider execution was started"})
			if finishErr != nil {
				return nil, finishErr
			}
			return nil, errors.New("Saved task project context is unavailable")
		}
		resp.ProjectRepo = fmt.Sprintf("%s/%s", p.RepoOwner, p.RepoName)
		resp.RepoOwner = p.RepoOwner
		resp.RepoName = p.RepoName
		resp.RepoURL = p.RepoURL
		resp.DefaultBranch = p.DefaultBranch
		resp.WorkingDir = p.LocalPath
		settings := project.SettingsFromMetadata(p.Metadata)
		resp.AutoDeploy = settings.IsAutoDeployEnabled() && !limits.DenyDeploy
		resp.Environment = settings.Environment
		resp.DeployBranch = strings.TrimSpace(settings.DeployBranch)
		if resp.DeployBranch == "" {
			resp.DeployBranch = p.DefaultBranch
		}
	}

	if verification != nil && verification.FinalizeAttemptID != "" {
		resp.Task = &task.Task{ID: t.ID, ProjectID: t.ProjectID, Title: t.Title, SourceRoomID: t.SourceRoomID, ExecutionCategory: t.ExecutionCategory}
		resp.Environment = ""
		resp.AutoDeploy = false
		resp.OwnerID = verification.OwnerID
	}
	if inspection != nil {
		resp.PHPUnitTargets = project.ParseEnvironment(resp.Environment)["CUAN_VERIFICATION_PHPUNIT_TARGETS"]
		resp.Environment = ""
		resp.AutoDeploy = false
	}
	return resp, nil
}

// Complete handles completion report from a runner daemon.
func (s *RunnerService) Complete(ctx context.Context, attemptID string, req CompleteAttemptRequest) (*task.TaskAttempt, error) {
	switch req.Status {
	case task.AttemptStatusSucceeded, task.AttemptStatusFailed, task.AttemptStatusCanceled, task.AttemptStatusInterrupted:
	default:
		return nil, task.ErrInvalidTaskInput
	}
	att, err := s.taskStore.GetAttempt(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	if att.RunnerID != req.RunnerID {
		return nil, task.ErrRequestConflict
	}
	if att.Status != task.AttemptStatusRunning && att.Status != task.AttemptStatusQueued {
		return nil, task.ErrAttemptFinal
	}
	latest, err := s.taskStore.GetLatestAttempt(ctx, att.TaskID)
	if err != nil {
		return nil, err
	}
	if latest == nil || latest.ID != att.ID {
		return nil, task.ErrAttemptFinal
	}
	currentTask, intentErr := s.taskStore.GetTask(ctx, att.TaskID)
	if intentErr != nil {
		return nil, intentErr
	}
	intent, intentErr := task.VerificationIntentForAttempt(ctx, s.taskStore, currentTask, att.ID)
	if intentErr != nil {
		return nil, intentErr
	}
	liveInspection, liveErr := task.LiveInspectionForAttempt(ctx, s.taskStore, currentTask, att.ID)
	if liveErr != nil {
		return nil, fmt.Errorf("completion inspection intent: %w", liveErr)
	}
	var effects []func()
	var completionActivities []*task.TaskActivity
	var nextAttempt *task.TaskAttempt
	readOnlyVerified := false

	now := time.Now().UTC()
	att.FinishedAt = &now
	att.PeakRSSBytes = req.PeakRSSBytes
	att.ExitCode = &req.ExitCode

	status := req.Status
	if IsInfraFault(status, req.ErrorMessage, req.ExitCode) {
		status = task.AttemptStatusInterrupted
	} else if req.ExitCode != 0 && status != task.AttemptStatusCanceled {
		status = task.AttemptStatusFailed
	}
	if status == task.AttemptStatusSucceeded && req.AgentReportRequired && !req.AgentReportConfirmed {
		status = task.AttemptStatusFailed
		att.ErrorMessage = "Agent finished without a confirmed final task report"
	}
	if status == task.AttemptStatusSucceeded {
		current, err := s.taskStore.GetTask(ctx, att.TaskID)
		if err != nil {
			return nil, err
		}
		if current == nil {
			return nil, task.ErrTaskNotFound
		}
		limits, err := task.AttemptPublicationPolicy(ctx, s.taskStore, current, att.ID)
		if err != nil {
			return nil, err
		}
		activities, readErr := s.taskStore.ListActivities(ctx, att.TaskID)
		if readErr != nil {
			return nil, readErr
		}
		expected, recipeErr := task.BoundVisualRecipe(activities, current.ID, att.ID)
		if recipeErr != nil {
			return nil, recipeErr
		}
		if expected != nil || req.VisualVerification != nil {
			if expected == nil || liveInspection == nil || expected.SourceCommit != liveInspection.TargetCommit || !req.VisualVerification.Valid() || req.VisualVerification.Recipe != *expected || !req.RepositoryVerification.Unchanged() || req.VisualVerification.Baseline != req.RepositoryVerification.Baseline {
				status = task.AttemptStatusFailed
				att.ErrorMessage = "Registered visual verification lacks its bound recipe, capture manifest or unchanged source"
			} else {
				recorded := task.VisualVerificationForAttempt(activities, current.ID, att.ID, req.RunnerID)
				given, _ := json.Marshal(req.VisualVerification)
				saved, _ := json.Marshal(recorded)
				if recorded == nil || string(given) != string(saved) {
					status = task.AttemptStatusFailed
					att.ErrorMessage = "Native visual capture was not durably recorded before completion"
				}

			}
		}
		if liveInspection != nil {
			if _, err := task.ValidateLiveInspection(ctx, s.taskStore, current, att.ID); err != nil {
				return nil, fmt.Errorf("completion inspection source validation: %w", err)
			}
			identityVerified := req.SourceAncestryVerified && !req.CurrentOutcomeVerified
			if liveInspection.Purpose == task.CurrentOutcomeInspection {
				identityVerified = req.CurrentOutcomeVerified && !req.SourceAncestryVerified
			}
			if !limits.ReadOnlyRepository || !req.RepositoryVerification.Unchanged() || !identityVerified || req.LiveInspectionCommit != liveInspection.TargetCommit || !task.PassingProjectCheckSummary(req.TestSummary) || !req.AgentReportRequired || !req.AgentReportConfirmed || req.LiveDeploymentConfirmed || req.CommitSHA != "" || req.PRURL != "" || req.PRNumber != 0 || (req.HasCodeChanges != nil && *req.HasCodeChanges) {
				status = task.AttemptStatusFailed
				att.ErrorMessage = "Live result inspection lacks exact revision evidence, fresh project checks, final report or unchanged scope"
			} else {
				receipt := task.LiveInspectionReceipt{Purpose: liveInspection.Purpose, SourceAttemptID: liveInspection.SourceAttemptID, SourceCommit: liveInspection.SourceCommit, TargetCommit: liveInspection.TargetCommit, Repository: liveInspection.Repository, ProjectChecksHash: task.VerificationSummaryHash(req.TestSummary)}
				raw, _ := json.Marshal(receipt)
				completionActivities = append(completionActivities, &task.TaskActivity{ID: "live-result:" + att.ID, TaskID: att.TaskID, AttemptID: &att.ID, ActivityType: task.ActivityLiveInspection, Author: req.RunnerID, Message: string(raw), CreatedAt: now})
				req.DoDValidation = &DoDValidationResult{Passed: false, Action: "review"}
			}
		}

		proof, proofErr := task.ValidateVerificationSource(ctx, s.taskStore, current, att.ID)
		if proofErr != nil {
			return nil, proofErr
		}
		if proof != nil {
			if (proof.FinalizeAttemptID != "" && (req.FinalizationManifestHash != proof.ArtifactManifestHash || req.RepositoryVerification == nil || req.RepositoryVerification.Baseline != proof.RepositoryBaseline)) || (proof.FinalizeAttemptID == "" && proof.RuntimeFingerprint != "" && req.VerificationRuntimeFingerprint != proof.RuntimeFingerprint) || req.VerifiedSourceCommit != proof.SourceCommit || !limits.ReadOnlyRepository || !req.RepositoryVerification.Unchanged() || req.LiveDeploymentConfirmed || req.CommitSHA != "" || req.PRURL != "" || req.PRNumber != 0 || (req.HasCodeChanges != nil && *req.HasCodeChanges) {
				status = task.AttemptStatusFailed
				att.ErrorMessage = "Verification-only source or publication scope was not verified by the runner"
			} else {
				completionActivities = append(completionActivities, &task.TaskActivity{ID: "verification-source:" + att.ID, TaskID: att.TaskID, AttemptID: &att.ID, ActivityType: "verification_source_verified", Author: req.RunnerID, Message: proof.SourceCommit, CreatedAt: now})
			}
		}
		if limits.ReadOnlyRepository {
			if !req.RepositoryVerification.Unchanged() || req.CommitSHA != "" || req.PRURL != "" || req.PRNumber != 0 || (req.HasCodeChanges != nil && *req.HasCodeChanges) {
				status = task.AttemptStatusFailed
				att.ErrorMessage = "Read-only repository scope was not verified by the runner; no completion accepted. Preserve saved work and report the scope mismatch without repairing unrelated tests."
			} else {
				readOnlyVerified = true
				completionActivities = append(completionActivities, &task.TaskActivity{ID: "repository-scope:" + att.ID, TaskID: att.TaskID, AttemptID: &att.ID, ActivityType: "repository_scope_verified", Author: req.RunnerID, NewStatus: "read_only", Message: "Runner verified unchanged repository state: " + req.RepositoryVerification.Baseline, CreatedAt: now})
			}
		}
	}
	if status != task.AttemptStatusSucceeded && req.RepositoryVerification != nil {
		if current, err := s.taskStore.GetTask(ctx, att.TaskID); err == nil && current != nil {
			if limits, err := task.AttemptPublicationPolicy(ctx, s.taskStore, current, att.ID); err == nil && limits.ReadOnlyRepository {
				raw, _ := json.Marshal(req.RepositoryVerification)
				completionActivities = append(completionActivities, &task.TaskActivity{ID: "repository-observation:" + att.ID, TaskID: att.TaskID, AttemptID: &att.ID, ActivityType: "repository_scope_observed", Author: req.RunnerID, NewStatus: status, Message: string(raw), CreatedAt: now})
			}
		}
	}
	if status == task.AttemptStatusSucceeded && s.projectFinder != nil {
		if t, err := s.taskStore.GetTask(ctx, att.TaskID); err == nil && t != nil && t.ProjectID != "" {
			if p, err := s.projectFinder.TaskContext(ctx, t.ProjectID, t.Instructions); err == nil && p != nil {
				settings := project.SettingsFromMetadata(p.Metadata)
				isCodeLane := t.ExecutionCategory == "" || t.ExecutionCategory == task.ExecutionCategoryCode
				requiresDeploy := isCodeLane
				if req.RequiresDeploy != nil {
					requiresDeploy = *req.RequiresDeploy
				} else if (req.HasCodeChanges != nil && !*req.HasCodeChanges) || (req.DiffSummary == "" && req.CommitSHA == "") {
					if isInvestigationOrNonCode(t.Title, t.Instructions, t.Summary, t.CompleteNote) {
						requiresDeploy = false
					}
				}
				// A caller's exemption cannot override observed code changes.
				if isCodeLane && req.HasCodeChanges != nil && *req.HasCodeChanges {
					requiresDeploy = true
				}

				limits, policyErr := task.AttemptPublicationPolicy(ctx, s.taskStore, t, att.ID)
				if policyErr != nil {
					return nil, policyErr
				}
				if settings.IsAutoDeployEnabled() && requiresDeploy && !limits.DenyDeploy {
					activities, receiptErr := s.taskStore.ListActivities(ctx, att.TaskID)
					if receiptErr != nil {
						return nil, receiptErr
					}
					if !req.LiveDeploymentConfirmed || !task.VerifiedDeployment(activities, att, req.CommitSHA, now) {
						status = task.AttemptStatusFailed
						att.ErrorMessage = "Kepatuhan auto_deploy: verified production deployment receipt is required for the exact task, attempt and commit; commit/push and report claims alone do not satisfy acceptance"
					}
				}
			} else {
				status = task.AttemptStatusFailed
				att.ErrorMessage = "Saved task project context could not be verified; completion was not accepted"
			}
		}
	}
	att.Status = status
	if att.ErrorMessage == "" {
		att.ErrorMessage = req.ErrorMessage
	}

	// Update task status:
	t, err := s.taskStore.GetTask(ctx, att.TaskID)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, task.ErrTaskNotFound
	}
	{
		oldStatus := t.Status
		expectedUpdatedAt := t.UpdatedAt
		if status == task.AttemptStatusSucceeded && intent != nil && intent.FinalizeAttemptID != "" {
			report := task.FinalizationReport{Summary: "Saved verification outputs checked against the unchanged source.", Verification: "The exact source commit and repository fingerprint match the prior verified baseline. Saved outputs are linked from the interrupted attempt for independent visual review. No AI execution, project commands, code changes, push or deployment occurred. Original implementation checks and the prior timeout retain their own provenance.", OriginAttemptID: intent.FinalizeAttemptID, ManifestHash: intent.ArtifactManifestHash}
			raw, _ := json.Marshal(report)
			completionActivities = append(completionActivities, &task.TaskActivity{ID: "finalization:" + att.ID, TaskID: t.ID, AttemptID: &att.ID, ActivityType: task.ActivityRunnerFinalization, Author: req.RunnerID, NewStatus: task.TaskStatusReview, Message: string(raw), CreatedAt: now})
			t.Summary = report.Summary
			t.CompleteNote = report.Verification
			req.DoDValidation = &DoDValidationResult{Passed: false, Action: "review"}
		}

		// 1. Emisi internal event task:attempt_succeeded / task:attempt_completed
		completedEvt := AttemptCompletedEvent{
			EventID:       fmt.Sprintf("evt_%d", time.Now().UnixNano()),
			EventType:     EventTaskAttemptCompleted,
			TaskID:        t.ID,
			AttemptID:     att.ID,
			AttemptNumber: att.AttemptNumber,
			RunnerID:      req.RunnerID,
			ExitCode:      req.ExitCode,
			BranchName:    req.BranchName,
			CommitSHA:     req.CommitSHA,
			ExecutionLane: t.ExecutionCategory,
			Status:        status,
			Summary:       t.Summary,
			CompleteNote:  t.CompleteNote,
			ErrorMessage:  att.ErrorMessage,
			SourceRoomID:  t.SourceRoomID,
			EmittedAt:     now,
			Metadata: map[string]any{
				"runner_id":    req.RunnerID,
				"exit_code":    req.ExitCode,
				"test_summary": req.TestSummary,
			},
		}
		if status == task.AttemptStatusSucceeded {
			completedEvt.EventType = EventTaskAttemptSucceeded
		} else {
			completedEvt.EventType = EventTaskAttemptFailed
		}
		if s.eventBus != nil {
			effects = append(effects, func() { s.eventBus.PublishCompleted(ctx, completedEvt) })
		} else {
			effects = append(effects, func() { EmitAttemptCompleted(ctx, completedEvt) })
		}

		// 2. Evaluasi DoD Otonom & Logika Auto-Mark Done / Auto-Requeue
		dodVal := req.DoDValidation
		if t.SourceRoomID != "" {
			// Inbox owns semantic acceptance and scoped continuation. A detached
			// worker may run an older evaluator: retain its assessment as data,
			// never let it inject unrelated requirements or create a new attempt.
			if dodVal != nil {
				raw, _ := json.Marshal(dodVal)
				completionActivities = append(completionActivities, &task.TaskActivity{
					ID: "worker-dod:" + att.ID, TaskID: t.ID, AttemptID: &att.ID,
					ActivityType: "worker_dod_assessment", Author: req.RunnerID,
					Message: string(raw), CreatedAt: now,
				})
			}
			dodVal = &DoDValidationResult{Action: "review"}
		}
		if dodVal == nil && status == task.AttemptStatusSucceeded && s.dodEvaluator != nil {
			workingDir := ""
			if s.projectFinder != nil && t.ProjectID != "" {
				if p, err := s.projectFinder.Get(ctx, t.ProjectID); err == nil && p != nil {
					workingDir = p.LocalPath
				}
			}
			valReq := DoDValidationRequest{
				RepositoryReadOnlyVerified: readOnlyVerified,
				Checklist:                  t.DoDChecklist, TaskID: t.ID,
				AttemptID:     att.ID,
				AttemptNumber: att.AttemptNumber,
				WorkingDir:    workingDir,
				TaskTitle:     t.Title,
				Instructions:  t.Instructions,
				Summary:       t.Summary,
				CompleteNote:  t.CompleteNote,
				ExecutionLane: t.ExecutionCategory,
			}
			dodVal, _ = s.dodEvaluator.EvaluateDoD(ctx, valReq)
		}

		if dodVal != nil && dodVal.Passed {
			if gaps := task.AcceptanceGaps(t); len(gaps) > 0 {
				dodVal = &DoDValidationResult{Passed: false, Action: "review", Gaps: gaps}
			}
		}
		autoRequeued := false
		if status == task.AttemptStatusSucceeded {
			if dodVal != nil && dodVal.Passed {
				// Lolos 100%: Otomatis ubah status task menjadi 'done' dan catat verifikasi
				t.Status = task.TaskStatusDone
				if dodVal.Details != "" && strings.TrimSpace(t.CompleteNote) == "" {
					t.CompleteNote = dodVal.Details
				}
				// Inbox owns final acceptance and the final result announcement.
				// A successful worker report is ready for review, not yet accepted.
				if t.SourceRoomID != "" {
					t.Status = task.TaskStatusReview
				}
				// Proactive messenger di Chat/Inbox: lapor tugas tuntas dalam bahasa bisnis ramah
				if s.chatNotifier != nil && t.SourceRoomID == "" {
					pName := ""
					if s.projectFinder != nil && t.ProjectID != "" {
						if p, err := s.projectFinder.Get(ctx, t.ProjectID); err == nil && p != nil {
							pName = p.Name
						}
					}
					for _, rID := range uniqueRooms(t.SourceRoomID, req.RoomID) {
						if pn, ok := s.chatNotifier.(ProjectAwareNotifier); ok && pName != "" {
							effects = append(effects, func() { _ = pn.SendTaskCompletedBusinessMessageWithContext(ctx, rID, t.ID, t.Title, t.Summary, pName) })
						} else {
							effects = append(effects, func() { _ = s.chatNotifier.SendTaskCompletedBusinessMessage(ctx, rID, t.ID, t.Title, t.Summary) })
						}
					}
				}
			} else if intent == nil && dodVal != nil && dodVal.Action == "auto_requeue" && dodVal.Fixable && req.AgentReportConfirmed && len(task.AcceptanceGaps(t)) == 0 && CanRequeue(att.AttemptNumber) {
				// Ada gap fixable & attempt < 3: Requeue otomatis dengan alokasi waktu adaptif
				t.Status = task.TaskStatusQueued
				autoRequeued = true
				nextAttNum := dodVal.NextAttemptNumber
				if nextAttNum <= 0 {
					nextAttNum = att.AttemptNumber + 1
				}
				nextTimeout := dodVal.NextTimeout
				if nextTimeout <= 0 {
					nextTimeout = CalculateAdaptiveTimeout(nextAttNum)
				}

				newAttempt := &task.TaskAttempt{
					ID:            fmt.Sprintf("att_%d", time.Now().UnixNano()),
					TaskID:        t.ID,
					AttemptNumber: nextAttNum,
					RunnerID:      req.RunnerID,
					AgentName:     att.AgentName,
					Status:        task.AttemptStatusQueued,
					CreatedAt:     now,
				}
				nextAttempt = newAttempt

				requeueMsg := fmt.Sprintf("Auto-Requeue: Celah DoD terdeteksi pada attempt #%d. Task otomatis dikembalikan ke antrean untuk perbaikan pada attempt #%d (alokasi timeout adaptif: %v).\n\nInstruksi Revisi:\n%s",
					att.AttemptNumber, nextAttNum, nextTimeout, dodVal.RequeueInstructions)
				requeueAct := &task.TaskActivity{
					ID:           fmt.Sprintf("act_%d", time.Now().UnixNano()),
					TaskID:       t.ID,
					AttemptID:    &newAttempt.ID,
					ActivityType: task.ActivityTypeAttempt,
					Author:       "cuan_dod_evaluator",
					OldStatus:    oldStatus,
					NewStatus:    task.TaskStatusQueued,
					Message:      requeueMsg,
					CreatedAt:    now,
				}
				completionActivities = append(completionActivities, requeueAct)
			} else {
				t.Status = task.TaskStatusReview
				// Inbox owns acceptance and can supply pending evidence through
				// its separate verifier. A successful handoff awaits that review;
				// incomplete criteria alone do not mean execution is blocked.
				if t.SourceRoomID == "" && len(task.AcceptanceGaps(t)) > 0 {
					t.Status = task.TaskStatusBlocked
				}
				if s.chatNotifier != nil && t.SourceRoomID == "" {
					var gaps []string
					if dodVal != nil {
						gaps = dodVal.Gaps
					}
					pName := ""
					if s.projectFinder != nil && t.ProjectID != "" {
						if p, err := s.projectFinder.Get(ctx, t.ProjectID); err == nil && p != nil {
							pName = p.Name
						}
					}
					for _, rID := range uniqueRooms(t.SourceRoomID, req.RoomID) {
						if pn, ok := s.chatNotifier.(ProjectAwareNotifier); ok && pName != "" {
							effects = append(effects, func() {
								_ = pn.SendTaskReviewBusinessMessageWithContext(ctx, rID, t.ID, t.Title, t.Summary, pName, gaps, att.AttemptNumber)
							})
						} else {
							effects = append(effects, func() {
								_ = s.chatNotifier.SendTaskReviewBusinessMessage(ctx, rID, t.ID, t.Title, t.Summary, gaps, att.AttemptNumber)
							})
						}
					}
				}
			}
		} else if status == task.AttemptStatusInterrupted || IsInfraFault(status, att.ErrorMessage, req.ExitCode) {
			t.Status = task.TaskStatusBlocked
			if s.chatNotifier != nil && t.SourceRoomID == "" {
				errMsg := att.ErrorMessage
				if errMsg == "" {
					errMsg = "Koneksi runner terputus saat eksekusi (Needs Attention). Konteks dan pekerjaan tersimpan aman."
				}
				pName := ""
				if s.projectFinder != nil && t.ProjectID != "" {
					if p, err := s.projectFinder.Get(ctx, t.ProjectID); err == nil && p != nil {
						pName = p.Name
					}
				}
				for _, rID := range uniqueRooms(t.SourceRoomID, req.RoomID) {
					if pn, ok := s.chatNotifier.(ProjectAwareNotifier); ok && pName != "" {
						effects = append(effects, func() {
							_ = pn.SendTaskReviewBusinessMessageWithContext(ctx, rID, t.ID, t.Title, t.Summary, pName, []string{errMsg}, att.AttemptNumber)
						})
					} else {
						effects = append(effects, func() {
							_ = s.chatNotifier.SendTaskReviewBusinessMessage(ctx, rID, t.ID, t.Title, t.Summary, []string{errMsg}, att.AttemptNumber)
						})
					}
				}
			}
		} else if status == task.AttemptStatusFailed || status == task.AttemptStatusCanceled {
			t.Status = task.TaskStatusBlocked
			if s.chatNotifier != nil && t.SourceRoomID == "" {
				errMsg := att.ErrorMessage
				if errMsg == "" {
					errMsg = "Pekerjaan terhenti dan memerlukan peninjauan lebih lanjut."
				}
				pName := ""
				if s.projectFinder != nil && t.ProjectID != "" {
					if p, err := s.projectFinder.Get(ctx, t.ProjectID); err == nil && p != nil {
						pName = p.Name
					}
				}
				for _, rID := range uniqueRooms(t.SourceRoomID, req.RoomID) {
					if pn, ok := s.chatNotifier.(ProjectAwareNotifier); ok && pName != "" {
						effects = append(effects, func() {
							_ = pn.SendTaskReviewBusinessMessageWithContext(ctx, rID, t.ID, t.Title, t.Summary, pName, []string{errMsg}, att.AttemptNumber)
						})
					} else {
						effects = append(effects, func() {
							_ = s.chatNotifier.SendTaskReviewBusinessMessage(ctx, rID, t.ID, t.Title, t.Summary, []string{errMsg}, att.AttemptNumber)
						})
					}
				}
			}
		}

		// Enforce state machine transition guardrails via pkg/taskfsm
		if srcState, errSrc := taskfsm.FromCuanTaskStatus(oldStatus); errSrc == nil {
			if dstState, errDst := taskfsm.FromCuanTaskStatus(t.Status); errDst == nil {
				if !taskfsm.CanTransition(srcState, dstState) && srcState != dstState {
					return nil, fmt.Errorf("completion task transition %s to %s is not allowed: %w", oldStatus, t.Status, task.ErrRequestConflict)
				}
			}
		}

		if req.BranchName != "" {
			t.BranchName = req.BranchName
		}
		if req.PRURL != "" {
			t.PRURL = req.PRURL
		}
		if req.PRNumber > 0 {
			t.PRNumber = req.PRNumber
		}
		if req.CommitSHA != "" {
			t.CommitSHA = req.CommitSHA
		}
		if req.DiffSummary != "" {
			t.DiffSummary = req.DiffSummary
		}
		if req.TestSummary != "" {
			t.TestSummary = req.TestSummary
		}

		t.UpdatedAt = now
		// Record activity
		msg := fmt.Sprintf("Attempt %s selesai dengan status '%s' (exit code: %d, peak RSS: %d bytes).", att.ID, status, req.ExitCode, req.PeakRSSBytes)
		if autoRequeued {
			msg = fmt.Sprintf("Attempt %s selesai: %s (auto-requeued untuk perbaikan celah DoD).", att.ID, status)
		} else if t.PRURL != "" {
			msg = fmt.Sprintf("Attempt %s selesai: %s. Branch: %s, PR: %s (commit: %s).", att.ID, status, t.BranchName, t.PRURL, t.CommitSHA)
		}

		act := &task.TaskActivity{
			ID:           fmt.Sprintf("act_%d", time.Now().UnixNano()),
			TaskID:       t.ID,
			AttemptID:    &att.ID,
			ActivityType: task.ActivityTypeAttempt,
			Author:       req.RunnerID,
			OldStatus:    oldStatus,
			NewStatus:    t.Status,
			Message:      msg,
			CreatedAt:    now,
		}
		completionActivities = append(completionActivities, act)
		var changes *task.RunnerChangeRecord
		if req.DiffSummary != "" && req.CommitSHA != "" {
			changes = task.NewRunnerChangeRecord(req.CommitSHA, req.DiffSummary)
		}
		verification, _ := json.Marshal(task.RunnerVerificationRecord{Summary: req.TestSummary, Changes: changes})
		completionActivities = append(completionActivities, &task.TaskActivity{
			ID: "verification:" + att.ID, TaskID: t.ID, AttemptID: &att.ID,
			ActivityType: task.ActivityTypeRunnerVerification, Author: req.RunnerID,
			Message: string(verification), CreatedAt: now,
		})
		completionStore, ok := s.taskStore.(task.CompletionStore)
		if !ok {
			return nil, fmt.Errorf("atomic execution completion unavailable")
		}
		if err := completionStore.CompleteExecution(ctx, att, t, nextAttempt, expectedUpdatedAt, req.RunnerID, true, completionActivities...); err != nil {
			return nil, err
		}
		for _, effect := range effects {
			effect()
		}

	}

	if assignments, ok := s.store.(interface {
		ReleaseAssignment(context.Context, string, string) error
	}); ok {
		_ = assignments.ReleaseAssignment(ctx, req.RunnerID, att.ID)
	}

	return att, nil
}

// ReconcileExpiredLeases sweeps expired running attempts and marks them interrupted.
func (s *RunnerService) ReconcileExpiredLeases(ctx context.Context) ([]*task.TaskAttempt, error) {
	return s.store.MarkInterruptedLeases(ctx)
}

func (s *RunnerService) ListRunners(ctx context.Context) ([]*RunnerInfo, error) {
	return s.store.ListRunners(ctx)
}

func (s *RunnerService) GetRunner(ctx context.Context, id string) (*RunnerInfo, error) {
	return s.store.GetRunner(ctx, id)
}

func isInvestigationOrNonCode(texts ...string) bool {
	combined := strings.ToLower(strings.Join(texts, " "))
	for _, term := range []string{
		"investigasi", "investigation", "rca", "root cause",
		"analisis", "analysis", "klarifikasi", "audit", "pemadaman", "outage",
	} {
		if strings.Contains(combined, term) {
			return true
		}
	}
	return false
}

func uniqueRooms(rooms ...string) []string {
	seen := make(map[string]bool)
	var list []string
	for _, r := range rooms {
		trimmed := strings.TrimSpace(r)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			list = append(list, trimmed)
		}
	}
	return list
}

// IsInfraFault returns true if the attempt status, error message, or exit code indicates
// an infrastructure or technical connection issue (runner disconnected, heartbeat lost,
// lease expired, runner killed/restarted) rather than a business task criteria failure.
func IsInfraFault(status, errorMessage string, exitCode int) bool {
	if status == task.AttemptStatusInterrupted {
		return true
	}
	if exitCode == 137 || exitCode == 143 {
		return true
	}
	lower := strings.ToLower(errorMessage)
	return strings.Contains(lower, "lease expired") ||
		strings.Contains(lower, "heartbeat lost") ||
		strings.Contains(lower, "runner killed") ||
		strings.Contains(lower, "runner restart") ||
		strings.Contains(lower, "runner context") ||
		strings.Contains(lower, "connection lost") ||
		strings.Contains(lower, "connection refused") ||
		strings.Contains(lower, "runner disconnected") ||
		strings.Contains(lower, "interrupted")
}
