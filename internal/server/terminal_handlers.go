package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
)

// handleTaskTerminalWebSocket handles bidirectional WebSocket PTY streaming for tasks.
// Supports both Watch Mode (?mode=watch) and Interactive Take-Over Mode (?mode=interactive).
// GET /api/tasks/{id}/terminal or GET /api/v1/tasks/{id}/terminal
func (s *Server) handleTaskTerminalWebSocket(w http.ResponseWriter, r *http.Request) {
	if s.terminalHandler == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "terminal engine is not configured")
		return
	}

	if err := s.terminalHandler.ValidateRequestAuth(r); err != nil {
		respondJSONError(w, 401, "Masuk ke Cuan untuk membuka terminal")
		return
	}
	taskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if taskID == "" {
		taskID = strings.TrimSpace(r.URL.Query().Get("id"))
	}
	if taskID == "" {
		taskID = strings.TrimSpace(r.URL.Query().Get("task_id"))
	}

	if taskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	found := false
	if s.projectStudioRepo != nil {
		if _, e := s.projectStudioRepo.GetTaskByID(r.Context(), taskID); e == nil {
			found = true
		}
	}
	if !found && s.taskSvc != nil {
		if _, e := s.taskSvc.GetTaskDetail(r.Context(), taskID); e == nil {
			found = true
		}
	}
	if !found {
		respondJSONError(w, 404, "Task tidak ditemukan; terminal tidak dibuat.")
		return
	}

	s.terminalHandler.ServeHTTP(w, r)
}

// handleGetTerminalSessionsStatus returns status of running terminal sessions.
// GET /api/terminal/sessions or GET /api/v1/terminal/sessions
func (s *Server) handleGetTerminalSessionsStatus(w http.ResponseWriter, r *http.Request) {
	if s.terminalManager == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "terminal manager is not configured")
		return
	}

	active := s.terminalManager.ActiveSessions()
	type SessionSummary struct {
		ID           string `json:"id"`
		TaskID       string `json:"task_id"`
		Subscribers  int    `json:"subscribers"`
		CreatedAt    string `json:"created_at"`
		LastActivity string `json:"last_activity"`
		Cols         uint16 `json:"cols"`
		Rows         uint16 `json:"rows"`
		PID          int    `json:"pid"`
		WorkDir      string `json:"work_dir"`
	}

	summaries := make([]SessionSummary, len(active))
	for i, sess := range active {
		cols, rows := sess.PTY().GetSize()
		summaries[i] = SessionSummary{
			ID:           sess.ID(),
			TaskID:       sess.TaskID(),
			Subscribers:  sess.SubscriberCount(),
			CreatedAt:    sess.CreatedAt().Format("2006-01-02T15:04:05Z07:00"),
			LastActivity: sess.LastActivity().Format("2006-01-02T15:04:05Z07:00"),
			Cols:         cols,
			Rows:         rows,
			PID:          sess.PTY().PID(),
			WorkDir:      sess.PTY().WorkDir(),
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sessions": summaries,
		"count":    len(summaries),
	})
}

// handleCloseTaskTerminalSession terminates an active terminal session for a task.
// DELETE /api/tasks/{id}/terminal or DELETE /api/v1/tasks/{id}/terminal
func (s *Server) handleCloseTaskTerminalSession(w http.ResponseWriter, r *http.Request) {
	if s.terminalManager == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "terminal manager is not configured")
		return
	}

	taskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if taskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	if err := s.terminalManager.CloseSession(taskID); err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to close terminal session: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "closed",
		"task_id": taskID,
	})
}

type runTaskAgentPayload struct {
	AgentID string `json:"agent_id"`
	Prompt  string `json:"prompt"`
	Binary  string `json:"binary"`
	Cols    uint16 `json:"cols"`
	Rows    uint16 `json:"rows"`
}

