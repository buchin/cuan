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

	updated, err := s.taskSvc.UpdateTask(r.Context(), taskID, "owner", task.UpdateTaskRequest{
		Status:       &doneStatus,
		CompleteNote: &note,
	})
	if err != nil {
		log.Printf("[Server] Task %s merged but status update note error: %v", taskID, err)
	} else if updated != nil && updated.Status == task.TaskStatusDone {
		s.DispatchTaskCompletedPush(r.Context(), updated)
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

	if prNumber > 0 {
		return s.projectSvc.ApplyTaskPullRequest(ctx, projectID, prNumber, commitTitle, commitMessage, detail.CommitSHA)
	}
	if branchName != "" {
		return s.projectSvc.ApplyTaskBranch(ctx, projectID, branchName, commitTitle, commitMessage, detail.CommitSHA)
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
			if strings.TrimSpace(settings.DeployScript) != "" && proj.LocalPath != "" {
				log.Printf("[AutoDeploy] Executing project deploy script for %s in %s", projectID, proj.LocalPath)
				cmd := exec.CommandContext(ctx, "bash", "-c", settings.DeployScript)
				cmd.Dir = proj.LocalPath
				baseEnv := append(os.Environ(),
					"CI=true",
					"npm_config_yes=true",
					"DEBIAN_FRONTEND=noninteractive",
				)
				if settings.Environment != "" {
					envMap := project.ParseEnvironment(settings.Environment)
					var envSlice []string
					for k, v := range envMap {
						envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
					}
					cmd.Env = append(baseEnv, envSlice...)
				} else {
					cmd.Env = baseEnv
				}
				if out, err := cmd.CombinedOutput(); err != nil {
					log.Printf("[AutoDeploy] Project deploy script failed for %s: %v: %s", projectID, err, string(out))
				} else {
					log.Printf("[AutoDeploy] Project deploy script succeeded for %s", projectID)
				}
			} else if (proj.RepoName == "cuan" || strings.Contains(strings.ToLower(proj.Name), "cuan")) && proj.LocalPath != "" {
				deployScript := filepath.Join(proj.LocalPath, "scripts", "deploy.sh")
				if info, err := os.Stat(deployScript); err == nil && !info.IsDir() {
					log.Printf("[AutoDeploy] Executing native Cuan auto-deploy in %s", proj.LocalPath)
					cmd := exec.CommandContext(ctx, "bash", deployScript, "--local", "--native-sync")
					cmd.Dir = proj.LocalPath
					if out, err := cmd.CombinedOutput(); err != nil {
						log.Printf("[AutoDeploy] Native Cuan deploy failed for %s: %v: %s", projectID, err, string(out))
					} else {
						log.Printf("[AutoDeploy] Native Cuan deploy succeeded for %s", projectID)
					}
				}
			}
			if proj.RepoName == "cuan" || strings.Contains(strings.ToLower(proj.Name), "cuan") {
				log.Printf("[AutoDeploy] Native Cuan project %s auto-deploy acknowledged", projectID)
			}
		}
	}
}

// autoApplyAndDeployTask is called when an agent completes a task on a project with auto_deploy enabled.
func (s *Server) autoApplyAndDeployTask(ctx context.Context, taskID, projectID string) {
	taskDetail, err := s.taskSvc.GetTaskDetail(ctx, taskID)
	if err != nil || taskDetail == nil {
		return
	}
	if taskDetail.LatestAttempt == nil || taskDetail.LatestAttempt.Status != task.AttemptStatusSucceeded {
		return // Never publish from an unverified agent report or failed attempt.
	}
	policy, err := s.taskPublicationPolicy(ctx, taskDetail)
	if err != nil || taskApplyPermission(policy) != nil {
		return
	}
	branch := taskDetail.BranchName
	if branch == "" && taskDetail.PRNumber <= 0 {
		branch = fmt.Sprintf("task/%s", taskDetail.ID)
	}
	if taskDetail.PRNumber <= 0 && branch == "" {
		return
	}
	if taskDetail.ExecutionCategory == task.ExecutionCategoryReadOnly {
		log.Printf("[AutoDeploy] Skipping auto-apply and deploy for read-only task %s", taskID)
		return
	}
	if taskDetail.ExecutionCategory != task.ExecutionCategoryCode && isInvestigationOrNonCodeText(taskDetail.Title) {
		log.Printf("[AutoDeploy] Skipping auto-apply and deploy for non-code investigation task %s", taskID)
		return
	}
	mergeRes, err := s.applyTaskInternal(ctx, taskDetail.ID, taskDetail.ProjectID, taskDetail.Title, taskDetail.Summary, branch, taskDetail.PRNumber)
	if err != nil {
		log.Printf("[AutoDeploy] Auto-apply failed for task %s: %v", taskID, err)
		return
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
	_, _ = s.taskSvc.UpdateTask(ctx, taskID, "system", task.UpdateTaskRequest{
		Status:       &doneStatus,
		CompleteNote: &note,
	})
	s.triggerAutoDeploy(ctx, projectID, taskID)
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
