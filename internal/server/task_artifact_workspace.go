package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
)

// Native code workers prepare a second isolated checkout after claim. The
// original grant points to the project clone; reports and publication must use
// the worker's actual task checkout, never an older project-clone file/commit.
// Paths derive only from server configuration and the authenticated task. Local
// sessions and independently bound inspection snapshots retain their own roots.
func (s *Server) taskExecutionRoot(ctx context.Context, grant taskAgentGrant, detail *task.TaskDetailResponse) (*os.Root, func(), error) {
	keep := func() {}
	if grant.ArtifactRoot == nil || s.cfg == nil || s.cfg.WorkspacesDir == "" || s.runnerSvc == nil || s.projectSvc == nil || detail == nil || detail.LatestAttempt == nil || (detail.ExecutionCategory != "" && detail.ExecutionCategory != task.ExecutionCategoryCode) {
		return grant.ArtifactRoot, keep, nil
	}
	attempt := detail.LatestAttempt
	if attempt.ID != grant.AttemptID || attempt.TaskID != grant.TaskID {
		return nil, keep, errors.New("The artifact report does not match its bound attempt")
	}
	if attempt.RunnerID == "local_terminal" {
		return grant.ArtifactRoot, keep, nil
	}
	registered, err := s.runnerSvc.GetRunner(ctx, attempt.RunnerID)
	if err != nil || registered == nil {
		return grant.ArtifactRoot, keep, nil
	}
	proof, proofErr := task.ValidateVerificationSource(ctx, s.taskSvc.Store(), &detail.Task, attempt.ID)
	if proofErr != nil {
		return nil, keep, errors.New("The verification source could not be validated")
	}
	if proof != nil && proof.IsolatedWorkspace {
		base, err := filepath.EvalSymlinks(s.cfg.WorkspacesDir)
		if err != nil {
			return nil, keep, errors.New("The bound verification workspace is unavailable")
		}
		path := git.VerificationWorkspacePath(base, detail.ID, proof.SourceCommit)
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil || resolved != path {
			return nil, keep, errors.New("The bound verification snapshot path is invalid")
		}
		if err := git.NewClient().VerifySnapshotCommit(ctx, path, proof.SourceCommit, proof.SourceCommit); err != nil {
			return nil, keep, errors.New("The bound verification snapshot is unavailable or changed")
		}
		root, err := os.OpenRoot(path)
		if err != nil {
			return nil, keep, errors.New("The bound verification snapshot could not be opened")
		}
		return root, func() { _ = root.Close() }, nil
	}
	project, err := s.projectSvc.TaskWorkspace(ctx, detail.ProjectID, detail.Instructions)
	if err != nil || project == nil {
		return nil, keep, errors.New("The saved task project workspace could not be verified")
	}
	bound, boundErr := filepath.EvalSymlinks(grant.ArtifactRoot.Name())
	original, originalErr := filepath.EvalSymlinks(project.LocalPath)
	if boundErr != nil || originalErr != nil {
		return nil, keep, errors.New("The bound task workspace is unavailable")
	}
	if bound != original {
		return grant.ArtifactRoot, keep, nil // Explicit local/inspection workspace.
	}
	if !filepath.IsLocal(grant.TaskID) || filepath.Base(grant.TaskID) != grant.TaskID {
		return nil, keep, errors.New("The bound task workspace identity is invalid")
	}
	base, err := filepath.EvalSymlinks(s.cfg.WorkspacesDir)
	if err != nil {
		return nil, keep, errors.New("The native task workspace is not available yet; saved files are preserved")
	}
	path := filepath.Join(base, grant.TaskID)
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, keep, errors.New("The native task workspace is not available yet; saved files are preserved")
	}
	gitInfo, err := os.Lstat(filepath.Join(path, ".git"))
	if err != nil || !gitInfo.IsDir() || gitInfo.Mode()&os.ModeSymlink != 0 {
		return nil, keep, errors.New("The native task repository could not be verified")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, keep, errors.New("The native task workspace could not be opened")
	}
	return root, func() { _ = root.Close() }, nil
}
