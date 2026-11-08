package task

import (
	"context"
	"errors"

	"github.com/buchin/cuan/internal/executionpolicy"
)

// ExecutionInfo describes the exact saved attempt, not the newest pending
// launch. Parsed intent and scope are diagnostic data, never acceptance proof.
// Deliberately exclude revision prose, environment, credentials and owner data.
type ExecutionInfo struct {
	TaskID             string                            `json:"task_id"`
	AttemptID          string                            `json:"attempt_id,omitempty"`
	AttemptStatus      string                            `json:"attempt_status,omitempty"`
	LaunchState        string                            `json:"launch_state,omitempty"`
	IntentKind         string                            `json:"intent_kind"`
	IntentParsed       bool                              `json:"intent_parsed"`
	SourceAttemptID    string                            `json:"source_attempt_id,omitempty"`
	SourceCommit       string                            `json:"source_commit,omitempty"`
	TargetCommit       string                            `json:"target_commit,omitempty"`
	RecoveryAttemptID  string                            `json:"recovery_attempt_id,omitempty"`
	RecoveryCause      string                            `json:"recovery_cause,omitempty"`
	RevalidationKind   string                            `json:"revalidation_kind,omitempty"`
	Scope              executionpolicy.PublicationPolicy `json:"scope"`
	LatestLaunch       *LaunchDiagnostic                 `json:"latest_launch,omitempty"`
	VerifierHandoff    *VerifierHandoffDiagnostic        `json:"verifier_handoff,omitempty"`
	LiveInspectionPlan *InspectionPlanDiagnostic         `json:"live_inspection_plan,omitempty"`
}

// VerifierHandoffDiagnostic exposes native routing facts, never source prose,
// credentials or permission to execute a correction.
type VerifierHandoffDiagnostic struct {
	Code                  string                             `json:"code"`
	SourceVerified        bool                               `json:"source_verified"`
	HistoricalLiveFailure bool                               `json:"historical_live_failure"`
	ApprovedScope         *executionpolicy.PublicationPolicy `json:"approved_scope,omitempty"`
	RunningCommit         string                             `json:"running_commit,omitempty"`
	PublicationRequired   *bool                              `json:"publication_required,omitempty"`
	SourceContained       *bool                              `json:"source_contained,omitempty"`
}

type InspectionPlanDiagnostic struct {
	ExistingAdmission          *LaunchDiagnostic `json:"existing_admission,omitempty"`
	ExistingAdmissionStopCode  string            `json:"existing_admission_stop_code,omitempty"`
	Code                       string            `json:"code"`
	SourceAttemptID            string            `json:"source_attempt_id,omitempty"`
	SupplementalSourceVerified bool              `json:"supplemental_source_verified"`
	TargetCommit               string            `json:"target_commit,omitempty"`
}

// Latest admission is separate from the attempt's binding: it may have stopped
// before any worker existed. Never expose its revision or stored free-text error.
type LaunchDiagnostic struct {
	Scope          *executionpolicy.PublicationPolicy `json:"scope,omitempty"`
	RevisionRunes  int                                `json:"revision_runes,omitempty"`
	ID             string                             `json:"id"`
	State          string                             `json:"state"`
	IntentKind     string                             `json:"intent_kind"`
	ValidationCode string                             `json:"validation_code,omitempty"`
}

// DiagnoseLaunch reports only fixed classifications and identifiers, never the
// revision, free-text error, or credentials retained by a saved admission.
func DiagnoseLaunch(t *Task, j *TaskLaunch) *LaunchDiagnostic { return diagnoseLaunch(t, j) }

func diagnoseLaunch(t *Task, j *TaskLaunch) *LaunchDiagnostic {
	if j == nil {
		return nil
	}
	d := &LaunchDiagnostic{ID: j.ID, State: j.State, IntentKind: "execution", RevisionRunes: len([]rune(j.Revision))}
	inspection, err := LiveInspectionForLaunch(t, j)
	if err != nil {
		d.IntentKind, d.ValidationCode = "invalid", "invalid_live_inspection"
		if code, ok := err.(liveInspectionValidationError); ok {
			d.ValidationCode = string(code)
		}
		return d
	}
	verification, err := VerificationIntentForLaunch(t, j)
	if err != nil {
		d.IntentKind, d.ValidationCode = "invalid", "invalid_verification"
		return d
	}
	nativeRelease, err := NativeReleaseIntentForLaunch(t, j)
	if err != nil {
		d.IntentKind, d.ValidationCode = "invalid", "invalid_native_release"
		return d
	}
	switch {
	case nativeRelease != nil:
		d.IntentKind = "native_release"
	case inspection != nil:
		d.IntentKind = "live_result_inspection"
		if inspection.Purpose == CurrentOutcomeInspection {
			d.IntentKind = "current_outcome_inspection"
		}
	case verification != nil:
		d.IntentKind = "verification"
	case j.InstructionsHash != LaunchInstructionsHash(t.Instructions):
		d.IntentKind, d.ValidationCode = "invalid", "instructions_changed"
	}
	if d.IntentKind != "invalid" {
		policy := launchPublicationPolicy(t, j, verification != nil)
		d.Scope = &policy
	}
	return d
}

func (s *Service) ExecutionInfo(ctx context.Context, taskID string, selectedAttempt ...string) (*ExecutionInfo, error) {
	t, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	a, err := s.store.GetLatestAttempt(ctx, taskID)
	if len(selectedAttempt) > 0 && selectedAttempt[0] != "" {
		a, err = s.store.GetAttempt(ctx, selectedAttempt[0])
		if errors.Is(err, ErrAttemptNotFound) {
			return nil, ErrTaskNotFound
		}
		if err == nil && (a == nil || a.TaskID != taskID) {
			return nil, ErrTaskNotFound
		}
	}
	if err != nil {
		return nil, err
	}
	info := &ExecutionInfo{TaskID: taskID, IntentKind: "no_attempt"}
	if reader, ok := s.store.(LaunchStore); ok {
		latest, err := reader.LatestLaunch(ctx, taskID)
		if err != nil {
			return nil, err
		}
		info.LatestLaunch = diagnoseLaunch(t, latest)
	}
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
	nativeRelease, err := NativeReleaseIntentForLaunch(t, j)
	if err != nil {
		info.IntentKind = "invalid"
		return info, nil
	}
	info.IntentParsed = true
	switch {
	case nativeRelease != nil:
		info.IntentKind = "native_release"
		info.SourceAttemptID, info.SourceCommit = nativeRelease.SourceAttemptID, nativeRelease.SourceCommit
	case inspection != nil:
		info.RevalidationKind = inspection.RevalidationKind
		info.IntentKind = "live_result_inspection"
		if inspection.Purpose == CurrentOutcomeInspection {
			info.IntentKind = "current_outcome_inspection"
		}
		info.SourceAttemptID, info.SourceCommit, info.TargetCommit = inspection.SourceAttemptID, inspection.SourceCommit, inspection.TargetCommit
	case verification != nil:
		info.IntentKind = "verification"
		info.SourceAttemptID, info.SourceCommit = verification.SourceAttemptID, verification.SourceCommit
		info.RecoveryAttemptID, info.RecoveryCause = verification.RecoveryAttemptID, verification.RecoveryCause
	case j.InstructionsHash != LaunchInstructionsHash(t.Instructions):
		info.IntentKind, info.IntentParsed = "invalid", false
	default:
		info.IntentKind = "execution"
	}
	return info, nil
}
