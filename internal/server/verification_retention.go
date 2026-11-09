package server

import (
	"context"
	"log"
	"path/filepath"
	"time"

	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
)

// The existing maintenance job reconciles retained native acceptance, including
// old Done tasks. This never accepts work or changes execution/task history.
func (s *Server) markAcceptedVerificationSnapshots(ctx context.Context) {
	if s.taskSvc == nil || s.projectSvc == nil || s.cfg == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	rows, err := s.taskSvc.Store().ListTasks(ctx, task.TaskFilter{Status: task.TaskStatusDone})
	if err != nil {
		log.Printf("[storage] accepted snapshot scan unavailable: %v", err)
		return
	}
	client := git.NewClient()
	for _, row := range rows {
		if ctx.Err() != nil {
			return
		}
		d, err := s.taskSvc.GetTaskDetail(ctx, row.ID)
		if err != nil || d == nil || d.Status != task.TaskStatusDone || d.LatestAttempt == nil || d.LatestAttempt.Status != task.AttemptStatusSucceeded || d.LatestAttempt.CancelRequested || d.LatestAttempt.CanceledAt != nil {
			continue
		}
		var path, source, target string
		if proof, e := task.LiveInspectionForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID); e == nil && proof != nil {
			source, target = proof.SnapshotSourceCommit(), proof.TargetCommit
			path, err = s.projectSvc.VerificationSnapshotPath(d.ProjectID, d.ID, source, target)
		} else if proof, e := task.VerificationIntentForAttempt(ctx, s.taskSvc.Store(), &d.Task, d.LatestAttempt.ID); e == nil && proof != nil && proof.IsolatedWorkspace && s.cfg.WorkspacesDir != "" {
			source, target = proof.SourceCommit, proof.SourceCommit
			path = git.VerificationWorkspacePath(s.cfg.WorkspacesDir, d.ID, source)
		}
		if err != nil || path == "" {
			continue
		}
		if path, err = filepath.Abs(path); err != nil {
			continue
		}
		if err = client.MarkVerificationSnapshotAccepted(ctx, path, d.ID, source, target, d.UpdatedAt); err != nil {
			log.Printf("[storage] retained verifier snapshot for task %s: %v", d.ID, err)
		}
	}
}
