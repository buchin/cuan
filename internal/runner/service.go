package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

// Store returns the underlying runner store.
func (s *RunnerService) Store() Store {
	return s.store
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

	if req.DependencyPreparation.Fresh(time.Now()) {
		copy := *req.DependencyPreparation
		info.RuntimeEvidence.DependencyPreparation = &copy
	}
	if req.BrowserRuntime.Fresh(time.Now()) {
		copy := *req.BrowserRuntime
		info.RuntimeEvidence.BrowserRuntime = &copy
	}
	if err := s.store.RegisterOrUpdateHeartbeat(ctx, info); err != nil {
		return nil, err
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
	claimCtx = context.WithValue(claimCtx, dependencyPreparationClaimKey{}, s.DependencyPreparationRuntime(ctx, req.RunnerID) != nil)
	claimCtx = context.WithValue(claimCtx, nativeReleaseClaimKey{}, req.SupportsNativeRelease)
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
	nativeRelease, err := task.NativeReleaseIntentForAttempt(ctx, s.taskStore, t, att.ID)
	if err != nil {
		_, finishErr := s.Complete(ctx, att.ID, CompleteAttemptRequest{RunnerID: req.RunnerID, Status: task.AttemptStatusFailed, ExitCode: 1, ErrorMessage: "Native release intent could not be verified; no release execution was started"})
		if finishErr != nil {
			return nil, finishErr
		}
		return nil, err
	}
	if nativeRelease != nil {
		if !req.SupportsNativeRelease {
			return nil, errors.New("runner does not support native release")
		}
		if err := task.ValidateNativeReleaseSource(ctx, s.taskStore, t, nativeRelease); err != nil {
			_, finishErr := s.Complete(ctx, att.ID, CompleteAttemptRequest{RunnerID: req.RunnerID, Status: task.AttemptStatusFailed, ExitCode: 1, ErrorMessage: "Native release source could not be verified; no release execution was started"})
			if finishErr != nil {
				return nil, finishErr
			}
			return nil, err
		}
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
		NativeRelease:     nativeRelease,
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
			log.Printf("[RunnerService] task=%s project=%s saved project context unavailable: %v", t.ID, t.ProjectID, err)
			_, finishErr := s.Complete(ctx, att.ID, CompleteAttemptRequest{RunnerID: req.RunnerID, Status: task.AttemptStatusFailed, ExitCode: 1, ErrorMessage: "Saved task project context is unavailable; no provider execution was started"})
			if finishErr != nil {
				return nil, finishErr
			}
			return nil, errors.New("Saved task project context is unavailable")
		}
		if nativeRelease == nil && verification == nil && inspection == nil && t.ExecutionCategory == task.ExecutionCategoryCode && !limits.ReadOnlyRepository && p.RepoURL != "" {
			if refresher, ok := s.projectFinder.(interface {
				RefreshExecutionBase(context.Context, *project.ProjectResponse) error
			}); ok {
				// Refresh only the registered remote-tracking ref. The runner pins
				// it when resuming its private branch; retained files and commits
				// remain untouched, including unresolved conflict state.
				if err := refresher.RefreshExecutionBase(ctx, p); err != nil {
					_, finishErr := s.Complete(ctx, att.ID, CompleteAttemptRequest{RunnerID: req.RunnerID, Status: task.AttemptStatusFailed, ExitCode: 1, ErrorMessage: "The registered project baseline could not be refreshed; saved work is preserved and no provider execution was started"})
					if finishErr != nil {
						return nil, finishErr
					}
					return nil, errors.New("Registered project baseline is unavailable")
				}
			}
		}
		resp.ProjectRepo = fmt.Sprintf("%s/%s", p.RepoOwner, p.RepoName)
		resp.RepoOwner = p.RepoOwner
		resp.RepoName = p.RepoName
		resp.RepoURL = p.RepoURL
		resp.DefaultBranch = p.DefaultBranch
		resp.WorkingDir = p.LocalPath
		settings := project.SettingsFromMetadata(p.Metadata)
		// Publication is a declared task stage when the approval says so. Without
		// a declaration the saved project setting still governs the claim, so a
		// frozen approval snapshot is never replaced by current defaults.
		autoDeploy := settings.IsAutoDeployEnabled() && !limits.DenyDeploy
		if task.StagesDeclared(t.RequiredStages) {
			autoDeploy = task.RequiresPublish(t.RequiredStages) && !limits.DenyDeploy
		}
		resp.AutoDeploy = autoDeploy
		resp.Environment = settings.Environment
		resp.DeployScript = strings.TrimSpace(settings.DeployScript)
		resp.DeployBranch = strings.TrimSpace(settings.DeployBranch)
		if resp.DeployBranch == "" {
			resp.DeployBranch = p.DefaultBranch
		}
	}

	if err := s.bindManagedReleaseClaim(ctx, req, resp); err != nil {
		return nil, err
	}

	if !limits.ReadOnlyRepository || verification != nil || inspection != nil {
		resp.PHPUnitTargets = project.ParseEnvironment(resp.Environment)["CUAN_VERIFICATION_PHPUNIT_TARGETS"]
	}
	if verification != nil && verification.FinalizeAttemptID != "" {
		resp.Task = &task.Task{ID: t.ID, ProjectID: t.ProjectID, Title: t.Title, SourceRoomID: t.SourceRoomID, ExecutionCategory: t.ExecutionCategory}
		resp.Environment = ""
		resp.PHPUnitTargets = ""
		resp.AutoDeploy = false
		resp.OwnerID = verification.OwnerID
	}
	if inspection != nil {
		resp.Environment = ""
		resp.AutoDeploy = false
	}
	return resp, nil
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

