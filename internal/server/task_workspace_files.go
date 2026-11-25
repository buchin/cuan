package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	projectpkg "github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/task"
)

// resolveTaskExecutionWorkspace resolves the actual active execution workspace for a task,
// checking active terminal sessions first, then project workspaces, and falling back to default task workspaces.
func (s *Server) resolveTaskExecutionWorkspace(ctx context.Context, rawTaskID string) (string, string, error) {
	taskID := strings.TrimSpace(rawTaskID)
	if taskID == "" {
		return "", "", fmt.Errorf("task id is required")
	}

	canonicalID := s.resolveTaskID(ctx, taskID)
	if canonicalID != "" {
		taskID = canonicalID
	}

	// 0. Project terminal sessions ("project-term-<id>" or "project:<id>") run inside ProjectTerminalWorkspace
	var termProjID string
	if strings.HasPrefix(taskID, "project-term-") {
		termProjID = strings.TrimSpace(strings.TrimPrefix(taskID, "project-term-"))
	} else if strings.HasPrefix(taskID, "project:") {
		termProjID = strings.TrimSpace(strings.TrimPrefix(taskID, "project:"))
	}
	if termProjID != "" && s.projectSvc != nil {
		if proj, err := s.projectSvc.Get(ctx, termProjID); err == nil && proj != nil {
			workDir, wErr := s.projectSvc.ProjectTerminalWorkspace(ctx, proj.ID, proj)
			if wErr != nil || workDir == "" {
				workDir = proj.LocalPath
			}
			if workDir != "" {
				_ = os.MkdirAll(workDir, 0755)
				return taskID, workDir, nil
			}
		}
	}

	// 1. Check if there is an active running terminal session for this task
	if s.terminalManager != nil {
		if sess, ok := s.terminalManager.GetSession(taskID); ok && sess != nil && sess.PTY() != nil {
			if dir := sess.PTY().WorkDir(); dir != "" {
				if info, err := os.Stat(dir); err == nil && info.IsDir() {
					return taskID, dir, nil
				}
			}
		}
	}

	// 2. Check if task belongs to a project with its own execution workspace
	if s.taskSvc != nil && s.projectSvc != nil {
		detail, err := s.taskSvc.GetTaskDetail(ctx, taskID)
		if err == nil && detail != nil && detail.ProjectID != "" {
			project, pErr := s.projectSvc.TaskContext(ctx, detail.ProjectID, detail.Instructions)
			if pErr == nil && project != nil && (project.RepoURL != "" || project.LocalPath != "") {
				workDir, e := s.projectSvc.TaskExecutionWorkspace(ctx, taskID, detail.Instructions, project)
				if errors.Is(e, projectpkg.ErrExecutionWorkspaceMissing) {
					workDir, e = s.projectSvc.ReconstructTaskExecutionWorkspace(ctx, taskID, project)
				}
				if e == nil && workDir != "" {
					_ = os.MkdirAll(workDir, 0755)
					return taskID, workDir, nil
				}
			}
		}
	}

	// 3. Fallback to terminal manager's canonical workspace directory
	if s.terminalManager != nil {
		workDir, err := s.terminalManager.ResolveTaskWorkspace(taskID)
		if err != nil {
			return taskID, "", err
		}
		_ = os.MkdirAll(workDir, 0755)
		return taskID, workDir, nil
	}

	return taskID, "", fmt.Errorf("terminal manager is not configured")
}

// handleUploadTaskFile handles uploading files directly into the canonical workspace of a task.
// POST /api/v1/tasks/{id}/files
func (s *Server) handleUploadTaskFile(w http.ResponseWriter, r *http.Request) {
	rawTaskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if rawTaskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	taskID, workDir, err := s.resolveTaskExecutionWorkspace(r.Context(), rawTaskID)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid task workspace: %v", err))
		return
	}

	if err := os.MkdirAll(workDir, 0755); err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to access task workspace")
		return
	}

	// Max 32MB file upload
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		respondJSONError(w, http.StatusBadRequest, "failed to parse multipart upload: "+err.Error())
		return
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		files = r.MultipartForm.File["files"]
	}
	if len(files) == 0 {
		respondJSONError(w, http.StatusBadRequest, "no file provided in form field 'file'")
		return
	}

	type UploadedFileResult struct {
		Filename string `json:"filename"`
		Path     string `json:"path"`
		Size     int64  `json:"size"`
	}
	uploaded := make([]UploadedFileResult, 0, len(files))

	for _, header := range files {
		cleanName := filepath.Base(filepath.Clean(header.Filename))
		if cleanName == "" || cleanName == "." || cleanName == ".." || strings.Contains(cleanName, "/") || strings.Contains(cleanName, "\\") {
			continue
		}

		src, err := header.Open()
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, "failed to read uploaded file: "+err.Error())
			return
		}

		destPath := filepath.Join(workDir, cleanName)
		dst, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			src.Close()
			respondJSONError(w, http.StatusInternalServerError, "failed to create destination file in workspace: "+err.Error())
			return
		}

		written, err := io.Copy(dst, src)
		src.Close()
		dst.Close()
		if err != nil {
			respondJSONError(w, http.StatusInternalServerError, "failed to write file content: "+err.Error())
			return
		}

		uploaded = append(uploaded, UploadedFileResult{
			Filename: cleanName,
			Path:     "./" + cleanName,
			Size:     written,
		})
	}

	if len(uploaded) == 0 {
		respondJSONError(w, http.StatusBadRequest, "no valid files were uploaded")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"task_id":  taskID,
		"files":    uploaded,
		"filename": uploaded[0].Filename,
		"path":     uploaded[0].Path,
		"size":     uploaded[0].Size,
		"message":  fmt.Sprintf("Uploaded %d file(s) to workspace", len(uploaded)),
	})
}