// handleRunTaskAgent spawns an autonomous or interactive agent process bound to the task PTY.
// POST /api/v1/tasks/{id}/run-agent
func (s *Server) handleRunTaskAgent(w http.ResponseWriter, r *http.Request) {
	if !s.requireOAuthOwner(w, r) {
		return
	}
	if s.terminalManager == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "terminal manager is not configured")
		return
	}

	taskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if taskID == "" {
		taskID = strings.TrimSpace(r.URL.Query().Get("id"))
	}
	if taskID == "" {
		taskID = strings.TrimSpace(r.URL.Query().Get("task_id"))
	}
	if taskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	var payload runTaskAgentPayload
	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}

	if _, busy := s.taskLaunches.LoadOrStore(taskID, true); busy {
		respondJSONError(w, 409, "Task sedang memulai sesi. Buka sesi yang ada.")
		return
	}
	defer s.taskLaunches.Delete(taskID)
	if history := s.terminalManager.GetTaskHistory(taskID); len(history) > 0 && history[len(history)-1].Status == "running" {
		respondJSONError(w, 409, "Task sudah memiliki sesi aktif. Buka terminal task.")
		return
	}
	agentID := strings.TrimSpace(payload.AgentID)
	if agentID == "" {
		respondJSONError(w, http.StatusBadRequest, "Pilih agen terdaftar terlebih dahulu")
		return
	}
	if s.agentRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "Agent registry unavailable")
		return
	}
	registeredAgent, err := s.agentRepo.GetAgentByID(r.Context(), agentID)
	if err != nil || registeredAgent == nil {
		respondJSONError(w, http.StatusNotFound, "Agen tidak terdaftar")
		return
	}
	if !registeredAgent.Enabled {
		respondJSONError(w, http.StatusConflict, "Agen dinonaktifkan")
		return
	}
	ownerID := s.accountOwner(w, r)
	if ownerID == "" {
		return
	}
	profileHome, err := s.ownedAgentProfileHome(r.Context(), ownerID, registeredAgent)
	if err != nil {
		respondJSONError(w, http.StatusConflict, err.Error())
		return
	}
	if profileHome != "" && payload.Binary != "" && payload.Binary != "agy" && payload.Binary != "antigravity" {
		respondJSONError(w, http.StatusBadRequest, "Gunakan jenis agen sesuai koneksinya")
		return
	}
	agentName := registeredAgent.Name
	if payload.Binary == "" {
		switch registeredAgent.Harness {
		case "claude":
			payload.Binary = "claude"
		case "codex":
			payload.Binary = "codex"
		default:
			payload.Binary = "agy"
		}
	}

	taskPrompt := strings.TrimSpace(payload.Prompt)
	workDir := ""
	taskFound := false
	sourceRoom := ""

	// 1. Check project tasks repository
	if s.projectStudioRepo != nil {
		if pt, err := s.projectStudioRepo.GetTaskByID(r.Context(), taskID); err == nil && pt != nil {
			taskFound = true
			if taskPrompt == "" {
				taskPrompt = pt.Description
				if taskPrompt == "" {
					taskPrompt = pt.Title
				}
			}
			pt.AssigneeID = &agentID
			pt.ExecutionStatus = "running"
			_ = s.projectStudioRepo.UpdateTask(r.Context(), pt)
			_ = s.projectStudioRepo.UpdateTaskExecution(r.Context(), taskID, "running", pt.AgentOutput)
		}
	}

	// Canonical tasks persist an attempt before the process is launched.
	var attempt *task.TaskAttempt
	if s.taskSvc != nil {
		if detail, e := s.taskSvc.GetTaskDetail(r.Context(), taskID); e == nil && detail != nil {
			taskFound = true
			sourceRoom = detail.SourceRoomID
			if detail.ProjectID != "" && s.projectSvc != nil {
				project, e := s.projectSvc.Get(r.Context(), detail.ProjectID)
				if e != nil || project == nil {
					respondJSONError(w, 409, "Project task tidak tersedia")
					return
				}
				if project.RepoURL != "" {
					info, e := os.Stat(project.LocalPath)
					if project.LocalPath == "" || e != nil || !info.IsDir() {
						respondJSONError(w, 409, "Repository project belum siap. Sinkronkan repository sebelum menjalankan task.")
						return
					}
					workDir = project.LocalPath
				}
			}

			if taskPrompt == "" {
				taskPrompt = detail.Instructions
			}
			attempt, err = s.taskSvc.CreateAttempt(r.Context(), taskID, "owner", task.CreateAttemptRequest{RunnerID: "local_terminal", AgentName: agentID})
			if errors.Is(err, task.ErrTaskActive) {
				respondJSONError(w, 409, "Task sudah memiliki attempt aktif; periksa sesi sebelum mengulang.")
				return
			}
			if err != nil {
				respondJSONError(w, 503, "Attempt belum dapat disimpan; eksekusi belum dimulai.")
				return
			}
			doing := task.TaskStatusDoing
			if _, err = s.taskSvc.UpdateTask(r.Context(), taskID, "owner", task.UpdateTaskRequest{Status: &doing, AssignedAgent: &agentID}); err != nil {
				_, _ = s.taskSvc.UpdateAttemptStatus(context.Background(), attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed, ErrorMessage: "Status task gagal disimpan sebelum peluncuran."})
				respondJSONError(w, 503, "Status task belum tersimpan")
				return
			}
			_, err = s.taskSvc.UpdateAttemptStatus(r.Context(), attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusRunning})
			if err != nil {
				respondJSONError(w, 503, "Status attempt belum tersimpan; proses belum dimulai.")
				return
			}
		}
	}

	if !taskFound {
		respondJSONError(w, 404, "Task tidak ditemukan")
		return
	}

	var output *taskOutputRecorder
	if attempt != nil {
		output = newTaskOutputRecorder(s.taskSvc, attempt.ID)
	}
	// 3. Spawn PTY process via Manager
	runCfg := terminal.TaskAgentRunConfig{
		TaskID:    taskID,
		AgentID:   agentID,
		AgentName: agentName,
		Command:   payload.Binary,
		Prompt:    taskPrompt,
		WorkDir:   workDir,
		Cols:      payload.Cols,
		Rows:      payload.Rows,
		Mode:      terminal.ModeInteractive,
		OnOutput: func(chunk []byte) {
			if output != nil {
				output.append(chunk)
			}
		},
		OnExit: func(h *terminal.TaskRunHistory) {
			defer func() {
				if sourceRoom != "" {
					s.publishInbox(sourceRoom)
				}
			}()
			if output != nil {
				output.flush()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if s.projectStudioRepo != nil {
				_ = s.projectStudioRepo.UpdateTaskExecution(ctx, taskID, h.Status, fmt.Sprintf("Runner exited with code %d (%s). Perlu review hasil dan bukti.", h.ExitCode, h.Status))
			}
			if attempt != nil && s.taskSvc != nil {
				state := task.AttemptStatusFailed
				if h.ExitCode == 0 {
					state = task.AttemptStatusSucceeded
				}
				if _, e := s.taskSvc.UpdateAttemptStatus(ctx, attempt.ID, "runner", task.UpdateAttemptRequest{Status: state, ExitCode: &h.ExitCode, ErrorMessage: output.failureNote()}); e == nil {
					_ = s.taskSvc.ReconcileLocalTask(ctx, taskID, attempt.ID)
				}
			}
		},
	}

	if profileHome != "" {
		runCfg.Env = agent.AntigravityProfileEnvironment(profileHome)
	}
	sess, err := s.terminalManager.RunTaskAgent(r.Context(), runCfg)
	if err != nil {
		if attempt != nil {
			_, _ = s.taskSvc.UpdateAttemptStatus(context.Background(), attempt.ID, "runner", task.UpdateAttemptRequest{Status: task.AttemptStatusFailed, ErrorMessage: "CLI tidak dapat dimulai. Periksa instalasi dan koneksi agen."})
			status := task.TaskStatusBlocked
			_, _ = s.taskSvc.UpdateTask(context.Background(), taskID, "runner", task.UpdateTaskRequest{Status: &status})
		}
		if s.projectStudioRepo != nil {
			_ = s.projectStudioRepo.UpdateTaskExecution(context.Background(), taskID, "failed", "CLI tidak dapat dimulai.")
		}
		respondJSONError(w, http.StatusInternalServerError, "CLI tidak dapat dimulai. Periksa instalasi dan koneksi agen.")
		return
	}

	if sourceRoom != "" {
		s.publishInbox(sourceRoom)
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "running",
		"session_id":   sess.ID(),
		"task_id":      taskID,
		"pid":          sess.PTY().PID(),
		"terminal_url": fmt.Sprintf("/api/v1/tasks/%s/terminal", taskID),
		"agent_id":     agentID,
		"agent_name":   agentName,
		"attempt":      attempt,
	})
}

// handleGetTaskRunHistory returns execution history of an agent runner on a task.
// GET /api/v1/tasks/{id}/run-agent/history
func (s *Server) handleGetTaskRunHistory(w http.ResponseWriter, r *http.Request) {
	if s.accountOwner(w, r) == "" {
		return
	}
	if s.terminalManager == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "terminal manager is not configured")
		return
	}

	taskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if taskID == "" {
		taskID = strings.TrimSpace(r.URL.Query().Get("id"))
	}
	if taskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}

	history := s.terminalManager.GetTaskHistory(taskID)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"task_id": taskID,
		"history": history,
		"count":   len(history),
	})
}
