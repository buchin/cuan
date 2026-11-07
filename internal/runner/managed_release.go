package runner

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

const managedReleaseClaimActivity = "managed_release_claim"

// ManagedReleaseClaim freezes native publication ownership at admission. It is
// not a release receipt and does not exempt completion from deployment checks.
type ManagedReleaseClaim struct {
	Version          int    `json:"version"`
	TaskID           string `json:"task_id"`
	AttemptID        string `json:"attempt_id"`
	RunnerID         string `json:"runner_id"`
	InstructionsHash string `json:"instructions_hash"`
	Repository       string `json:"repository"`
	TargetBranch     string `json:"target_branch"`
}

func (s *RunnerService) bindManagedReleaseClaim(ctx context.Context, req ClaimTaskRequest, claim *ClaimTaskResponse) error {
	if claim == nil || !claim.Claimed || claim.Task == nil || claim.Attempt == nil || !req.SupportsNativeRelease || !claim.AutoDeploy || claim.NativeRelease != nil || claim.Verification != nil || claim.LiveInspection != nil {
		return nil
	}
	t, att := claim.Task, claim.Attempt
	limits := claim.PublicationLimits
	if project.ParseEnvironment(claim.Environment)["CUAN_MANAGED_RELEASE"] != "1" || t.ProjectID == "" || t.SourceRoomID == "" || t.ExecutionCategory != task.ExecutionCategoryCode || limits.DenyPush || limits.DenyDeploy || limits.ReadOnlyRepository || executionpolicy.StandaloneArtifactWork(t.Instructions) {
		return nil
	}
	if att.TaskID != t.ID || att.RunnerID != req.RunnerID || att.Status != task.AttemptStatusRunning || att.StartedAt == nil || att.CancelRequested || att.CanceledAt != nil || claim.RepoOwner == "" || claim.RepoName == "" || strings.TrimSpace(claim.DeployBranch) == "" {
		return errors.New("managed release admission identity is unavailable")
	}
	binding := &ManagedReleaseClaim{Version: 1, TaskID: t.ID, AttemptID: att.ID, RunnerID: att.RunnerID, InstructionsHash: task.LaunchInstructionsHash(t.Instructions), Repository: claim.RepoOwner + "/" + claim.RepoName, TargetBranch: claim.DeployBranch}
	raw, err := json.Marshal(binding)
	if err != nil {
		return err
	}
	if err := s.taskStore.CreateActivity(ctx, &task.TaskActivity{ID: "managed-release-claim:" + att.ID, TaskID: t.ID, AttemptID: &att.ID, ActivityType: managedReleaseClaimActivity, Author: att.RunnerID, Message: string(raw), CreatedAt: time.Now().UTC()}); err != nil {
		return err
	}
	claim.ManagedRelease = binding
	return nil
}
