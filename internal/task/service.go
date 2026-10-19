package task

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/project"
)

type ProjectFinder interface {
	Get(ctx context.Context, id string) (*project.ProjectResponse, error)
}

type Service struct {
	attemptMu     sync.Mutex
	store         Store
	projectFinder ProjectFinder
}

func NewService(store Store, projectFinder ProjectFinder) *Service {
	return &Service{
		store:         store,
		projectFinder: projectFinder,
	}
}

func (s *Service) SeedSprint1Tasks(ctx context.Context) error {
	existing, err := s.store.ListTasks(ctx, TaskFilter{})
	if err != nil || len(existing) > 0 {
		return nil
	}

	seeds := []struct {
		id           string
		title        string
		instructions string
		status       string
		priority     string
	}{
		{"T1", "Establish the monorepo foundation and agent adapter feasibility", "Prototype the agy adapter and Go + React Vite TypeScript monorepo under Apache-2.0", TaskStatusDone, PriorityHigh},
		{"T2", "Provide persistent installation and owner access", "Single-owner mode, bcrypt, takeover protection (403), and persistent PostgreSQL", TaskStatusDone, PriorityHigh},
		{"T3", "Connect an existing GitHub repository", "GitHub access with AES-256-GCM-protected PATs, 404/401 validation, and non-destructive deletion", TaskStatusDone, PriorityHigh},
		{"T4", "Build the task board and work history", "Task CRUD, separate task/attempt states, activity history, Kanban UI, and details", TaskStatusDoing, PriorityNormal},
		{"T5", "Execute tasks through a runner and one agent", "Authenticated runner, lease/heartbeat, cancellation/timeout, and CPU/RAM monitoring", TaskStatusBacklog, PriorityHigh},
		{"T6", "Produce Git changes and a pull request for review", "Per-task workspace, targeted validation, commit/push, and a PR in the Cuan repository", TaskStatusBacklog, PriorityHigh},
		{"T7", "Verify staging, updates, backup, and recovery", "Versioned deployment, isolated staging, backup/restore, health checks, and rollback drill", TaskStatusBacklog, PriorityNormal},
		{"T8", "Demonstrate dogfooding and prepare the OSS release", "Develop Cuan using Cuan, demo dogfooding, OSS readiness checklist", TaskStatusBacklog, PriorityNormal},
	}

	for _, seed := range seeds {
		t := &Task{
			ID:            seed.id,
			Title:         seed.title,
			Instructions:  seed.instructions,
			Status:        seed.status,
			Priority:      seed.priority,
			AssignedAgent: "agy",
			CreatedAt:     time.Now().UTC(),
			UpdatedAt:     time.Now().UTC(),
		}
		_ = s.store.CreateTask(ctx, t)
		act := &TaskActivity{
			ID:           generateID("act"),
			TaskID:       t.ID,
			ActivityType: ActivityTypeCreated,
			Author:       "system",
			NewStatus:    t.Status,
			Message:      fmt.Sprintf("Sprint 1 task '%s' initialized with status '%s'.", t.ID, t.Status),
			CreatedAt:    t.CreatedAt,
		}
		_ = s.store.CreateActivity(ctx, act)
	}

	return nil
}

func generateID(prefix string) string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func (s *Service) CreateTask(ctx context.Context, author string, req CreateTaskRequest) (*Task, error) {
	return s.createTaskWithID(ctx, author, req, generateID("task"))
}

