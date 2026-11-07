package runner

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func (s *RunnerService) prepareNativeReleaseCompletion(ctx context.Context, t *task.Task, att *task.TaskAttempt, req CompleteAttemptRequest, now time.Time) (*task.TaskActivity, error) {
	intent, err := task.NativeReleaseIntentForAttempt(ctx, s.taskStore, t, att.ID)
	if err != nil {
		return nil, err
	}
	if intent == nil {
		if req.NativeReleaseProcess != nil {
			return nil, errors.New("native process result has no admitted release intent")
		}
		return nil, nil
	}
	prep, err := task.NativeReleasePreparationForAttempt(ctx, s.taskStore, t, att.ID)
	if err != nil || prep == nil {
		return nil, errors.New("native release preparation unavailable at completion")
	}
	result := req.NativeReleaseProcess
	if result == nil || result.ExitCode != 0 || result.Error != "" || !result.SourceUnchanged || result.Commit != prep.TargetCommit || result.Branch != prep.TargetBranch || att.StartedAt == nil || result.StartedAt.Before(*att.StartedAt) || result.FinishedAt.Before(result.StartedAt) || result.FinishedAt.After(now) || req.ExitCode != 0 || req.AgentReportRequired || req.AgentReportConfirmed || req.CommitSHA != "" || req.BranchName != "" || req.PRURL != "" || req.PRNumber != 0 || req.DiffSummary != "" || req.ManagedReleaseSource != nil || (req.HasCodeChanges != nil && *req.HasCodeChanges) {
		return nil, errors.New("native release process facts do not match prepared target")
	}
	if s.projectFinder == nil {
		return nil, errors.New("saved native release project unavailable")
	}
	p, err := s.projectFinder.TaskContext(ctx, t.ProjectID, t.Instructions)
	if err != nil || p == nil || p.IsArchived {
		return nil, errors.New("saved native release project unavailable")
	}
	owner, repo, err := git.ParseGitHubURL(p.RepoURL)
	settings := project.SettingsFromMetadata(p.Metadata)
	branch := strings.TrimSpace(settings.DeployBranch)
	if branch == "" {
		branch = p.DefaultBranch
	}
	if err != nil || owner+"/"+repo != prep.Repository || branch != prep.TargetBranch || !settings.IsAutoDeployEnabled() || project.ParseEnvironment(settings.Environment)["CUAN_MANAGED_RELEASE"] != "1" {
		return nil, errors.New("native release project differs from preparation")
	}
	activities, err := s.taskStore.ListActivities(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	if !task.VerifiedDeployment(activities, att, prep.TargetCommit, now) {
		return nil, errors.New("native release requires independent target deployment observation")
	}
	preparedBeforeProcess := false
	for _, event := range activities {
		if event != nil && event.ID == "native-release-preparation:"+att.ID && !event.CreatedAt.After(result.StartedAt) {
			preparedBeforeProcess = true
		}
	}
	if !preparedBeforeProcess {
		return nil, errors.New("native release process predates preparation")
	}
	observedAfterProcess := false
	for _, event := range activities {
		if event != nil && event.ID == task.DeploymentReceiptID(t.ID, att.ID, prep.TargetCommit) && !event.CreatedAt.Before(result.FinishedAt) {
			observedAfterProcess = true
		}
	}
	if !observedAfterProcess {
		return nil, errors.New("native deployment observation predates the release process")
	}
	delivery := task.NativeReleaseDelivery{Version: 1, TaskID: t.ID, AttemptID: att.ID, SourceAttemptID: intent.SourceAttemptID, SourceCommit: prep.SourceCommit, TargetCommit: prep.TargetCommit, InstructionsHash: prep.InstructionsHash, Repository: prep.Repository, TargetBranch: prep.TargetBranch, ManifestHash: prep.ManifestHash, ProcessStartedAt: result.StartedAt, ProcessFinishedAt: result.FinishedAt}
	raw, _ := json.Marshal(delivery)
	return &task.TaskActivity{ID: "native-release-delivery:" + att.ID, TaskID: t.ID, AttemptID: &att.ID, Author: att.RunnerID, ActivityType: "native_release_delivery", Message: string(raw), CreatedAt: now}, nil
}
