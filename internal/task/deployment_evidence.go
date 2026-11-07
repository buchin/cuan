package task

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

type DeploymentReceipt struct {
	Version     int    `json:"version"`
	TaskID      string `json:"task_id"`
	AttemptID   string `json:"attempt_id"`
	CommitSHA   string `json:"commit_sha"`
	ReleaseSHA  string `json:"release_sha"`
	Environment string `json:"environment"`
	Health      string `json:"health"`
}

// This matches the trusted activation publisher, not the agent report endpoint.
func DeploymentReceiptID(taskID, attemptID, commit string) string {
	digest := sha256.Sum256([]byte(taskID + "\x00" + attemptID + "\x00" + commit))
	return "act_deploy_" + hex.EncodeToString(digest[:16])
}

func validDeploymentCommit(commit string) bool {
	if len(commit) != 40 || commit != strings.ToLower(commit) {
		return false
	}
	_, err := hex.DecodeString(commit)
	return err == nil
}

// VerifiedDeployment requires the exact publisher event, attempt and expected
// repository commit. Prose, an agent metadata object and old receipts cannot
// substitute for this deployment fact. Remote durability and general host
// isolation are outside this activity-store trust boundary.
func VerifiedDeployment(activities []*TaskActivity, attempt *TaskAttempt, expectedCommit string, now time.Time) bool {
	if attempt == nil || attempt.ID == "" || attempt.TaskID == "" || !validDeploymentCommit(expectedCommit) {
		return false
	}
	started := attempt.CreatedAt
	if attempt.StartedAt != nil {
		started = *attempt.StartedAt
	}
	if started.IsZero() {
		return false
	}
	for _, a := range activities {
		if a == nil || a.TaskID != attempt.TaskID || a.AttemptID == nil || *a.AttemptID != attempt.ID || a.ActivityType != "deployment_verified" || a.Author != "cuan-deploy" || a.ID != DeploymentReceiptID(attempt.TaskID, attempt.ID, expectedCommit) || a.CreatedAt.Before(started) || a.CreatedAt.After(now) {
			continue
		}
		if attempt.FinishedAt != nil && a.CreatedAt.After(*attempt.FinishedAt) {
			continue
		}
		var receipt DeploymentReceipt
		if json.Unmarshal([]byte(a.Message), &receipt) != nil {
			continue
		}
		if receipt.Version == 1 && receipt.TaskID == attempt.TaskID && receipt.AttemptID == attempt.ID && receipt.CommitSHA == expectedCommit && receipt.ReleaseSHA == expectedCommit && receipt.Environment == "production" && receipt.Health == "healthy" {
			return true
		}
	}
	return false
}

// PostProcessDeploymentReceiptID separates an activation-time receipt from the
// independent observation requested after a particular native process finishes.
func PostProcessDeploymentReceiptID(taskID, attemptID, commit string, finished time.Time) string {
	sum := sha256.Sum256([]byte(taskID + "\x00" + attemptID + "\x00" + commit + "\x00" + finished.UTC().Format(time.RFC3339Nano)))
	return "act_observe_" + hex.EncodeToString(sum[:16])
}

func VerifiedPostProcessDeployment(activities []*TaskActivity, attempt *TaskAttempt, commit string, finished, now time.Time) bool {
	if attempt == nil || finished.IsZero() || finished.After(now) {
		return false
	}
	for _, event := range activities {
		if event == nil || event.CreatedAt.Before(finished) {
			continue
		}
		if event.ID == DeploymentReceiptID(attempt.TaskID, attempt.ID, commit) && VerifiedDeployment([]*TaskActivity{event}, attempt, commit, now) {
			return true
		}
		if event.ID != PostProcessDeploymentReceiptID(attempt.TaskID, attempt.ID, commit, finished) || event.ActivityType != "native_publication_observed" {
			continue
		}
		normalized := *event
		normalized.ID = DeploymentReceiptID(attempt.TaskID, attempt.ID, commit)
		normalized.ActivityType = "deployment_verified"
		if VerifiedDeployment([]*TaskActivity{&normalized}, attempt, commit, now) {
			return true
		}
	}
	return false
}