// handleGetTaskWorkspaceFile serves a file from the task's execution workspace for inline previewing or downloading.
// GET /api/v1/tasks/{id}/files/raw?path=relative/file/path
func (s *Server) handleGetTaskWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	rawTaskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if rawTaskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	_, workDir, err := s.resolveTaskExecutionWorkspace(r.Context(), rawTaskID)
	if err != nil || workDir == "" {
		respondJSONError(w, http.StatusNotFound, "task workspace not found")
		return
	}

	rawPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if rawPath == "" {
		respondJSONError(w, http.StatusBadRequest, "path query parameter is required")
		return
	}

	// Normalize and clean path to prevent directory traversal
	cleanRel := filepath.Clean(rawPath)
	cleanRel = strings.TrimPrefix(cleanRel, "/")
	cleanRel = strings.TrimPrefix(cleanRel, "./")
	if cleanRel == "" || cleanRel == "." || strings.HasPrefix(cleanRel, "..") {
		respondJSONError(w, http.StatusForbidden, "invalid file path")
		return
	}

	fullPath := filepath.Join(workDir, cleanRel)
	rel, err := filepath.Rel(workDir, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		respondJSONError(w, http.StatusForbidden, "file path outside workspace")
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			respondJSONError(w, http.StatusNotFound, "file not found in task workspace")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to read file: "+err.Error())
		return
	}
	if info.IsDir() {
		respondJSONError(w, http.StatusBadRequest, "requested path is a directory")
		return
	}

	// Set inline preview headers
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(cleanRel)))
	} else {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, filepath.Base(cleanRel)))
	}

	http.ServeFile(w, r, fullPath)
}

// handleGetTaskUsage returns real-time execution statistics for a task session.
// GET /api/v1/tasks/{id}/usage
func (s *Server) handleGetTaskUsage(w http.ResponseWriter, r *http.Request) {
	rawTaskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if rawTaskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	taskID, workDir, _ := s.resolveTaskExecutionWorkspace(r.Context(), rawTaskID)

	var detail *task.TaskDetailResponse
	var err error
	if s.taskSvc != nil {
		detail, err = s.taskSvc.GetTaskDetail(r.Context(), taskID)
	}
	if err != nil || detail == nil {
		respondJSONError(w, http.StatusNotFound, "task not found")
		return
	}

	var durationSec float64
	var activeAttempt string
	var runnerID string
	var agentName string
	var status string = detail.Status

	if len(detail.Attempts) > 0 {
		latest := detail.Attempts[len(detail.Attempts)-1]
		activeAttempt = latest.ID
		runnerID = latest.RunnerID
		agentName = latest.AgentName
		if latest.FinishedAt != nil {
			durationSec = latest.FinishedAt.Sub(latest.CreatedAt).Seconds()
		} else {
			durationSec = time.Since(latest.CreatedAt).Seconds()
		}
	}

	var branch string
	var fileCount int
	if workDir != "" {
		_ = filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				fileCount++
			}
			return nil
		})

		if _, statErr := os.Stat(filepath.Join(workDir, ".git")); statErr == nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			cmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
			cmd.Dir = workDir
			if out, err := cmd.Output(); err == nil {
				branch = strings.TrimSpace(string(out))
			}
			cancel()
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"ok":               true,
		"task_id":          taskID,
		"status":           status,
		"duration_seconds": int(durationSec),
		"active_attempt":   activeAttempt,
		"runner_id":        runnerID,
		"agent_name":       agentName,
		"workspace": map[string]interface{}{
			"branch":      branch,
			"files_count": fileCount,
		},
	})
}

// handleGetTaskDiff returns git diff / status of the task workspace.
// GET /api/v1/tasks/{id}/diff
func (s *Server) handleGetTaskDiff(w http.ResponseWriter, r *http.Request) {
	rawTaskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if rawTaskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	_, workDir, err := s.resolveTaskExecutionWorkspace(r.Context(), rawTaskID)
	if err != nil || workDir == "" {
		respondJSONError(w, http.StatusBadRequest, "invalid workspace")
		return
	}

	if _, err := os.Stat(filepath.Join(workDir, ".git")); err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"ok":       true,
			"has_diff": false,
			"diff":     "No git repository initialized in this workspace.",
			"status":   "",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	cmdDiff := exec.CommandContext(ctx, "git", "diff", "--stat")
	cmdDiff.Dir = workDir
	diffOut, _ := cmdDiff.Output()

	cmdStatus := exec.CommandContext(ctx, "git", "status", "--short")
	cmdStatus.Dir = workDir
	statusOut, _ := cmdStatus.Output()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"has_diff": len(diffOut) > 0 || len(statusOut) > 0,
		"stat":     strings.TrimSpace(string(diffOut)),
		"status":   strings.TrimSpace(string(statusOut)),
	})
}
