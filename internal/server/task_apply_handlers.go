package server

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
	"github.com/go-chi/chi/v5"
)

// handleApplyTask merges the verified task branch / GitHub PR to the default branch (main)
// and updates the task status to done, providing a non-developer friendly 1-click experience.
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

	if taskDetail.PRNumber <= 0 && taskDetail.BranchName == "" {
		respondJSONError(w, http.StatusBadRequest, "This task has no pull request or task branch to apply")
		return
	}

	var mergeResult *git.MergePullRequestResult
	if taskDetail.PRNumber > 0 {
		commitTitle := fmt.Sprintf("Apply Task %s: %s", taskDetail.ID, taskDetail.Title)
		commitMessage := fmt.Sprintf("Applied via Cuan One-Click Apply to Website.\n\nTask: %s\nSummary: %s", taskDetail.ID, taskDetail.Summary)

		res, err := s.projectSvc.ApplyTaskPullRequest(r.Context(), taskDetail.ProjectID, taskDetail.PRNumber, commitTitle, commitMessage)
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Gagal menerapkan perubahan ke website: %v", err))
			return
		}
		mergeResult = res
	}

	// Update task status to "done"
	doneStatus := "done"
	note := taskDetail.CompleteNote
	if note != "" {
		note += "\n\n"
	}
	appliedSHA := ""
	if mergeResult != nil {
		appliedSHA = mergeResult.SHA
	}
	if appliedSHA != "" {
		note += fmt.Sprintf("🚀 Perubahan telah berhasil diterapkan ke website (Commit %s).", appliedSHA)
	} else {
		note += "🚀 Perubahan telah berhasil diterapkan ke website."
	}

	updated, err := s.taskSvc.UpdateTask(r.Context(), taskID, "owner", task.UpdateTaskRequest{
		Status:       &doneStatus,
		CompleteNote: &note,
	})
	if err != nil {
		log.Printf("[Server] Task %s merged but status update note error: %v", taskID, err)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"applied": true,
		"sha":     appliedSHA,
		"message": "Perubahan berhasil diterapkan ke website!",
		"task":    updated,
	})
}
