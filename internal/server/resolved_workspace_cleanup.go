package server

import (
	"context"
	"log"
	"path/filepath"

	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

// Only repo-backed task copies are disposable. Chat attachment storage and
// standalone artifact tasks never enter this path. No archive or new clone.
func (s *Server) releaseResolvedTaskCopies(ctx context.Context, item *task.TaskItemResponse) {
	if s.cfg == nil || s.cfg.WorkspacesDir == "" || s.projectStudioRepo == nil || s.terminalManager == nil || item.SourceRoomID == "" {
		return
	}
	lease, err := s.terminalManager.AcquireTaskExecutionLease(item.ID)
	if err != nil {
		return
	}
	defer lease.Close()
	detail, err := s.taskSvc.GetTaskDetail(ctx, item.ID)
	if err != nil || detail == nil || (detail.Status != "done" && detail.Status != "cancelled" && detail.Status != "canceled") || detail.ProjectID == "" {
		return
	}
	room, err := s.projectStudioRepo.GetRoomByID(ctx, detail.SourceRoomID)
	if err != nil || room.ResolvedAt == nil {
		return
	}
	messages, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, room.ID)
	if err != nil {
		return
	}
	for _, message := range messages {
		if message.CreatedAt.After(*room.ResolvedAt) {
			return
		}
	}
	if reason, err := s.conversationResolutionBlocker(ctx, room.ID); err != nil || reason != "" {
		return
	}
	p, err := s.projectSvc.TaskWorkspace(ctx, detail.ProjectID, detail.Instructions)
	if err != nil || p.RepoURL == "" {
		return
	}
	if detail.LatestAttempt != nil && (detail.LatestAttempt.Status == "queued" || detail.LatestAttempt.Status == "running") {
		return
	}
	if filepath.Base(detail.ID) != detail.ID || detail.ID == "." || detail.ID == ".." {
		return
	}
	type copyPath struct{ base, path string }
	paths := []copyPath{{s.cfg.WorkspacesDir, filepath.Join(s.cfg.WorkspacesDir, detail.ID)}}
	attempts, err := s.taskSvc.Store().ListAttempts(ctx, detail.ID)
	if err != nil {
		return
	}
	for _, attempt := range attempts {
		if attempt == nil {
			continue
		}
		if proof, err := task.VerificationIntentForAttempt(ctx, s.taskSvc.Store(), &detail.Task, attempt.ID); err == nil && proof != nil && proof.IsolatedWorkspace {
			paths = append(paths, copyPath{s.cfg.WorkspacesDir, git.VerificationWorkspacePath(s.cfg.WorkspacesDir, detail.ID, proof.SourceCommit)})
		}
		if delivery, err := task.NativeReleaseDeliveryForAttempt(ctx, s.taskSvc.Store(), &detail.Task, attempt.ID); err == nil && delivery != nil {
			path, err := s.projectSvc.NativeReleaseWorkspacePath(project.NativeReleaseWorkspaceRequest{ProjectID: detail.ProjectID, TaskID: detail.ID, SourceCommit: delivery.SourceCommit, TargetCommit: delivery.TargetCommit, TargetBranch: delivery.TargetBranch, ManifestHash: delivery.ManifestHash})
			if err == nil {
				paths = append(paths, copyPath{filepath.Dir(path), path})
			}
		}
		if proof, err := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &detail.Task, attempt.ID); err == nil && proof != nil {
			path, err := s.projectSvc.VerificationSnapshotPath(detail.ProjectID, detail.ID, proof.SnapshotSourceCommit(), proof.TargetCommit)
			if err == nil {
				paths = append(paths, copyPath{filepath.Dir(path), path})
			}
		}
	}
	client := git.NewClient()
	seen := map[string]bool{}
	for _, copy := range paths {
		if ctx.Err() != nil {
			return
		}
		if seen[copy.path] {
			continue
		}
		seen[copy.path] = true
		bytes, err := client.DiscardResolvedRepoCopy(ctx, copy.base, copy.path)
		if err != nil {
			log.Printf("[retention] resolved task repo copy retained: task=%s", detail.ID)
			continue
		}
		if bytes > 0 {
			log.Printf("[retention] resolved task repo copy removed: task=%s logical_bytes=%d", detail.ID, bytes)
		}
	}
}
