package server

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

func (s *Server) authorizedNativeReleaseAttempt(ctx context.Context, taskID, attemptID, runnerID string) (*task.TaskDetailResponse, *task.NativeReleaseIntent, error) {
	d, err := s.taskSvc.GetTaskDetail(ctx, taskID)
	if err != nil || d == nil || d.LatestAttempt == nil {
		return nil, nil, errors.New("native release task unavailable")
	}
	a := d.LatestAttempt
	if a.ID != attemptID || a.RunnerID != runnerID || runnerID == "" || a.Status != task.AttemptStatusRunning || a.CancelRequested || a.CanceledAt != nil || a.StartedAt == nil || (a.LeaseExpiresAt != nil && !a.LeaseExpiresAt.After(time.Now())) {
		return nil, nil, task.ErrRequestConflict
	}
	intent, err := task.NativeReleaseIntentForAttempt(ctx, s.taskSvc.Store(), &d.Task, attemptID)
	if err != nil || intent == nil {
		return nil, nil, errors.New("native release intent unavailable")
	}
	if err := task.ValidateNativeReleaseSource(ctx, s.taskSvc.Store(), &d.Task, intent); err != nil {
		return nil, nil, err
	}
	reader, ok := s.taskSvc.Store().(task.AttemptLaunchStore)
	if !ok {
		return nil, nil, errors.New("native release launch unavailable")
	}
	launch, err := reader.LaunchForAttempt(ctx, attemptID)
	if err != nil || launch == nil {
		return nil, nil, errors.New("native release launch unavailable")
	}
	trigger, authorized, err := s.authorizeSavedTaskLaunch(ctx, *launch, d)
	if err != nil || !authorized || !s.taskContinuationAfterCancel(ctx, trigger, d) {
		return nil, nil, errors.New("original release authorization unavailable")
	}
	return d, intent, nil
}

// prepareNativeReleaseWorkspace is native coordination, not a caller-controlled
// HTTP operation. target must come from the coordinator's reconciled merge
// receipt. This method performs no merge/deploy and never repairs prior records.
func (s *Server) prepareNativeReleaseWorkspace(ctx context.Context, taskID, attemptID, runnerID, target string) (*project.PreparedNativeReleaseWorkspace, error) {
	if !git.IsCommitHash(target) || s.projectSvc == nil {
		return nil, errors.New("native release target unavailable")
	}
	d, intent, err := s.authorizedNativeReleaseAttempt(ctx, taskID, attemptID, runnerID)
	if err != nil {
		return nil, err
	}
	projectContext, contextErr := s.projectSvc.TaskContext(ctx, d.ProjectID, d.Instructions)
	if contextErr != nil || projectContext == nil {
		return nil, errors.New("native preparation repository context unavailable")
	}
	lease, leaseErr := git.AcquireVerificationLease(ctx, projectContext.LocalPath)
	if leaseErr != nil {
		return nil, leaseErr
	}
	defer lease.Close()
	d, intent, err = s.authorizedNativeReleaseAttempt(ctx, taskID, attemptID, runnerID)
	if err != nil {
		return nil, err
	}
	for _, event := range d.Activities {
		if event == nil || event.ID != "native-release-preparation:"+attemptID {
			continue
		}
		saved, err := task.NativeReleasePreparationForAttempt(ctx, s.taskSvc.Store(), &d.Task, attemptID)
		if err != nil || saved == nil || saved.TargetCommit != target {
			return nil, errors.New("saved native preparation differs; existing record preserved")
		}
	}
	prepared, err := s.projectSvc.NativeReleaseWorkspace(ctx, project.NativeReleaseWorkspaceRequest{ProjectID: d.ProjectID, TaskID: d.ID, Instructions: d.Instructions, Repository: intent.Repository, TargetBranch: intent.TargetBranch, SourceCommit: intent.SourceCommit, TargetCommit: target, ManifestHash: intent.ManifestHash})
	if err != nil {
		return nil, err
	}
	fresh, current, err := s.authorizedNativeReleaseAttempt(ctx, taskID, attemptID, runnerID)
	if err != nil || current == nil || *current != *intent || fresh.ProjectID != d.ProjectID || fresh.Instructions != d.Instructions {
		return nil, errors.New("native release context changed during preparation")
	}
	currentProject, projectErr := s.projectSvc.TaskContext(ctx, fresh.ProjectID, fresh.Instructions)
	if projectErr != nil || currentProject == nil || currentProject.IsArchived || currentProject.LocalPath != projectContext.LocalPath || currentProject.RepoURL != projectContext.RepoURL || currentProject.DefaultBranch != projectContext.DefaultBranch || string(currentProject.Metadata) != string(projectContext.Metadata) {
		return nil, errors.New("native release project changed during preparation")
	}
	receipt := task.NativeReleasePreparation{Version: 1, TaskID: taskID, AttemptID: attemptID, SourceCommit: prepared.SourceCommit, TargetCommit: prepared.TargetCommit, InstructionsHash: intent.InstructionsHash, Repository: intent.Repository, TargetBranch: prepared.Branch, ManifestHash: prepared.ManifestHash}
	raw, _ := json.Marshal(receipt)
	// Existing valid records are immutable. Reuse instead of rewriting their
	// timestamps or creating a second apparent preparation after a lost response.
	for _, event := range fresh.Activities {
		if event != nil && event.ID == "native-release-preparation:"+attemptID {
			saved, err := task.NativeReleasePreparationForAttempt(ctx, s.taskSvc.Store(), &fresh.Task, attemptID)
			if err != nil || saved == nil || *saved != receipt {
				return nil, errors.New("native preparation changed before persistence")
			}
			return prepared, nil
		}
	}
	event := &task.TaskActivity{ID: "native-release-preparation:" + attemptID, TaskID: taskID, AttemptID: &attemptID, Author: "cuan-native-release", ActivityType: task.ActivityNativeReleasePreparation, Message: string(raw), CreatedAt: time.Now().UTC()}
	if err := s.taskSvc.Store().CreateActivity(ctx, event); err != nil {
		return nil, err
	}
	if _, err := task.NativeReleasePreparationForAttempt(ctx, s.taskSvc.Store(), &fresh.Task, attemptID); err != nil {
		return nil, err
	}
	return prepared, nil
}
