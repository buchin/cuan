package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/studio"
	"github.com/buchin/cuan/internal/task"
)

// CreateProjectTaskPayload defines the request body for creating a task in a project.
type CreateProjectTaskPayload struct {
	ID           string        `json:"id,omitempty"`
	ProjectIDs   []string      `json:"project_ids,omitempty"`
	RoomID       *string       `json:"room_id,omitempty"`
	Title        string        `json:"title"`
	Description  string        `json:"description"`
	Status       string        `json:"status"`   // 'backlog', 'doing', 'review', 'done'
	Priority     string        `json:"priority"` // 'low', 'normal', 'high', 'urgent'
	DoDChecklist []db.DoDItem  `json:"dod_checklist"`
	AssigneeID   *string       `json:"assignee_id,omitempty"`
	ParentTaskID    *string       `json:"parent_task_id,omitempty"`
	BatchID         *string       `json:"batch_id,omitempty"`
	Tags            []string      `json:"tags"`
	AgentOutput     string        `json:"agent_output,omitempty"`
	ExecutionStatus string        `json:"execution_status,omitempty"`
}

// UpdateProjectTaskPayload defines the request body for updating a project task.
type UpdateProjectTaskPayload struct {
	ProjectIDs      *[]string     `json:"project_ids,omitempty"`
	RoomID          *string       `json:"room_id,omitempty"`
	Title           *string       `json:"title,omitempty"`
	Description     *string       `json:"description,omitempty"`
	Status          *string       `json:"status,omitempty"`
	Priority        *string       `json:"priority,omitempty"`
	DoDChecklist    *[]db.DoDItem `json:"dod_checklist,omitempty"`
	AssigneeID      *string       `json:"assignee_id,omitempty"`
	ParentTaskID    *string       `json:"parent_task_id,omitempty"`
	BatchID         *string       `json:"batch_id,omitempty"`
	Tags            *[]string     `json:"tags,omitempty"`
	AgentOutput     *string       `json:"agent_output,omitempty"`
	ExecutionStatus *string       `json:"execution_status,omitempty"`
}

// PatchTaskStatusPayload defines the request body for updating only the status of a project task.
type PatchTaskStatusPayload struct {
	Status string `json:"status"`
}