// CreateTaskFromMessage uses the source UUID as the idempotency key. A lost
// response or concurrent click cannot create a second task for the same message.
func (s *Service) CreateTaskFromMessage(ctx context.Context, author, source string, req CreateTaskRequest) (*Task, error) {
	id := "msg_" + source
	req.SourceMessageID = source
	if existing, err := s.store.GetTask(ctx, id); err == nil {
		return existing, nil
	}
	result, err := s.createTaskWithID(ctx, author, req, id)
	if err != nil {
		if existing, e := s.store.GetTask(ctx, id); e == nil {
			return existing, nil
		}
	}
	return result, err
}
func (s *Service) createTaskWithID(ctx context.Context, author string, req CreateTaskRequest, id string) (*Task, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: task title is required", ErrInvalidTaskInput)
	}

	priority := strings.ToLower(strings.TrimSpace(req.Priority))
	if priority == "" {
		priority = PriorityNormal
	}
	switch priority {
	case PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent:
	default:
		return nil, fmt.Errorf("%w: invalid priority ('%s')", ErrInvalidTaskInput, priority)
	}

	assignedAgent := strings.TrimSpace(req.AssignedAgent)
	if assignedAgent == "" {
		assignedAgent = "agy"
	}

	projectIDs := req.ProjectIDs
	projectID := strings.TrimSpace(req.ProjectID)
	if projectID == "" && len(projectIDs) > 0 {
		projectID = projectIDs[0]
	}
	if len(projectIDs) == 0 && projectID != "" {
		projectIDs = []string{projectID}
	}

	t := &Task{
		ID:           id,
		SourceRoomID: req.SourceRoomID, SourceMessageID: req.SourceMessageID,
		ProjectID:     projectID,
		ProjectIDs:    projectIDs,
		Title:         title,
		Instructions:  strings.TrimSpace(req.Instructions),
		Status:        TaskStatusBacklog,
		Priority:      priority,
		AssignedAgent: assignedAgent,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	if err := s.store.CreateTask(ctx, t); err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	// Record task_created activity
	act := &TaskActivity{
		ID:           generateID("act"),
		TaskID:       t.ID,
		ActivityType: ActivityTypeCreated,
		Author:       author,
		NewStatus:    t.Status,
		Message:      fmt.Sprintf("Task created with priority '%s' and agent '%s'.", t.Priority, t.AssignedAgent),
		CreatedAt:    t.CreatedAt,
	}
	_ = s.store.CreateActivity(ctx, act)

	return t, nil
}

func (s *Service) GetTaskDetail(ctx context.Context, id string) (*TaskDetailResponse, error) {
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}

	latestAttempt, _ := s.store.GetLatestAttempt(ctx, id)
	attempts, _ := s.store.ListAttempts(ctx, id)
	if attempts == nil {
		attempts = []*TaskAttempt{}
	}
	activities, _ := s.store.ListActivities(ctx, id)
	if activities == nil {
		activities = []*TaskActivity{}
	}

	projectName := ""
	repoURL := ""
	if s.projectFinder != nil && t.ProjectID != "" {
		if p, err := s.projectFinder.Get(ctx, t.ProjectID); err == nil && p != nil {
			projectName = p.Name
			repoURL = p.RepoURL
		}
	}

	var projectNamesList []string
	for _, pid := range t.ProjectIDs {
		if s.projectFinder != nil {
			if p, err := s.projectFinder.Get(ctx, pid); err == nil && p != nil {
				projectNamesList = append(projectNamesList, p.Name)
			}
		}
	}

	return &TaskDetailResponse{
		Task:          *t,
		ProjectName:   projectName,
		ProjectNames:  projectNamesList,
		RepoURL:       repoURL,
		LatestAttempt: latestAttempt,
		Attempts:      attempts,
		Activities:    activities,
	}, nil
}

