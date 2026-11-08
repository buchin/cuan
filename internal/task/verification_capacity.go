package task

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

const ActivityVerificationCapacityRecovery = "verification_capacity_recovery"
const ActivityVerificationPreparationRecovery = "verification_setup_recovery"
const PHPPreparationPolicy = "locked-php-owner-cache-v1"

type DependencyPreparationRuntime struct {
	Policy    string    `json:"policy"`
	CheckedAt time.Time `json:"checked_at"`
}

func (r *DependencyPreparationRuntime) Fresh(now time.Time) bool {
	return r != nil && r.Policy == PHPPreparationPolicy && !r.CheckedAt.Before(now.Add(-2*time.Minute)) && !r.CheckedAt.After(now.Add(5*time.Second))
}

// Recognize only the legacy two-minute cold-download boundary with no native
// checks or model result. New preparation errors include an explicit diagnosis;
// the same policy cannot manufacture another retry after its own failure.
func PHPPreparationStoppedVerification(a *TaskAttempt, events []*TaskActivity) bool {
	return a != nil && a.StartedAt != nil && a.FinishedAt != nil && a.FinishedAt.Sub(*a.StartedAt) >= 2*time.Minute && a.FinishedAt.Sub(*a.StartedAt) <= 140*time.Second && strings.HasPrefix(a.ErrorMessage, InspectionDependencyFailurePrefix+": PHP locked dependency preparation: ") && strings.Contains(a.ErrorMessage, "- Downloading ") && InspectionInfrastructureFailure(a, events)
}

// Admission evidence from the coordinator's actual account-slot lease. It is
// not provider execution/quota success, source integrity, or task acceptance.
type VerificationCapacity struct {
	PreparationPolicy string    `json:"preparation_policy,omitempty"`
	OwnerID           string    `json:"owner_id"`
	AccountID         string    `json:"account_id"`
	CheckedAt         time.Time `json:"checked_at"`
}

func QuotaStoppedVerification(a *TaskAttempt) bool {
	return a != nil && a.Status == AttemptStatusFailed && !a.CancelRequested && a.CanceledAt == nil && a.FinishedAt != nil && a.ExitCode != nil && *a.ExitCode == 75 && strings.HasPrefix(a.ErrorMessage, "The AI provider reported a usage limit. The run was stopped;")
}

// ValidateVerificationCapacityRecovery shares the one-per-source identity with
// runtime repair. It cannot chain, change source/scope or accept its failed parent.
func ValidateVerificationCapacityRecovery(ctx context.Context, store Store, t *Task, proof *VerificationIntent, activities []*TaskActivity) error {
	return validateVerificationAdmissionRecovery(ctx, store, t, proof, activities, "capacity")
}

func ValidateVerificationPreparationRecovery(ctx context.Context, store Store, t *Task, proof *VerificationIntent, activities []*TaskActivity) error {
	return validateVerificationAdmissionRecovery(ctx, store, t, proof, activities, "php_preparation")
}

func validateVerificationAdmissionRecovery(ctx context.Context, store Store, t *Task, proof *VerificationIntent, activities []*TaskActivity, cause string) error {
	if t == nil || proof == nil || proof.RecoveryCause != cause || proof.RecoveryAttemptID == "" || proof.RuntimeFingerprint != "" || proof.FinalizeAttemptID != "" {
		return ErrInvalidTaskInput
	}
	parent, err := store.GetAttempt(ctx, proof.RecoveryAttemptID)
	if err != nil {
		return err
	}
	eligible := QuotaStoppedVerification(parent)
	eventType, eventID := ActivityVerificationCapacityRecovery, "capacity-recovery:"
	if cause == "php_preparation" {
		eligible = PHPPreparationStoppedVerification(parent, activities)
		eventType, eventID = ActivityVerificationPreparationRecovery, "preparation-recovery:"
	}
	if !eligible || parent.TaskID != t.ID {
		return ErrInvalidTaskInput
	}
	original, err := VerificationIntentForAttempt(ctx, store, t, parent.ID)
	if err != nil || original == nil || original.RecoveryAttemptID != "" || original.RecoveryCause != "" || original.FinalizeAttemptID != "" {
		return ErrInvalidTaskInput
	}
	expected := *proof
	expected.RecoveryAttemptID, expected.RecoveryCause = "", ""
	// The original review remains the authority for the missing evidence.
	left, _ := json.Marshal(expected)
	right, _ := json.Marshal(original)
	if string(left) != string(right) {
		return ErrInvalidTaskInput
	}
	for _, event := range activities {
		if event == nil || event.ID != eventID+parent.ID || event.TaskID != t.ID || event.AttemptID == nil || *event.AttemptID != parent.ID || event.ActivityType != eventType || event.Author != "coordinator" || event.NewStatus != "ready" {
			continue
		}
		var receipt VerificationCapacity
		if json.Unmarshal([]byte(event.Message), &receipt) != nil || receipt.OwnerID != proof.OwnerID || !strings.HasPrefix(receipt.AccountID, "acct_") || len(receipt.AccountID) <= 5 || receipt.CheckedAt.Before(*parent.FinishedAt) || receipt.CheckedAt.After(time.Now().UTC().Add(5*time.Second)) || event.CreatedAt.After(time.Now().UTC().Add(5*time.Second)) || event.CreatedAt.Before(receipt.CheckedAt) || event.CreatedAt.Sub(receipt.CheckedAt) > time.Minute {
			continue
		}
		if (cause == "php_preparation" && receipt.PreparationPolicy != PHPPreparationPolicy) || (cause == "capacity" && receipt.PreparationPolicy != "") {
			continue
		}
		return nil
	}
	return ErrInvalidTaskInput
}