// handleCreateProjectTask creates a new project task with optional fan-out and batch attributes.
// POST /api/projects/{id}/tasks
func (s *Server) handleCreateProjectTask(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	var payload CreateProjectTaskPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
		return
	}

	title := strings.TrimSpace(payload.Title)
	if title == "" {
		respondJSONError(w, http.StatusBadRequest, "task title cannot be empty")
		return
	}

	status := strings.TrimSpace(payload.Status)
	if status == "" {
		status = "backlog"
	}
	normStatus, err := db.NormalizeTaskStatus(status)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if normStatus == "doing" && s.isProjectActive(r.Context(), projectID, "") {
		normStatus = "queued"
	}

	priority := strings.TrimSpace(payload.Priority)
	if priority == "" {
		priority = "normal"
	}
	normPriority, err := db.NormalizeTaskPriority(priority)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if payload.RoomID != nil && strings.TrimSpace(*payload.RoomID) != "" {
		trimmedRoom := strings.TrimSpace(*payload.RoomID)
		if !db.IsValidUUID(trimmedRoom) {
			respondJSONError(w, http.StatusBadRequest, "invalid room_id uuid format")
			return
		}
		payload.RoomID = &trimmedRoom
	} else {
		payload.RoomID = nil
	}

	if payload.ParentTaskID != nil && strings.TrimSpace(*payload.ParentTaskID) != "" {
		trimmedParent := strings.TrimSpace(*payload.ParentTaskID)
		if !db.IsValidUUID(trimmedParent) {
			respondJSONError(w, http.StatusBadRequest, "invalid parent_task_id uuid format")
			return
		}
		payload.ParentTaskID = &trimmedParent
	} else {
		payload.ParentTaskID = nil
	}

	checklist := payload.DoDChecklist
	if checklist == nil {
		checklist = []db.DoDItem{}
	}

	tags := payload.Tags
	if tags == nil {
		tags = []string{}
	}

	normExec := "idle"
	if payload.ExecutionStatus != "" {
		ne, err := db.NormalizeTaskExecutionStatus(payload.ExecutionStatus)
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		normExec = ne
	}

	pids := payload.ProjectIDs
	if len(pids) == 0 && projectID != "" {
		pids = []string{projectID}
	}

	task := db.ProjectTask{
		ID:              payload.ID,
		ProjectID:       projectID,
		ProjectIDs:      pids,
		RoomID:          payload.RoomID,
		Title:           title,
		Description:     strings.TrimSpace(payload.Description),
		Status:          normStatus,
		Priority:        normPriority,
		DoDChecklist:    checklist,
		AssigneeID:      payload.AssigneeID,
		ParentTaskID:    payload.ParentTaskID,
		BatchID:         payload.BatchID,
		Tags:            tags,
		AgentOutput:     payload.AgentOutput,
		ExecutionStatus: normExec,
	}

	if err := s.projectStudioRepo.CreateTask(r.Context(), &task); err != nil {
		if errors.Is(err, db.ErrDuplicateTask) {
			respondJSONError(w, http.StatusConflict, "project task already exists")
			return
		}
		if errors.Is(err, db.ErrEmptyTaskTitle) || errors.Is(err, db.ErrEmptyProjectID) || errors.Is(err, db.ErrInvalidUUID) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to create project task: "+err.Error())
		return
	}

	if s.taskProjectRepo != nil && len(pids) > 0 {
		_ = s.taskProjectRepo.SetTaskProjects(r.Context(), task.ID, pids)
	}

	var dispatchRes *studio.ExecutionDispatchResult
	if s.agentExecutionHook != nil && task.Status == "doing" {
		dispatchRes, _ = s.agentExecutionHook.OnTaskStatusChanged(r.Context(), &task, "backlog", task.Status)
	}

	if s.roomEventBroker != nil && task.RoomID != nil && *task.RoomID != "" {
		s.roomEventBroker.Broadcast(*task.RoomID, "task.created", task.ToDTO())
	}

	taskDTO := task.ToDTO()
	taskDTO.ProjectIDs = pids
	resp := map[string]interface{}{
		"task": taskDTO,
	}
	if dispatchRes != nil {
		resp["execution_dispatch"] = dispatchRes
	}

	respondJSON(w, http.StatusCreated, resp)
}

