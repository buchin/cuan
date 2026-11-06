package task

import (
	"context"

	"github.com/buchin/cuan/internal/executionpolicy"
)

// ExecutionInfo describes the exact saved attempt, not the newest pending
// launch. Parsed intent and scope are diagnostic data, never acceptance proof.
// Deliberately exclude revision prose, environment, credentials and owner data.
type ExecutionInfo struct {
	TaskID          string                            `json:"task_id"`
	AttemptID       string                            `json:"attempt_id,omitempty"`
	AttemptStatus   string                            `json:"attempt_status,omitempty"`
	LaunchState     string                            `json:"launch_state,omitempty"`
	IntentKind      string                            `json:"intent_kind"`
	IntentParsed    bool                              `json:"intent_parsed"`
	SourceAttemptID string                            `json:"source_attempt_id,omitempty"`
	SourceCommit    string                            `json:"source_commit,omitempty"`
	TargetCommit    string                            `json:"target_commit,omitempty"`
	Scope           executionpolicy.PublicationPolicy `json:"scope"`
}

func (s *Service) ExecutionInfo(ctx context.Context, taskID string) (*ExecutionInfo, error) {
	t, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	a, err := s.store.GetLatestAttempt(ctx, taskID)
	if err != nil {
		return nil, err
	}
	info := &ExecutionInfo{TaskID: taskID, IntentKind: "no_attempt"}
	if a == nil {
		return info, nil
	}
	info.AttemptID, info.AttemptStatus = a.ID, a.Status
	info.Scope, err = AttemptPublicationPolicy(ctx, s.store, t, a.ID)
	if err != nil {
		return nil, err
	}
	reader, ok := s.store.(AttemptLaunchStore)
	if !ok {
		info.IntentKind = "legacy"
		return info, nil
	}
	j, err := reader.LaunchForAttempt(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	if j == nil {
		info.IntentKind = "legacy"
		return info, nil
	}
	info.LaunchState = j.State
	inspection, err := LiveInspectionForLaunch(t, j)
	if err != nil {
		info.IntentKind = "invalid"
		return info, nil
	}
	verification, err := VerificationIntentForLaunch(t, j)
	if err != nil {
		info.IntentKind = "invalid"
		return info, nil
	}
	info.IntentParsed = true
	switch {
	case inspection != nil:
		info.IntentKind = "live_result_inspection"
		if inspection.Purpose == CurrentOutcomeInspection {
			info.IntentKind = "current_outcome_inspection"
		}
		info.SourceAttemptID, info.SourceCommit, info.TargetCommit = inspection.SourceAttemptID, inspection.SourceCommit, inspection.TargetCommit
	case verification != nil:
		info.IntentKind = "verification"
		info.SourceAttemptID, info.SourceCommit = verification.SourceAttemptID, verification.SourceCommit
	case j.InstructionsHash != LaunchInstructionsHash(t.Instructions):
		info.IntentKind, info.IntentParsed = "invalid", false
	default:
		info.IntentKind = "execution"
	}
	return info, nil
}
