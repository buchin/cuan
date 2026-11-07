package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

// ensureNativeReleasePreparation derives every publication input from saved
// approval/source/context. Unknown remote outcomes are reconciled by immutable
// read-first push/merge helpers before a repeated request can mutate them again.
func (s *Server) ensureNativeReleasePreparation(ctx context.Context, taskID, attemptID, runnerID string) (*project.PreparedNativeReleaseWorkspace, error) {
	d, intent, err := s.authorizedNativeReleaseAttempt(ctx, taskID, attemptID, runnerID)
	if err != nil {
		return nil, err
	}
	reuse := func(current *task.TaskDetailResponse) (*project.PreparedNativeReleaseWorkspace, bool, error) {
		for _, event := range current.Activities {
			if event != nil && event.ID == "native-release-preparation:"+attemptID {
				saved, err := task.NativeReleasePreparationForAttempt(ctx, s.taskSvc.Store(), &current.Task, attemptID)
				if err != nil || saved == nil {
					return nil, true, errors.New("canonical native preparation invalid; existing record preserved")
				}
				prepared, err := s.prepareNativeReleaseWorkspace(ctx, taskID, attemptID, runnerID, saved.TargetCommit)
				return prepared, true, err
			}
		}
		return nil, false, nil
	}
	if prepared, found, err := reuse(d); found {
		return prepared, err
	}
	if s.cfg == nil || s.cfg.WorkspacesDir == "" || s.projectSvc == nil || !filepath.IsLocal(taskID) || filepath.Base(taskID) != taskID || d.BranchName == "" {
		return nil, errors.New("native developer source workspace identity unavailable")
	}
	base, err := filepath.EvalSymlinks(s.cfg.WorkspacesDir)
	if err != nil {
		return nil, errors.New("native developer workspace unavailable")
	}
	base, err = filepath.Abs(base)
	if err != nil {
		return nil, err
	}
	sourcePath := filepath.Join(base, taskID)
	real, err := filepath.EvalSymlinks(sourcePath)
	if err != nil || real != sourcePath {
		return nil, errors.New("native developer source must be a private checkout")
	}
	info, err := os.Lstat(sourcePath)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("native developer checkout unavailable")
	}
	projectContext, projectErr := s.projectSvc.TaskContext(ctx, d.ProjectID, d.Instructions)
	if projectErr != nil || projectContext == nil {
		return nil, errors.New("saved source project unavailable")
	}
	cache, cacheErr := filepath.EvalSymlinks(projectContext.LocalPath)
	if cacheErr != nil || cache == sourcePath {
		return nil, errors.New("native developer checkout must be separate from project cache")
	}
	lease, err := git.AcquireVerificationLease(ctx, sourcePath)
	if err != nil {
		return nil, err
	}
	defer lease.Close()
	fresh, current, err := s.authorizedNativeReleaseAttempt(ctx, taskID, attemptID, runnerID)
	if err != nil || current == nil || *current != *intent || fresh.ProjectID != d.ProjectID || fresh.BranchName != d.BranchName {
		return nil, errors.New("native source changed before publication")
	}
	if prepared, found, err := reuse(fresh); found {
		return prepared, err
	}
	if err := git.NewClient().VerifySnapshotCommit(ctx, sourcePath, intent.SourceCommit, intent.SourceCommit); err != nil {
		return nil, err
	}
	pushed, err := s.projectSvc.PushNativeReleaseSource(ctx, d.ProjectID, d.Instructions, intent.Repository, sourcePath, d.BranchName, intent.SourceCommit)
	if err != nil || pushed != intent.SourceCommit {
		return nil, errors.New("native source push was not confirmed; reconcile saved source before retrying")
	}
	fresh, current, err = s.authorizedNativeReleaseAttempt(ctx, taskID, attemptID, runnerID)
	if err != nil || current == nil || *current != *intent || fresh.ProjectID != d.ProjectID || fresh.BranchName != d.BranchName {
		return nil, errors.New("native source authorization changed after push")
	}
	merged, err := s.projectSvc.ApplyNativeReleaseSource(ctx, d.ProjectID, d.Instructions, intent.Repository, intent.TargetBranch, intent.SourceCommit)
	if err != nil || merged == nil || !merged.Merged || !git.IsCommitHash(merged.SHA) {
		return nil, errors.New("native merge outcome not confirmed; reconcile remote branch before retrying")
	}
	return s.prepareNativeReleaseWorkspace(ctx, taskID, attemptID, runnerID, merged.SHA)
}