// handleListProjectTasks retrieves project tasks matching query filters (?parent_task_id=, ?batch_id=, ?tag=, ?status=, etc.).
// GET /api/projects/{id}/tasks
func (s *Server) handleListProjectTasks(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	q := r.URL.Query()
	filter := db.ProjectTaskFilter{
		ProjectID:       projectID,
		Status:          strings.TrimSpace(q.Get("status")),
		Priority:        strings.TrimSpace(q.Get("priority")),
		ExecutionStatus: strings.TrimSpace(q.Get("execution_status")),
	}

	if parentID := strings.TrimSpace(q.Get("parent_task_id")); parentID != "" {
		filter.ParentTaskID = &parentID
	}
	if batchID := strings.TrimSpace(q.Get("batch_id")); batchID != "" {
		filter.BatchID = &batchID
	}
	if roomID := strings.TrimSpace(q.Get("room_id")); roomID != "" {
		filter.RoomID = &roomID
	}
	if assigneeID := strings.TrimSpace(q.Get("assignee_id")); assigneeID != "" {
		filter.AssigneeID = &assigneeID
	}

	tag := strings.TrimSpace(q.Get("tag"))
	if tag == "" {
		tag = strings.TrimSpace(q.Get("tags"))
	}
	filter.Tag = tag

	tasks, err := s.projectStudioRepo.ListTasks(r.Context(), filter)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to list project tasks: "+err.Error())
		return
	}

	dtos := make([]db.ProjectTaskDTO, len(tasks))
	for i, t := range tasks {
		dto := t.ToDTO()
		if s.taskProjectRepo != nil {
			if linkedPids, pErr := s.taskProjectRepo.GetProjectsByTask(r.Context(), t.ID); pErr == nil && len(linkedPids) > 0 {
				dto.ProjectIDs = linkedPids
			}
		}
		dtos[i] = dto
	}

	// Support multi-project filter via ?project_ids=
	projectIDsParam := strings.TrimSpace(q.Get("project_ids"))
	if projectIDsParam != "" {
		reqPids := strings.Split(projectIDsParam, ",")
		var filteredDtos []db.ProjectTaskDTO
		for _, dto := range dtos {
			matched := false
			for _, pid := range reqPids {
				pid = strings.TrimSpace(pid)
				if pid == "" {
					continue
				}
				if dto.ProjectID == pid {
					matched = true
					break
				}
				for _, linked := range dto.ProjectIDs {
					if linked == pid {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if matched {
				filteredDtos = append(filteredDtos, dto)
			}
		}
		dtos = filteredDtos
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": dtos,
		"count": len(dtos),
	})
}

// handleGetProjectTask retrieves a single project task by its ID.
// GET /api/tasks/{id} or GET /api/projects/tasks/{id}
func (s *Server) handleGetProjectTask(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	taskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if taskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}
	if !db.IsValidUUID(taskID) {
		respondJSONError(w, http.StatusBadRequest, "invalid task uuid format")
		return
	}

	task, err := s.projectStudioRepo.GetTaskByID(r.Context(), taskID)
	if err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "project task not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get project task: "+err.Error())
		return
	}

	dto := task.ToDTO()
	if s.taskProjectRepo != nil {
		if linkedPids, pErr := s.taskProjectRepo.GetProjectsByTask(r.Context(), task.ID); pErr == nil && len(linkedPids) > 0 {
			dto.ProjectIDs = linkedPids
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"task": dto,
	})
}