func (s *Service) ListTasks(ctx context.Context, filter TaskFilter) ([]*TaskItemResponse, error) {
	tasks, err := s.store.ListTasks(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	// Cache project names to avoid duplicate lookups
	projectNames := make(map[string]string)

	var items []*TaskItemResponse
	for _, t := range tasks {
		pName := ""
		if t.ProjectID != "" {
			if cached, ok := projectNames[t.ProjectID]; ok {
				pName = cached
			} else if s.projectFinder != nil {
				if p, err := s.projectFinder.Get(ctx, t.ProjectID); err == nil && p != nil {
					pName = p.Name
					projectNames[t.ProjectID] = pName
				}
			}
		}

		var pNames []string
		for _, pid := range t.ProjectIDs {
			if cached, ok := projectNames[pid]; ok {
				pNames = append(pNames, cached)
			} else if s.projectFinder != nil {
				if p, err := s.projectFinder.Get(ctx, pid); err == nil && p != nil {
					projectNames[pid] = p.Name
					pNames = append(pNames, p.Name)
				}
			}
		}

		latestAttempt, _ := s.store.GetLatestAttempt(ctx, t.ID)
		items = append(items, &TaskItemResponse{
			Task:          *t,
			ProjectName:   pName,
			ProjectNames:  pNames,
			LatestAttempt: latestAttempt,
		})
	}

	return items, nil
}

func (s *Service) UpdateTask(ctx context.Context, id string, author string, req UpdateTaskRequest) (*Task, error) {
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}

	statusChanged := false
	oldStatus := t.Status
	newStatus := oldStatus

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, fmt.Errorf("%w: task title cannot be empty", ErrInvalidTaskInput)
		}
		t.Title = title
	}

	if req.Instructions != nil {
		t.Instructions = strings.TrimSpace(*req.Instructions)
	}

	if req.ProjectIDs != nil {
		t.ProjectIDs = *req.ProjectIDs
		if len(t.ProjectIDs) > 0 && t.ProjectID == "" {
			t.ProjectID = t.ProjectIDs[0]
		}
	}

	if req.Priority != nil {
		p := strings.ToLower(strings.TrimSpace(*req.Priority))
		switch p {
		case PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent:
			t.Priority = p
		default:
			return nil, fmt.Errorf("%w: invalid priority ('%s')", ErrInvalidTaskInput, p)
		}
	}

	if req.AssignedAgent != nil {
		agent := strings.TrimSpace(*req.AssignedAgent)
		if agent != "" {
			t.AssignedAgent = agent
		}
	}

	if req.Status != nil {
		st := strings.ToLower(strings.TrimSpace(*req.Status))
		switch st {
		case TaskStatusBacklog, "todo", TaskStatusDoing, TaskStatusReview, TaskStatusDone, TaskStatusBlocked:
			if st != oldStatus {
				statusChanged = true
				newStatus = st
				t.Status = st
			}
		default:
			return nil, fmt.Errorf("%w: invalid task status '%s'", ErrInvalidTaskInput, st)
		}
	}

	if req.BranchName != nil {
		t.BranchName = strings.TrimSpace(*req.BranchName)
	}
	if req.PRURL != nil {
		t.PRURL = strings.TrimSpace(*req.PRURL)
	}
	if req.PRNumber != nil {
		t.PRNumber = *req.PRNumber
	}
	if req.CommitSHA != nil {
		t.CommitSHA = strings.TrimSpace(*req.CommitSHA)
	}
	if req.DiffSummary != nil {
		t.DiffSummary = strings.TrimSpace(*req.DiffSummary)
	}
	if req.TestSummary != nil {
		t.TestSummary = strings.TrimSpace(*req.TestSummary)
	}

	t.UpdatedAt = time.Now().UTC()
	if err := s.store.UpdateTask(ctx, t); err != nil {
		return nil, fmt.Errorf("update task: %w", err)
	}

	// Auto-schedule revision attempt when returning from review to doing
	if oldStatus == TaskStatusReview && newStatus == TaskStatusDoing {
		existingAttempts, _ := s.store.ListAttempts(ctx, t.ID)
		hasActive := false
		for _, a := range existingAttempts {
			if a.Status == AttemptStatusQueued || a.Status == AttemptStatusRunning {
				hasActive = true
				break
			}
		}
		if !hasActive {
			attemptNum := len(existingAttempts) + 1
			revAttempt := &TaskAttempt{
				ID:            generateID("att"),
				TaskID:        t.ID,
				RunnerID:      "",
				AgentName:     t.AssignedAgent,
				Status:        AttemptStatusQueued,
				AttemptNumber: attemptNum,
				CreatedAt:     t.UpdatedAt,
			}
			_ = s.store.CreateAttempt(ctx, revAttempt)
		}
	}

	if statusChanged {
		actMsg := fmt.Sprintf("Status task diubah dari '%s' ke '%s'.", oldStatus, newStatus)
		if oldStatus == TaskStatusReview && newStatus == TaskStatusDoing {
			actMsg = "Task dikembalikan dari review ke doing untuk siklus revisi. Attempt baru dijadwalkan."
		}
		act := &TaskActivity{
			ID:           generateID("act"),
			TaskID:       t.ID,
			ActivityType: ActivityTypeStatusChange,
			Author:       author,
			OldStatus:    oldStatus,
			NewStatus:    newStatus,
			Message:      actMsg,
			CreatedAt:    t.UpdatedAt,
		}
		_ = s.store.CreateActivity(ctx, act)
	} else {
		act := &TaskActivity{
			ID:           generateID("act"),
			TaskID:       t.ID,
			ActivityType: ActivityTypeUpdated,
			Author:       author,
			Message:      "Atribut task diperbarui oleh owner.",
			CreatedAt:    t.UpdatedAt,
		}
		_ = s.store.CreateActivity(ctx, act)
	}

	return t, nil
}

