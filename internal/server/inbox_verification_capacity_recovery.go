package server

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

// Quota and legacy cold-setup failures share the original verifier recovery
// allowance with browser runtime repair. Reconcile effects and inspect the original source;
// never rerun implementation or assert that the failed provider did no work.
func (s *Server) recoverSourceVerifier(ctx context.Context, d *task.TaskDetailResponse, origin db.InboxRequest) bool {
	if d == nil || d.Status != task.TaskStatusBlocked || d.SourceRoomID != origin.RoomID || d.SourceMessageID != origin.MessageID || !s.sameRuntimeAgent(ctx, origin.AgentID, d.AssignedAgent) || !s.reviewSnapshotRecoveryAuthorized(ctx, d, origin) || !s.taskContinuationAfterCancel(ctx, origin, d) || s.runnerSvc == nil {
		return false
	}
	cause, eventType, eventPrefix := "capacity", task.ActivityVerificationCapacityRecovery, "capacity-recovery:"
	policy := ""
	if !task.QuotaStoppedVerification(d.LatestAttempt) {
		if !task.PHPPreparationStoppedVerification(d.LatestAttempt, d.Activities) {
			return false
		}
		runtime := s.runnerSvc.DependencyPreparationRuntime(ctx, d.LatestAttempt.RunnerID)
		if runtime == nil || runtime.CheckedAt.Before(*d.LatestAttempt.FinishedAt) {
			return false
		}
		cause, eventType, eventPrefix, policy = "php_preparation", task.ActivityVerificationPreparationRecovery, "preparation-recovery:", runtime.Policy
	}
	prior, err := task.ValidateVerificationSource(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID)
	if err != nil || prior == nil || prior.RecoveryAttemptID != "" || prior.FinalizeAttemptID != "" || prior.OwnerID != origin.OwnerID || prior.RoomID != origin.RoomID || prior.SourceRequestID != origin.RequestID {
		return false
	}
	worker, err := s.runnerSvc.GetRunner(ctx, d.LatestAttempt.RunnerID)
	if err != nil || worker == nil || worker.Status == "offline" || worker.LastHeartbeatAt.After(time.Now().Add(5*time.Second)) || time.Since(worker.LastHeartbeatAt) > 2*time.Minute {
		return false
	}
	requestID := task.VerificationRecoveryRequestID(d.ID, prior.SourceCommit)
	if saved, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, requestID)); err == nil && saved != nil {
		proof, err := task.VerificationIntentForLaunch(&d.Task, saved)
		return err == nil && proof != nil && proof.RecoveryCause == cause && proof.RecoveryAttemptID == d.LatestAttempt.ID && (saved.State == "queued" || saved.State == "claimed" || saved.State == "started")
	} else if err != nil && !errors.Is(err, task.ErrTaskNotFound) {
		return false
	}
	reader, ok := s.taskSvc.Store().(task.AttemptLaunchStore)
	if !ok {
		return false
	}
	previous, err := reader.LaunchForAttempt(ctx, d.LatestAttempt.ID)
	if err != nil || previous == nil {
		return false
	}
	// Readiness is observed by native code holding a real task-class slot.
	// The runner still leases capacity again when executing this saved intent.
	_, brain, release, err := agent.LeaseHealthyBrainForTask(origin.OwnerID, nil, s.taskRequiresVision(ctx, origin.OwnerID, d, "", previous.Revision))
	if err != nil {
		return false
	}
	release()
	checked := time.Now().UTC().Truncate(time.Microsecond)
	receipt, _ := json.Marshal(task.VerificationCapacity{OwnerID: origin.OwnerID, AccountID: brain.ID, CheckedAt: checked, PreparationPolicy: policy})
	event := &task.TaskActivity{ID: eventPrefix + d.LatestAttempt.ID, TaskID: d.ID, AttemptID: &d.LatestAttempt.ID, ActivityType: eventType, Author: "coordinator", NewStatus: "ready", Message: string(receipt), CreatedAt: checked}
	proof := *prior
	proof.RecoveryAttemptID = d.LatestAttempt.ID
	proof.RecoveryCause = cause
	_ = s.taskSvc.Store().CreateActivity(ctx, event)
	// Confirm durable evidence even after an uncertain write or duplicate ID.
	events, readErr := s.taskSvc.Store().ListActivities(ctx, d.ID)
	if readErr != nil {
		return false
	}
	validationErr := task.ValidateVerificationCapacityRecovery(ctx, s.taskSvc.Store(), &d.Task, &proof, events)
	if cause == "php_preparation" {
		validationErr = task.ValidateVerificationPreparationRecovery(ctx, s.taskSvc.Store(), &d.Task, &proof, events)
	}
	if validationErr != nil {
		return false
	}
	raw, _ := json.Marshal(proof)
	start := strings.Index(previous.Revision, task.VerificationIntentMarker) + len(task.VerificationIntentMarker)
	end := strings.IndexByte(previous.Revision[start:], '\n')
	if end < 0 {
		end = len(previous.Revision) - start
	}
	reason := "Capacity recovery: the previous verifier stopped on a provider limit; its effects remain uncertain and its failure remains historical."
	if cause == "php_preparation" {
		reason = "Preparation recovery: the previous verifier exhausted its cold Composer setup deadline before model execution. The runner now retains owner-scoped locked archives, isolates vendor/auth and reserves verification time. The failed attempt remains historical."
	}
	revision := previous.Revision[:start] + string(raw) + previous.Revision[start+end:] + "\n" + reason + " Inspect retained files and previous output before repeating checks. Verify only the original exact source and missing evidence. Do not change implementation, publish, perform external writes, waive original criteria or assume the failed provider performed no actions. For required visual evidence use actual application screenshots, never HTML audit mockups."
	s.startAuthorizedLaunch(ctx, origin, d, requestID, prior.SourceRequestID, revision)
	saved, err := s.taskSvc.GetLaunch(ctx, task.LaunchID(d.ID, requestID))
	return err == nil && saved != nil && (saved.State == "queued" || saved.State == "claimed" || saved.State == "started")
}