// handleUpdateProjectTask updates details of a project task.
// PUT /api/tasks/{id} or PUT /api/projects/tasks/{id}
func (s *Server) handleUpdateProjectTask(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	taskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if taskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}
	if !db.IsValidUUID(taskID) {
		respondJSONError(w, http.StatusBadRequest, "invalid task uuid format")
		return
	}

	task, err := s.projectStudioRepo.GetTaskByID(r.Context(), taskID)
	if err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "project task not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get project task: "+err.Error())
		return
	}

	oldStatus := task.Status

	var payload UpdateProjectTaskPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
		return
	}

	if payload.Title != nil {
		t := strings.TrimSpace(*payload.Title)
		if t == "" {
			respondJSONError(w, http.StatusBadRequest, "task title cannot be empty")
			return
		}
		task.Title = t
	}

	if payload.Description != nil {
		task.Description = strings.TrimSpace(*payload.Description)
	}

	if payload.Status != nil {
		normStatus, err := db.NormalizeTaskStatus(*payload.Status)
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		if normStatus == "doing" && oldStatus != "doing" && s.isProjectActive(r.Context(), task.ProjectID, task.ID) {
			normStatus = "queued"
		}
		task.Status = normStatus
	}

	if payload.Priority != nil {
		normPriority, err := db.NormalizeTaskPriority(*payload.Priority)
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		task.Priority = normPriority
	}

	if payload.DoDChecklist != nil {
		task.DoDChecklist = *payload.DoDChecklist
	}

	if payload.AssigneeID != nil {
		task.AssigneeID = payload.AssigneeID
	}

	if payload.ParentTaskID != nil {
		pID := strings.TrimSpace(*payload.ParentTaskID)
		if pID != "" {
			if !db.IsValidUUID(pID) {
				respondJSONError(w, http.StatusBadRequest, "invalid parent_task_id uuid format")
				return
			}
			task.ParentTaskID = &pID
		} else {
			task.ParentTaskID = nil
		}
	}

	if payload.BatchID != nil {
		bID := strings.TrimSpace(*payload.BatchID)
		if bID != "" {
			task.BatchID = &bID
		} else {
			task.BatchID = nil
		}
	}

	if payload.Tags != nil {
		task.Tags = *payload.Tags
	}

	if payload.RoomID != nil {
		rID := strings.TrimSpace(*payload.RoomID)
		if rID != "" {
			if !db.IsValidUUID(rID) {
				respondJSONError(w, http.StatusBadRequest, "invalid room_id uuid format")
				return
			}
			task.RoomID = &rID
		} else {
			task.RoomID = nil
		}
	}

	if payload.AgentOutput != nil {
		task.AgentOutput = *payload.AgentOutput
	}

	if payload.ExecutionStatus != nil {
		normExec, err := db.NormalizeTaskExecutionStatus(*payload.ExecutionStatus)
		if err != nil {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		task.ExecutionStatus = normExec
	}

	if payload.ProjectIDs != nil {
		task.ProjectIDs = *payload.ProjectIDs
		if s.taskProjectRepo != nil {
			_ = s.taskProjectRepo.SetTaskProjects(r.Context(), task.ID, *payload.ProjectIDs)
		}
	}

	if err := s.projectStudioRepo.UpdateTask(r.Context(), task); err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "project task not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to update project task: "+err.Error())
		return
	}

	var dispatchRes *studio.ExecutionDispatchResult
	if s.agentExecutionHook != nil && task.Status == "doing" && oldStatus != "doing" {
		dispatchRes, _ = s.agentExecutionHook.OnTaskStatusChanged(r.Context(), task, oldStatus, task.Status)
	}

	taskDTO := task.ToDTO()
	if payload.ProjectIDs != nil {
		taskDTO.ProjectIDs = *payload.ProjectIDs
	} else if s.taskProjectRepo != nil {
		if linkedPids, err := s.taskProjectRepo.GetProjectsByTask(r.Context(), task.ID); err == nil && len(linkedPids) > 0 {
			taskDTO.ProjectIDs = linkedPids
		}
	}

	if s.roomEventBroker != nil && task.RoomID != nil && *task.RoomID != "" {
		eventData := map[string]interface{}{
			"task": taskDTO,
		}
		if dispatchRes != nil {
			eventData["execution_dispatch"] = dispatchRes
		}
		s.roomEventBroker.Broadcast(*task.RoomID, "task.updated", eventData)
	}

	resp := map[string]interface{}{
		"task": taskDTO,
	}
	if dispatchRes != nil {
		resp["execution_dispatch"] = dispatchRes
	}

	respondJSON(w, http.StatusOK, resp)
}

// handlePatchProjectTaskStatus updates only the status of a project task.
// PATCH /api/tasks/{id}/status or PATCH /api/projects/tasks/{id}/status
func (s *Server) handlePatchProjectTaskStatus(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	taskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if taskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}
	if !db.IsValidUUID(taskID) {
		respondJSONError(w, http.StatusBadRequest, "invalid task uuid format")
		return
	}

	var payload PatchTaskStatusPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request json: "+err.Error())
		return
	}

	normStatus, err := db.NormalizeTaskStatus(payload.Status)
	if err != nil {
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	oldTask, err := s.projectStudioRepo.GetTaskByID(r.Context(), taskID)
	if err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "project task not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get project task: "+err.Error())
		return
	}
	oldStatus := oldTask.Status

	if normStatus == "doing" && oldStatus != "doing" && s.isProjectActive(r.Context(), oldTask.ProjectID, taskID) {
		normStatus = "queued"
	}

	if err := s.projectStudioRepo.UpdateTaskStatus(r.Context(), taskID, normStatus); err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "project task not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to update task status: "+err.Error())
		return
	}

	task, err := s.projectStudioRepo.GetTaskByID(r.Context(), taskID)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to get updated project task: "+err.Error())
		return
	}

	var dispatchRes *studio.ExecutionDispatchResult
	if s.agentExecutionHook != nil && normStatus == "doing" && oldStatus != "doing" {
		dispatchRes, _ = s.agentExecutionHook.OnTaskStatusChanged(r.Context(), task, oldStatus, normStatus)
	}

	if s.roomEventBroker != nil && task.RoomID != nil && *task.RoomID != "" {
		eventData := map[string]interface{}{
			"id":     taskID,
			"status": normStatus,
			"task":   task.ToDTO(),
		}
		if dispatchRes != nil {
			eventData["execution_dispatch"] = dispatchRes
		}
		s.roomEventBroker.Broadcast(*task.RoomID, "task.status_changed", eventData)
	}

	resp := map[string]interface{}{
		"task": task.ToDTO(),
	}
	if dispatchRes != nil {
		resp["execution_dispatch"] = dispatchRes
	}

	respondJSON(w, http.StatusOK, resp)
}

