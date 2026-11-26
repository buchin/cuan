package server

import (
	"context"
	"errors"
	"fmt"
	"github.com/buchin/cuan/internal/executionpolicy"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
	"github.com/go-chi/chi/v5"
)

// handleApplyTask merges the verified task branch / GitHub PR to the default branch (main)
// and updates the task status to done, providing a non-developer friendly 1-click experience.
// If the project has auto_deploy enabled, it automatically triggers live deployment.
// POST /api/tasks/{id}/apply or /api/v1/tasks/{id}/apply
func (s *Server) handleApplyTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	if taskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	taskDetail, err := s.taskSvc.GetTaskDetail(r.Context(), taskID)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "Task not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get task: %v", err))
		return
	}

	policy, err := s.taskPublicationPolicy(r.Context(), taskDetail)
	if err != nil {
		respondJSONError(w, http.StatusServiceUnavailable, "Task publication limits could not be verified; no changes were applied")
		return
	}
	if err := taskApplyPermission(policy); err != nil {
		respondJSONError(w, http.StatusForbidden, err.Error())
		return
	}
	if taskDetail.PRNumber <= 0 && taskDetail.BranchName == "" {
		respondJSONError(w, http.StatusBadRequest, "This task has no pull request or task branch to apply")
		return
	}

	mergeResult, err := s.applyTaskInternal(r.Context(), taskDetail.ID, taskDetail.ProjectID, taskDetail.Title, taskDetail.Summary, taskDetail.BranchName, taskDetail.PRNumber)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Gagal menerapkan perubahan: %v", err))
		return
	}

	// Check if auto_deploy is enabled for this project
	autoDeployTriggered := false
	if proj, err := s.projectSvc.Get(r.Context(), taskDetail.ProjectID); err == nil && proj != nil {
		settings := project.SettingsFromMetadata(proj.Metadata)
		if settings.IsAutoDeployEnabled() {
			autoDeployTriggered = true
			go func() {
				s.triggerAutoDeploy(context.Background(), taskDetail.ProjectID, taskID)
			}()
		}
	}

	// Merge completion is not production acceptance. Keep auto-deploy work
	// in review until its deployment has independently been verified.
	doneStatus := "done"
	if autoDeployTriggered {
		doneStatus = task.TaskStatusReview
	}
	appliedSHA := ""
	if mergeResult != nil {
		appliedSHA = mergeResult.SHA
	}
	note := taskDetail.CompleteNote
	if !strings.Contains(note, "Perubahan telah berhasil diterapkan") {
		if note != "" {
			note += "\n\n"
		}
		if appliedSHA != "" {
			note += fmt.Sprintf("🚀 Perubahan telah berhasil diterapkan (Commit %s).", appliedSHA)
		} else {
			note += "🚀 Perubahan telah berhasil diterapkan."
		}
		if autoDeployTriggered {
			note += " Verifikasi deployment production belum dikonfirmasi."
		}
	}

	updateReq := task.UpdateTaskRequest{
		Status:       &doneStatus,
		CompleteNote: &note,
	}
	if appliedSHA != "" {
		updateReq.CommitSHA = &appliedSHA
	}
	updated, err := s.taskSvc.UpdateTask(r.Context(), taskID, "owner", updateReq)
	if err != nil {
		log.Printf("[Server] Task %s merged but status update note error: %v", taskID, err)
	} else if updated != nil && updated.Status == task.TaskStatusDone {
		s.DispatchTaskCompletedPush(r.Context(), updated)
		if updated.SourceRoomID != "" && s.projectStudioRepo != nil {
			s.publishInbox(updated.SourceRoomID)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"applied":            true,
		"sha":                appliedSHA,
		"auto_deploy":        autoDeployTriggered,
		"deployment_pending": autoDeployTriggered,
		"message":            "Perubahan berhasil diterapkan!",
		"task":               updated,
	})
}