func (s *Service) DeleteTask(ctx context.Context, id string) error {
	return s.store.DeleteTask(ctx, id)
}

func (s *Service) AddComment(ctx context.Context, taskID string, author string, message string) (*TaskActivity, error) {
	msg := strings.TrimSpace(message)
	if msg == "" {
		return nil, fmt.Errorf("%w: comment cannot be empty", ErrInvalidTaskInput)
	}

	// Verify task exists
	if _, err := s.store.GetTask(ctx, taskID); err != nil {
		return nil, err
	}

	act := &TaskActivity{
		ID:           generateID("act"),
		TaskID:       taskID,
		ActivityType: ActivityTypeComment,
		Author:       author,
		Message:      msg,
		CreatedAt:    time.Now().UTC(),
	}

	if err := s.store.CreateActivity(ctx, act); err != nil {
		return nil, fmt.Errorf("create comment activity: %w", err)
	}

	return act, nil
}

func (s *Service) CreateAttempt(ctx context.Context, taskID string, author string, req CreateAttemptRequest) (*TaskAttempt, error) {
	t, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	runnerID := strings.TrimSpace(req.RunnerID)
	if runnerID == "" {
		runnerID = "runner_default"
	}

	agentName := strings.TrimSpace(req.AgentName)
	if agentName == "" {
		agentName = t.AssignedAgent
		if agentName == "" {
			agentName = "agy"
		}
	}

	att := &TaskAttempt{
		ID:           generateID("att"),
		TaskID:       t.ID,
		RunnerID:     runnerID,
		AgentName:    agentName,
		Status:       AttemptStatusQueued,
		PeakRSSBytes: 0,
		CreatedAt:    time.Now().UTC(),
	}

	if runnerID == "local_terminal" {
		lease := time.Now().UTC().Add(11 * time.Minute)
		att.LeaseExpiresAt = &lease
	}
	if err := s.store.CreateAttempt(ctx, att); err != nil {
		return nil, fmt.Errorf("create attempt: %w", err)
	}

	// Record attempt_transition activity
	act := &TaskActivity{
		ID:           generateID("act"),
		TaskID:       t.ID,
		AttemptID:    &att.ID,
		ActivityType: ActivityTypeAttempt,
		Author:       author,
		NewStatus:    att.Status,
		Message:      fmt.Sprintf("Attempt %s dijadwalkan (agent: %s, runner: %s).", att.ID, att.AgentName, att.RunnerID),
		CreatedAt:    att.CreatedAt,
	}
	_ = s.store.CreateActivity(ctx, act)

	return att, nil
}