// handleDeleteProjectTask deletes a project task by its ID.
// DELETE /api/tasks/{id} or DELETE /api/projects/tasks/{id}
func (s *Server) handleDeleteProjectTask(w http.ResponseWriter, r *http.Request) {
	if s.projectStudioRepo == nil {
		respondJSONError(w, http.StatusServiceUnavailable, "project studio repository is not configured")
		return
	}

	taskID := strings.TrimSpace(chi.URLParam(r, "id"))
	if taskID == "" {
		respondJSONError(w, http.StatusBadRequest, "task id is required")
		return
	}
	if !db.IsValidUUID(taskID) {
		respondJSONError(w, http.StatusBadRequest, "invalid task uuid format")
		return
	}

	task, err := s.projectStudioRepo.GetTaskByID(r.Context(), taskID)
	if err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "project task not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to get project task: "+err.Error())
		return
	}

	if err := s.projectStudioRepo.DeleteTask(r.Context(), taskID); err != nil {
		if errors.Is(err, db.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "project task not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, "failed to delete project task: "+err.Error())
		return
	}

	if s.roomEventBroker != nil && task.RoomID != nil && *task.RoomID != "" {
		s.roomEventBroker.Broadcast(*task.RoomID, "task.deleted", map[string]string{
			"id":         taskID,
			"project_id": task.ProjectID,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status": "deleted",
		"id":     taskID,
	})
}

// ProjectRunnerStateResponse represents real-time runner concurrency and queue state for a project.
type ProjectRunnerStateResponse struct {
	ProjectID    string                     `json:"project_id"`
	IsLocked     bool                       `json:"is_locked"`
	ActiveRunner *ProjectRunnerActiveDetail `json:"active_runner,omitempty"`
	QueuedTasks  []ProjectRunnerQueuedTask  `json:"queued_tasks"`
}

// ProjectRunnerActiveDetail represents the active lock-holder runner information.
type ProjectRunnerActiveDetail struct {
	TaskID          string    `json:"task_id"`
	Title           string    `json:"title"`
	AgentName       string    `json:"agent_name"`
	PID             int       `json:"pid,omitempty"`
	StartedAt       time.Time `json:"started_at"`
	DurationSeconds int       `json:"duration_seconds"`
	LiveActivity    string    `json:"live_activity,omitempty"`
	RecentOutput    string    `json:"recent_output,omitempty"`
}

// ProjectRunnerQueuedTask represents a task waiting in queue for the project lock.
type ProjectRunnerQueuedTask struct {
	QueuePosition   int       `json:"queue_position"`
	TaskID          string    `json:"task_id"`
	Title           string    `json:"title"`
	AgentName       string    `json:"agent_name"`
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	DurationSeconds int       `json:"duration_seconds"`
}

// handleGetProjectRunnerState provides real-time runner and queue state visibility.
// GET /api/projects/{id}/runner-state or GET /api/v1/projects/{id}/runner-state
func (s *Server) handleGetProjectRunnerState(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(chi.URLParam(r, "id"))
	if projectID == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	isLocked := s.isProjectActive(r.Context(), projectID, "")

	var activeTaskID string
	var pid int
	var startedAt time.Time
	var recentOutput string

	if s.terminalManager != nil {
		if active := s.terminalManager.GetProjectActiveRunner(projectID); active != nil {
			activeTaskID = active.TaskID
			pid = active.PID
			startedAt = active.CreatedAt
			recentOutput = active.RecentOutput
		}
	}

	if activeTaskID == "" && s.taskSvc != nil {
		if tasks, err := s.taskSvc.ListTasks(r.Context(), task.TaskFilter{ProjectID: projectID}); err == nil {
			for _, t := range tasks {
				if t.Status == task.TaskStatusDoing || t.Status == "working" {
					activeTaskID = t.ID
					startedAt = t.UpdatedAt
					break
				}
				if detail, err := s.taskSvc.GetTaskDetail(r.Context(), t.ID); err == nil && detail.LatestAttempt != nil && detail.LatestAttempt.Status == task.AttemptStatusRunning {
					activeTaskID = t.ID
					if detail.LatestAttempt.StartedAt != nil {
						startedAt = *detail.LatestAttempt.StartedAt
					}
					break
				}
			}
		}
	}

	var activeRunner *ProjectRunnerActiveDetail
	if activeTaskID != "" {
		taskTitle := activeTaskID
		taskAgent := "agy"
		var liveActivity string
		if s.taskSvc != nil {
			if detail, err := s.taskSvc.GetTaskDetail(r.Context(), activeTaskID); err == nil && detail != nil {
				taskTitle = detail.Title
				if detail.AssignedAgent != "" {
					taskAgent = detail.AssignedAgent
				} else if detail.LatestAttempt != nil && detail.LatestAttempt.AgentName != "" {
					taskAgent = detail.LatestAttempt.AgentName
				}
				for i := len(detail.Activities) - 1; i >= 0; i-- {
					act := detail.Activities[i]
					if act != nil && strings.TrimSpace(act.Message) != "" {
						liveActivity = act.Message
						break
					}
				}
				if liveActivity == "" && detail.LatestAttempt != nil {
					liveActivity = s.formatAttemptTerminalLogs(r.Context(), detail.LatestAttempt.ID, 15)
				}
			}
		}
		var durSec int
		if !startedAt.IsZero() {
			durSec = int(time.Since(startedAt).Seconds())
		}
		activeRunner = &ProjectRunnerActiveDetail{
			TaskID:          activeTaskID,
			Title:           taskTitle,
			AgentName:       taskAgent,
			PID:             pid,
			StartedAt:       startedAt,
			DurationSeconds: durSec,
			LiveActivity:    liveActivity,
			RecentOutput:    recentOutput,
		}
	}

	queuedTasks := []ProjectRunnerQueuedTask{}
	if s.taskSvc != nil {
		if tasks, err := s.taskSvc.ListTasks(r.Context(), task.TaskFilter{ProjectID: projectID}); err == nil {
			var qTasks []*task.TaskItemResponse
			for _, t := range tasks {
				if t.ID == activeTaskID {
					continue
				}
				if t.Status == task.TaskStatusQueued || t.Status == "pending" {
					qTasks = append(qTasks, t)
				} else if detail, err := s.taskSvc.GetTaskDetail(r.Context(), t.ID); err == nil && detail != nil {
					if detail.LatestAttempt != nil && detail.LatestAttempt.Status == task.AttemptStatusQueued {
						qTasks = append(qTasks, t)
					}
				}
			}
			sort.Slice(qTasks, func(i, j int) bool {
				return qTasks[i].CreatedAt.Before(qTasks[j].CreatedAt)
			})
			for idx, qt := range qTasks {
				agentName := qt.AssignedAgent
				if agentName == "" {
					agentName = "agy"
				}
				durSec := int(time.Since(qt.CreatedAt).Seconds())
				queuedTasks = append(queuedTasks, ProjectRunnerQueuedTask{
					QueuePosition:   idx + 1,
					TaskID:          qt.ID,
					Title:           qt.Title,
					AgentName:       agentName,
					Status:          string(qt.Status),
					CreatedAt:       qt.CreatedAt,
					DurationSeconds: durSec,
				})
			}
		}
	}

	respondJSON(w, http.StatusOK, ProjectRunnerStateResponse{
		ProjectID:    projectID,
		IsLocked:     isLocked,
		ActiveRunner: activeRunner,
		QueuedTasks:  queuedTasks,
	})
}