// applyTaskInternal merges the task pull request or task branch into the project default branch.
func (s *Server) applyTaskInternal(ctx context.Context, taskID, projectID, title, summary, branchName string, prNumber int) (*git.MergePullRequestResult, error) {
	// Re-read the persisted scope at the publication boundary. Callers may hold
	// a stale detail from before an owner narrowed the approved work.
	detail, err := s.taskSvc.GetTaskDetail(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if detail.ProjectID != projectID {
		return nil, errors.New("task project changed; reload before applying")
	}
	policy, err := s.taskPublicationPolicy(ctx, detail)
	if err != nil {
		return nil, err
	}
	if err := taskApplyPermission(policy); err != nil {
		return nil, err
	}
	commitTitle := fmt.Sprintf("Apply Task %s: %s", taskID, title)
	commitMessage := fmt.Sprintf("Applied via Cuan One-Click Apply.\n\nTask: %s\nSummary: %s", taskID, summary)

	var prErr error
	if prNumber > 0 {
		res, err := s.projectSvc.ApplyTaskPullRequest(ctx, projectID, prNumber, commitTitle, commitMessage, detail.CommitSHA)
		if err == nil {
			return res, nil
		}
		prErr = err
		log.Printf("[applyTaskInternal] ApplyTaskPullRequest for task %s (PR #%d) failed: %v; attempting ApplyTaskBranch fallback", taskID, prNumber, err)
	}
	if branchName != "" {
		if s.cfg != nil && s.cfg.WorkspacesDir != "" && s.projectSvc != nil {
			sourcePath := filepath.Join(s.cfg.WorkspacesDir, taskID)
			if info, err := os.Stat(sourcePath); err == nil && info.IsDir() {
				if _, _, pushErr := s.projectSvc.PushNativeTaskBranch(ctx, taskID, projectID, detail.Instructions, sourcePath, branchName, false, title); pushErr != nil {
					log.Printf("[applyTaskInternal] note: PushNativeTaskBranch for %s: %v", taskID, pushErr)
				}
				if proj, err := s.projectSvc.Get(ctx, projectID); err == nil && proj != nil && proj.LocalPath != "" {
					cmd := exec.CommandContext(ctx, "git", "-C", proj.LocalPath, "fetch", sourcePath, fmt.Sprintf("%s:%s", branchName, branchName))
					if out, fetchErr := cmd.CombinedOutput(); fetchErr != nil {
						log.Printf("[applyTaskInternal] note: local workspace branch fetch for %s: %v (%s)", taskID, fetchErr, strings.TrimSpace(string(out)))
					}
				}
			}
		}
		res, err := s.projectSvc.ApplyTaskBranch(ctx, projectID, branchName, commitTitle, commitMessage, detail.CommitSHA)
		if err == nil {
			return res, nil
		}
		if prErr != nil {
			return nil, fmt.Errorf("pull request apply failed (%v) and task branch apply failed: %w", prErr, err)
		}
		return nil, err
	}
	if prErr != nil {
		return nil, prErr
	}
	return nil, errors.New("neither pull request nor task branch is available")
}

// The exact latest attempt supplies continuation restrictions. An unrelated
// newer queued launch must neither remove nor add limits to the result applied.
func (s *Server) taskPublicationPolicy(ctx context.Context, detail *task.TaskDetailResponse) (executionpolicy.PublicationPolicy, error) {
	// Cancellation is authoritative even when an old report/merge callback is
	// still pending. Recheck at each publication boundary, not just in the UI.
	if detail == nil || detail.Status == "cancelled" || detail.Status == "canceled" {
		return executionpolicy.PublicationPolicy{}, errors.New("Task was cancelled; no changes may be published")
	}
	if a := detail.LatestAttempt; a != nil && (a.CancelRequested || a.CanceledAt != nil || a.Status == task.AttemptStatusCanceled) {
		return executionpolicy.PublicationPolicy{}, errors.New("Task execution was cancelled; no changes may be published")
	}
	if detail.LatestAttempt == nil {
		return executionpolicy.FromInstructions(detail.Instructions), nil
	}
	attemptID := detail.LatestAttempt.ID
	if s.taskSvc != nil && s.taskSvc.Store() != nil {
		if proof, err := task.ValidateVerificationSource(ctx, s.taskSvc.Store(), &detail.Task, attemptID); err == nil && proof != nil && proof.SourceAttemptID != "" {
			attemptID = proof.SourceAttemptID
		}
	}
	return s.taskSvc.AttemptPublicationPolicy(ctx, detail.ID, attemptID)
}

func taskApplyPermission(policy executionpolicy.PublicationPolicy) error {
	if policy.DenyPush {
		return errors.New("Scope task melarang publikasi perubahan ke repository; perubahan belum diterapkan")
	}
	if policy.DenyDeploy {
		return errors.New("Scope task melarang deployment dan apply/merge; perubahan belum diterapkan")
	}
	return nil
}

// triggerAutoDeploy initiates automated live deployment when project auto_deploy is enabled.
func (s *Server) triggerAutoDeploy(ctx context.Context, projectID, taskID string) {
	// A project default never overrides the approved task's publication limits.
	detail, err := s.taskSvc.GetTaskDetail(ctx, taskID)
	if err != nil || detail.ProjectID != projectID {
		return
	}
	policy, err := s.taskPublicationPolicy(ctx, detail)
	if err != nil || policy.DenyDeploy {
		return
	}
	log.Printf("[AutoDeploy] Automated live deployment triggered for project %s (task %s)", projectID, taskID)

	if s.scriptDeployRepo != nil && s.deployerDriver != nil {
		dep, err := s.scriptDeployRepo.GetDeploymentByProjectID(ctx, projectID)
		if err == nil && dep != nil {
			log.Printf("[AutoDeploy] Found registered deployment %s for project %s, executing restart...", dep.ID, projectID)
			if err := s.deployerDriver.Restart(ctx, dep.ID); err != nil {
				log.Printf("[AutoDeploy] Restart failed for %s: %v, attempting Start...", dep.ID, err)
				_ = s.deployerDriver.Start(ctx, dep.ID)
			} else {
				log.Printf("[AutoDeploy] Deployment %s restarted successfully", dep.ID)
			}
			return
		}
	}

	if s.projectSvc != nil {
		proj, err := s.projectSvc.Get(ctx, projectID)
		if err == nil && proj != nil {
			settings := project.SettingsFromMetadata(proj.Metadata)
			job := DeployJob{
				ProjectID:    projectID,
				TaskID:       taskID,
				LocalPath:    proj.LocalPath,
				DeployScript: settings.DeployScript,
				Environment:  settings.Environment,
				CommitSHA:    detail.CommitSHA,
			}
			if s.deployCoordinator == nil {
				s.deployCoordinator = NewDeployCoordinator()
			}
			s.deployCoordinator.Enqueue(ctx, job)
			if isCuanProject(proj) {
				log.Printf("[AutoDeploy] Native Cuan project %s auto-deploy acknowledged", projectID)
			}
		}
	}
}

// autoApplyAndDeployTask is called when an agent completes a task on a project with auto_deploy enabled.
// Returns true if auto-apply/deploy was attempted, or false if skipped by policy/preconditions.
func (s *Server) autoApplyAndDeployTask(ctx context.Context, taskID, projectID string) bool {
	taskDetail, err := s.taskSvc.GetTaskDetail(ctx, taskID)
	if err != nil || taskDetail == nil {
		return false
	}
	if taskDetail.LatestAttempt == nil || taskDetail.LatestAttempt.Status != task.AttemptStatusSucceeded {
		log.Printf("[AutoDeploy] Skipping auto-apply and deploy for task %s: latest attempt not succeeded", taskID)
		return false // Never publish from an unverified agent report or failed attempt.
	}
	policy, err := s.taskPublicationPolicy(ctx, taskDetail)
	if err != nil {
		log.Printf("[AutoDeploy] Skipping auto-apply and deploy for task %s: policy error: %v", taskID, err)
		return false
	}
	if permErr := taskApplyPermission(policy); permErr != nil {
		log.Printf("[AutoDeploy] Skipping auto-apply and deploy for task %s: %v", taskID, permErr)
		return false
	}
	if taskDetail.ExecutionCategory != "" && taskDetail.ExecutionCategory != task.ExecutionCategoryCode {
		log.Printf("[AutoDeploy] Skipping auto-apply and deploy for non-code task %s (category: %s)", taskID, taskDetail.ExecutionCategory)
		return false
	}
	if taskDetail.ExecutionCategory != task.ExecutionCategoryCode && isInvestigationOrNonCodeText(taskDetail.Title) {
		log.Printf("[AutoDeploy] Skipping auto-apply and deploy for non-code investigation task %s", taskID)
		return false
	}
	if taskDetail.PRNumber <= 0 && taskDetail.BranchName == "" {
		log.Printf("[AutoDeploy] Skipping auto-apply and deploy for task %s without branch or PR", taskID)
		return false
	}
	if !releaseObjectID.MatchString(taskDetail.CommitSHA) {
		log.Printf("[AutoDeploy] Skipping auto-apply and deploy for task %s without valid commit SHA", taskID)
		return false
	}
	branch := taskDetail.BranchName
	mergeRes, err := s.applyTaskInternal(ctx, taskDetail.ID, taskDetail.ProjectID, taskDetail.Title, taskDetail.Summary, branch, taskDetail.PRNumber)
	if err != nil {
		log.Printf("[AutoDeploy] Auto-apply failed for task %s: %v", taskID, err)
		const autoApplyFailMarker = "⚠️ Auto-apply tidak dapat menggabungkan perubahan secara otomatis"
		if !strings.Contains(taskDetail.CompleteNote, autoApplyFailMarker) || taskDetail.Status != task.TaskStatusReview {
			reviewStatus := task.TaskStatusReview
			req := task.UpdateTaskRequest{Status: &reviewStatus}
			if !strings.Contains(taskDetail.CompleteNote, autoApplyFailMarker) {
				failNote := taskDetail.CompleteNote
				if failNote != "" {
					failNote += "\n\n"
				}
				failNote += fmt.Sprintf("%s (%v). Hasil pekerjaan tetap tersimpan untuk ditinjau.", autoApplyFailMarker, err)
				req.CompleteNote = &failNote
			}
			_, _ = s.taskSvc.UpdateTask(ctx, taskID, "system", req)
		}
		return false
	}
	appliedSHA := ""
	if mergeRes != nil {
		appliedSHA = mergeRes.SHA
	}
	note := taskDetail.CompleteNote
	if !strings.Contains(note, "Perubahan telah berhasil diterapkan") {
		if note != "" {
			note += "\n\n"
		}
		if appliedSHA != "" {
			note += fmt.Sprintf("🚀 Perubahan telah berhasil diterapkan (Commit %s).", appliedSHA)
		} else {
			note += "🚀 Perubahan telah berhasil diterapkan."
		}
		note += " Verifikasi deployment production belum dikonfirmasi."
	}
	doneStatus := task.TaskStatusReview
	autoUpdateReq := task.UpdateTaskRequest{
		Status:       &doneStatus,
		CompleteNote: &note,
	}
	if appliedSHA != "" {
		autoUpdateReq.CommitSHA = &appliedSHA
	}
	_, _ = s.taskSvc.UpdateTask(ctx, taskID, "system", autoUpdateReq)
	s.triggerAutoDeploy(ctx, projectID, taskID)
	return true
}

func isInvestigationOrNonCodeText(texts ...string) bool {
	combined := strings.ToLower(strings.Join(texts, " "))
	for _, term := range []string{
		"investigasi", "investigation", "root cause",
		"analisis", "analysis", "klarifikasi", "audit", "pemadaman", "outage",
	} {
		if strings.Contains(combined, term) {
			return true
		}
	}
	return false
}