func (s *Service) UpdateAttemptStatus(ctx context.Context, attemptID string, author string, req UpdateAttemptRequest) (*TaskAttempt, error) {
	s.attemptMu.Lock()
	defer s.attemptMu.Unlock()
	att, err := s.store.GetAttempt(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	oldStatus := att.Status
	if att.Status != "queued" && att.Status != "running" {
		return att, nil
	}
	newStatus := strings.ToLower(strings.TrimSpace(req.Status))

	// CRITICAL DoD Rule:
	// "Kegagalan attempt tidak ditampilkan sebagai sukses. Agent tidak otomatis menganggap hasil accepted hanya karena exit code nol."
	if req.ExitCode != nil && *req.ExitCode != 0 {
		newStatus = AttemptStatusFailed
	}
	if newStatus == AttemptStatusFailed && req.ExitCode == nil {
		exitFail := 1
		att.ExitCode = &exitFail
	} else if req.ExitCode != nil {
		att.ExitCode = req.ExitCode
	}

	switch newStatus {
	case AttemptStatusQueued, AttemptStatusRunning, AttemptStatusSucceeded, AttemptStatusFailed, AttemptStatusCanceled, AttemptStatusInterrupted:
		att.Status = newStatus
	default:
		return nil, fmt.Errorf("%w: invalid attempt status '%s'", ErrInvalidTaskInput, newStatus)
	}

	now := time.Now().UTC()
	if newStatus == AttemptStatusRunning && att.StartedAt == nil {
		att.StartedAt = &now
	}
	if (newStatus == AttemptStatusSucceeded || newStatus == AttemptStatusFailed || newStatus == AttemptStatusCanceled || newStatus == AttemptStatusInterrupted) && att.FinishedAt == nil {
		att.FinishedAt = &now
	}

	if req.PeakRSSBytes != nil {
		att.PeakRSSBytes = *req.PeakRSSBytes
	}

	if req.ErrorMessage != "" {
		att.ErrorMessage = req.ErrorMessage
	}

	if err := s.store.UpdateAttempt(ctx, att); err != nil {
		if errors.Is(err, ErrAttemptFinal) {
			return s.store.GetAttempt(ctx, attemptID)
		}
		return nil, fmt.Errorf("update attempt: %w", err)
	}

	// Audit activity log
	msg := fmt.Sprintf("Attempt %s berubah status: '%s' → '%s'", att.ID, oldStatus, newStatus)
	if att.ExitCode != nil {
		msg += fmt.Sprintf(" (exit code: %d)", *att.ExitCode)
	}
	if att.ErrorMessage != "" {
		msg += fmt.Sprintf(" [Message: %s]", att.ErrorMessage)
	}

	act := &TaskActivity{
		ID:           generateID("act"),
		TaskID:       att.TaskID,
		AttemptID:    &att.ID,
		ActivityType: ActivityTypeAttempt,
		Author:       author,
		OldStatus:    oldStatus,
		NewStatus:    newStatus,
		Message:      msg,
		CreatedAt:    now,
	}
	_ = s.store.CreateActivity(ctx, act)

	return att, nil
}

// CancelTask marks the active attempt for a task as canceled and triggers process cancellation.
func (s *Service) CancelTask(ctx context.Context, taskID string, author string) (*TaskAttempt, error) {
	s.attemptMu.Lock()
	defer s.attemptMu.Unlock()
	t, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	latest, err := s.store.GetLatestAttempt(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if latest == nil {
		return nil, fmt.Errorf("%w: no attempt is available to cancel", ErrAttemptNotFound)
	}

	if latest.Status != AttemptStatusRunning && latest.Status != AttemptStatusQueued {
		return nil, fmt.Errorf("%w: cannot cancel an attempt with status '%s'", ErrInvalidTaskInput, latest.Status)
	}

	now := time.Now().UTC()
	latest.CancelRequested = true
	latest.Status = AttemptStatusCanceled
	latest.CanceledAt = &now
	latest.FinishedAt = &now
	exitCanceled := 130 // SIGINT exit code standard
	latest.ExitCode = &exitCanceled
	latest.ErrorMessage = fmt.Sprintf("Cancelled by %s", author)

	if err := s.store.UpdateAttempt(ctx, latest); err != nil {
		return nil, fmt.Errorf("update cancel attempt: %w", err)
	}

	// Update task status to blocked or review
	t.Status = TaskStatusBlocked
	t.UpdatedAt = now
	_ = s.store.UpdateTask(ctx, t)

	// Record activity
	act := &TaskActivity{
		ID:           generateID("act"),
		TaskID:       taskID,
		AttemptID:    &latest.ID,
		ActivityType: ActivityTypeCancelRequested,
		Author:       author,
		OldStatus:    t.Status,
		NewStatus:    TaskStatusBlocked,
		Message:      fmt.Sprintf("Cancellation of attempt %s requested by %s. The agent's entire process tree was stopped.", latest.ID, author),
		CreatedAt:    now,
	}
	_ = s.store.CreateActivity(ctx, act)

	return latest, nil
}

// RetryTask creates a brand-new execution attempt for a failed, canceled, or interrupted task.
func (s *Service) RetryTask(ctx context.Context, taskID string, author string, runnerID string, agentName string) (*TaskAttempt, error) {
	t, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	latest, err := s.store.GetLatestAttempt(ctx, taskID)
	if err != nil {
		return nil, err
	}

	nextAttemptNum := 1
	if latest != nil {
		if latest.Status == AttemptStatusRunning || latest.Status == AttemptStatusQueued {
			return nil, fmt.Errorf("%w: task is running or already queued", ErrInvalidTaskInput)
		}
		if latest.AttemptNumber > 0 {
			nextAttemptNum = latest.AttemptNumber + 1
		} else {
			attempts, _ := s.store.ListAttempts(ctx, taskID)
			nextAttemptNum = len(attempts) + 1
		}
	}

	if runnerID == "" {
		runnerID = "runner_default"
	}
	if agentName == "" {
		agentName = t.AssignedAgent
		if agentName == "" {
			agentName = "agy"
		}
	}

	now := time.Now().UTC()
	newAttempt := &TaskAttempt{
		ID:            generateID("att"),
		TaskID:        taskID,
		RunnerID:      runnerID,
		AgentName:     agentName,
		Status:        AttemptStatusQueued,
		AttemptNumber: nextAttemptNum,
		CreatedAt:     now,
	}

	if err := s.store.CreateAttempt(ctx, newAttempt); err != nil {
		return nil, fmt.Errorf("create retry attempt: %w", err)
	}

	// Update task status back to doing
	t.Status = TaskStatusDoing
	t.UpdatedAt = now
	_ = s.store.UpdateTask(ctx, t)

	// Record activity
	act := &TaskActivity{
		ID:           generateID("act"),
		TaskID:       taskID,
		AttemptID:    &newAttempt.ID,
		ActivityType: ActivityTypeRetried,
		Author:       author,
		NewStatus:    AttemptStatusQueued,
		Message:      fmt.Sprintf("Attempt %s dijadwalkan ulang secara eksplisit (attempt #%d) oleh %s.", newAttempt.ID, nextAttemptNum, author),
		CreatedAt:    now,
	}
	_ = s.store.CreateActivity(ctx, act)

	return newAttempt, nil
}

func (s *Service) AppendAttemptLogs(ctx context.Context, attemptID string, logs []LogRecord, maxLines int) error {
	return s.store.AppendLogs(ctx, attemptID, logs, maxLines)
}

func (s *Service) GetAttemptLogs(ctx context.Context, attemptID string, afterID int64, limit int) ([]*LogRecord, error) {
	return s.store.GetLogs(ctx, attemptID, afterID, limit)
}

// RecoverLocalAttempts exposes an unknown result after the local runner lease
// expires. It never launches the previous attempt again.
func (s *Service) RecoverLocalAttempts(ctx context.Context) error {
	tasks, err := s.store.ListTasks(ctx, TaskFilter{})
	if err != nil {
		return err
	}
	for _, t := range tasks {
		a, err := s.store.GetLatestAttempt(ctx, t.ID)
		if err != nil || a == nil || a.RunnerID != "local_terminal" || (a.Status != "running" && a.Status != "queued") {
			continue
		}
		if a.LeaseExpiresAt != nil && time.Now().Before(*a.LeaseExpiresAt) {
			continue
		}
		if _, err = s.UpdateAttemptStatus(ctx, a.ID, "recovery", UpdateAttemptRequest{Status: AttemptStatusInterrupted, ErrorMessage: "The local session was interrupted or its lease expired. The result is unknown. It will not be rerun automatically."}); err != nil {
			return err
		}
		if err = s.ReconcileLocalTask(ctx, t.ID, a.ID); err != nil {
			return err
		}
	}
	return nil
}
