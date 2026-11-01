package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/pkg/taskfsm"
)

type ProjectFinder interface {
	Get(ctx context.Context, id string) (*project.ProjectResponse, error)
}

type RunnerService struct {
	store         Store
	taskStore     task.Store
	projectFinder ProjectFinder
	dodEvaluator  *DoDEvaluator
	chatNotifier  BusinessChatNotifier
	eventBus      *EventBus
}

func NewService(store Store, taskStore task.Store, projectFinder ...ProjectFinder) *RunnerService {
	var pf ProjectFinder
	if len(projectFinder) > 0 {
		pf = projectFinder[0]
	}
	return &RunnerService{
		store:         store,
		taskStore:     taskStore,
		projectFinder: pf,
		dodEvaluator:  NewDoDEvaluator(),
		eventBus:      GlobalEventBus,
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

	return &HeartbeatResponse{
		Status:             "ok",
		LeaseDurationSec:   int(leaseDuration.Seconds()),
		ShouldCancel:       shouldCancel,
		CanceledAttemptIDs: canceledIDs,
	}, nil
}

// Claim attempts to acquire the next queued task attempt for the runner.
func (s *RunnerService) Claim(ctx context.Context, req ClaimTaskRequest, leaseDuration time.Duration) (*ClaimTaskResponse, error) {
	if req.RunnerID == "" {
		return nil, fmt.Errorf("runner_id is required")
	}

	att, t, err := s.store.ClaimNextQueuedAttempt(ctx, req.RunnerID, req.SupportedAgents, leaseDuration, req.AvailableLanes)
	if err != nil {
		return nil, fmt.Errorf("claim attempt: %w", err)
	}

	if att == nil || t == nil {
		return &ClaimTaskResponse{Claimed: false}, nil
	}

	var leaseExpires time.Time
	if att.LeaseExpiresAt != nil {
		leaseExpires = *att.LeaseExpiresAt
	}

	resp := &ClaimTaskResponse{
		Claimed:      true,
		Attempt:      att,
		Task:         t,
		LeaseExpires: leaseExpires,
	}

	if s.projectFinder != nil && t.ProjectID != "" {
		if p, err := s.projectFinder.Get(ctx, t.ProjectID); err == nil && p != nil {
			resp.ProjectRepo = fmt.Sprintf("%s/%s", p.RepoOwner, p.RepoName)
			resp.RepoOwner = p.RepoOwner
			resp.RepoName = p.RepoName
			resp.RepoURL = p.RepoURL
			resp.DefaultBranch = p.DefaultBranch
			resp.WorkingDir = p.LocalPath
			settings := project.SettingsFromMetadata(p.Metadata)
			resp.AutoDeploy = settings.IsAutoDeployEnabled()
		}
	}

	return resp, nil
}

// Complete handles completion report from a runner daemon.
func (s *RunnerService) Complete(ctx context.Context, attemptID string, req CompleteAttemptRequest) (*task.TaskAttempt, error) {
	att, err := s.taskStore.GetAttempt(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	att.FinishedAt = &now
	att.PeakRSSBytes = req.PeakRSSBytes
	att.ExitCode = &req.ExitCode

	status := req.Status
	if req.ExitCode != 0 && status != task.AttemptStatusCanceled {
		status = task.AttemptStatusFailed
	}
	if status == task.AttemptStatusSucceeded && req.AgentReportRequired && !req.AgentReportConfirmed {
		status = task.AttemptStatusFailed
		att.ErrorMessage = "Agent finished without a confirmed final task report"
	}
	if status == task.AttemptStatusSucceeded && s.projectFinder != nil {
		if t, err := s.taskStore.GetTask(ctx, att.TaskID); err == nil && t != nil && t.ProjectID != "" {
			if p, err := s.projectFinder.Get(ctx, t.ProjectID); err == nil && p != nil {
				settings := project.SettingsFromMetadata(p.Metadata)
				isCodeLane := t.ExecutionCategory == "" || t.ExecutionCategory == task.ExecutionCategoryCode
				if settings.IsAutoDeployEnabled() && isCodeLane {
					if !req.LiveDeploymentConfirmed && !hasAutoDeployVerification(t.CompleteNote, t.Summary) {
						status = task.AttemptStatusFailed
						att.ErrorMessage = "Kepatuhan auto_deploy: perubahan wajib di-deploy dan terverifikasi live di production sebelum task dinyatakan selesai; commit/push saja tidak memenuhi syarat"
					}
				}
			}
		}
	}
	att.Status = status
	if att.ErrorMessage == "" {
		att.ErrorMessage = req.ErrorMessage
	}

	if err := s.taskStore.UpdateAttempt(ctx, att); err != nil {
		return nil, fmt.Errorf("update attempt on complete: %w", err)
	}

	// Update task status:
	t, err := s.taskStore.GetTask(ctx, att.TaskID)
	if err == nil && t != nil {
		oldStatus := t.Status

		// 1. Emisi internal event task:attempt_succeeded saat worker sukses
		if status == task.AttemptStatusSucceeded {
			evt := AttemptSucceededEvent{
				EventID:       fmt.Sprintf("evt_%d", time.Now().UnixNano()),
				EventType:     EventTaskAttemptSucceeded,
				TaskID:        t.ID,
				AttemptID:     att.ID,
				AttemptNumber: att.AttemptNumber,
				BranchName:    req.BranchName,
				CommitSHA:     req.CommitSHA,
				ExecutionLane: t.ExecutionCategory,
				Status:        status,
				Summary:       t.Summary,
				CompleteNote:  t.CompleteNote,
				EmittedAt:     now,
				Metadata: map[string]any{
					"runner_id":    req.RunnerID,
					"exit_code":    req.ExitCode,
					"test_summary": req.TestSummary,
				},
			}
			if s.eventBus != nil {
				s.eventBus.Publish(ctx, evt)
			} else {
				EmitAttemptSucceeded(ctx, evt)
			}
		}

		// 2. Evaluasi DoD Otonom & Logika Auto-Mark Done / Auto-Requeue
		dodVal := req.DoDValidation
		if dodVal == nil && status == task.AttemptStatusSucceeded && s.dodEvaluator != nil {
			workingDir := ""
			if s.projectFinder != nil && t.ProjectID != "" {
				if p, err := s.projectFinder.Get(ctx, t.ProjectID); err == nil && p != nil {
					workingDir = p.LocalPath
				}
			}
			valReq := DoDValidationRequest{
				TaskID:        t.ID,
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

		autoRequeued := false
		if status == task.AttemptStatusSucceeded {
			if dodVal != nil && dodVal.Passed {
				// Lolos 100%: Otomatis ubah status task menjadi 'done' dan catat verifikasi
				t.Status = task.TaskStatusDone
				if dodVal.Details != "" {
					t.CompleteNote = dodVal.Details
				}
				for i := range t.DoDChecklist {
					t.DoDChecklist[i].Completed = true
				}
				// Proactive messenger di Chat/Inbox: lapor tugas tuntas dalam bahasa bisnis ramah
				if s.chatNotifier != nil {
					for _, rID := range uniqueRooms(t.SourceRoomID, req.RoomID) {
						_ = s.chatNotifier.SendTaskCompletedBusinessMessage(ctx, rID, t.ID, t.Title, t.Summary)
					}
				}
			} else if dodVal != nil && dodVal.Action == "auto_requeue" && att.AttemptNumber < MaxAttempts {
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
				_ = s.taskStore.CreateAttempt(ctx, newAttempt)

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
				_ = s.taskStore.CreateActivity(ctx, requeueAct)
			} else {
				t.Status = task.TaskStatusReview
				if s.chatNotifier != nil {
					var gaps []string
					if dodVal != nil {
						gaps = dodVal.Gaps
					}
					for _, rID := range uniqueRooms(t.SourceRoomID, req.RoomID) {
						_ = s.chatNotifier.SendTaskReviewBusinessMessage(ctx, rID, t.ID, t.Title, t.Summary, gaps, att.AttemptNumber)
					}
				}
			}
		} else if status == task.AttemptStatusFailed || status == task.AttemptStatusCanceled {
			t.Status = task.TaskStatusBlocked
			if s.chatNotifier != nil {
				errMsg := att.ErrorMessage
				if errMsg == "" {
					errMsg = "Pekerjaan terhenti dan memerlukan peninjauan lebih lanjut."
				}
				for _, rID := range uniqueRooms(t.SourceRoomID, req.RoomID) {
					_ = s.chatNotifier.SendTaskReviewBusinessMessage(ctx, rID, t.ID, t.Title, t.Summary, []string{errMsg}, att.AttemptNumber)
				}
			}
		}

		// Enforce state machine transition guardrails via pkg/taskfsm
		if srcState, errSrc := taskfsm.FromCuanTaskStatus(oldStatus); errSrc == nil {
			if dstState, errDst := taskfsm.FromCuanTaskStatus(t.Status); errDst == nil {
				if !taskfsm.CanTransition(srcState, dstState) && srcState != dstState {
					t.Status = task.TaskStatusReview
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
		_ = s.taskStore.UpdateTask(ctx, t)

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
		_ = s.taskStore.CreateActivity(ctx, act)
	}

	// Reset runner status to idle
	if r, err := s.store.GetRunner(ctx, req.RunnerID); err == nil && r != nil {
		r.Status = StatusIdle
		r.CurrentTaskID = ""
		r.CurrentAttemptID = ""
		_ = s.store.RegisterOrUpdateHeartbeat(ctx, r)
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

func hasAutoDeployVerification(note, summary string) bool {
	combined := strings.ToLower(note + " " + summary)
	return (strings.Contains(combined, "deploy") || strings.Contains(combined, "release") || strings.Contains(combined, "rilis")) &&
		(strings.Contains(combined, "live") || strings.Contains(combined, "production") || strings.Contains(combined, "healthz") || strings.Contains(combined, "smoke"))
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
