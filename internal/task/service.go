package task

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	sdk "github.com/buchin/cuan/pkg/extensions"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/project"
)

type ProjectFinder interface {
	Get(ctx context.Context, id string) (*project.ProjectResponse, error)
}

type Service struct {
	extensions    *sdk.Registry
	taskMu        sync.Mutex
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

func (s *Service) Store() Store {
	return s.store
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
	s.taskMu.Lock()
	var createdEvent *Task
	defer func() {
		s.taskMu.Unlock()
		if createdEvent != nil {
			s.extensionEvent(ctx, "task.created", createdEvent)
		}
	}()
	id := generateID("task")
	if req.RequestID != "" {
		if !validRequestUUID(req.RequestID) {
			return nil, fmt.Errorf("%w: request_id must be a UUID", ErrInvalidTaskInput)
		}
		sum := sha256.Sum256([]byte(strings.ToLower(req.RequestID)))
		id = "task_" + hex.EncodeToString(sum[:16])
		if prior, err := s.store.GetTask(ctx, id); err == nil {
			if prior.CreationHash != taskCreationHash(req) {
				// Default is the new home for formerly unassigned work. Preserve
				// replay of a request accepted before that routing change.
				legacy := req
				legacy.ProjectID = ""
				legacy.ProjectIDs = nil
				if prior.ProjectID != "" || req.ProjectID != "default" || len(req.ProjectIDs) != 1 || req.ProjectIDs[0] != "default" || prior.CreationHash != taskCreationHash(legacy) {
					return nil, ErrRequestConflict
				}
			}
			return prior, nil
		} else if !errors.Is(err, ErrTaskNotFound) {
			return nil, err
		}
	}
	created, err := s.createTaskWithID(ctx, author, req, id)
	if err == nil {
		snapshot := *created
		createdEvent = &snapshot
	}
	return created, err
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
	if err == nil {
		s.extensionEvent(ctx, "task.created", result)
	}
	return result, err
}
func (s *Service) createTaskWithID(ctx context.Context, author string, req CreateTaskRequest, id string) (*Task, error) {
	title := strings.TrimSpace(req.Title)
	inferredTitle, inferredPriority := inferPrompt(req.Instructions)
	if title == "" {
		title = inferredTitle
	}
	if title == "" || len([]rune(title)) > 200 {
		return nil, fmt.Errorf("%w: task title must contain 1–200 characters", ErrInvalidTaskInput)
	}

	priority := strings.ToLower(strings.TrimSpace(req.Priority))
	if priority == "" {
		priority = inferredPriority
	}
	switch priority {
	case PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent:
	default:
		return nil, fmt.Errorf("%w: invalid priority ('%s')", ErrInvalidTaskInput, priority)
	}

	assignedAgent := strings.TrimSpace(req.AssignedAgent)
	if assignedAgent == "" {
		assignedAgent = ""
	}

	projectIDs := req.ProjectIDs
	projectID := strings.TrimSpace(req.ProjectID)
	if projectID == "" && len(projectIDs) > 0 {
		projectID = projectIDs[0]
	}
	if len(projectIDs) == 0 && projectID != "" {
		projectIDs = []string{projectID}
	}

	if len([]rune(req.Instructions)) > 24000 {
		return nil, fmt.Errorf("%w: instructions may contain up to 24000 characters", ErrInvalidTaskInput)
	}
	if err := validateChecklist(req.DoDChecklist); err != nil {
		return nil, err
	}
	initialStatus := TaskStatusBacklog
	if req.Status != "" {
		if req.Status != "backlog" && req.Status != "todo" && req.Status != TaskStatusQueued && req.Status != TaskStatusPending {
			return nil, fmt.Errorf("%w: new tasks must start in Backlog, Todo, or Queued", ErrInvalidTaskInput)
		}
		initialStatus = req.Status
	}
	instructions := strings.TrimSpace(req.Instructions)
	if s.projectFinder != nil && projectID != "" {
		p, err := s.projectFinder.Get(ctx, projectID)
		if err != nil || p == nil {
			return nil, fmt.Errorf("%w: project is unavailable", ErrInvalidTaskInput)
		}
		instructions = project.SnapshotProjectGuidance(instructions, p)
		if len([]rune(instructions)) > 24000 {
			return nil, fmt.Errorf("%w: instructions and project guidance exceed 24000 characters", ErrInvalidTaskInput)
		}
	}
	executionCategory := NormalizeExecutionCategory(req.ExecutionCategory)
	if executionCategory == "" {
		executionCategory = NormalizeExecutionCategory(req.TaskType)
	}
	if executionCategory == "" {
		executionCategory = NormalizeExecutionCategory(req.Category)
	}
	if executionCategory == "" {
		executionCategory = InferExecutionCategory(title, instructions)
	}
	t := &Task{
		CreationHash:      taskCreationHash(req),
		DoDChecklist:      append([]DoDItem{}, req.DoDChecklist...),
		ID:                id,
		SourceRoomID:      req.SourceRoomID,
		SourceMessageID:   req.SourceMessageID,
		ProjectID:         projectID,
		ProjectIDs:        projectIDs,
		Title:             title,
		Instructions:      instructions,
		Status:            initialStatus,
		Priority:          priority,
		ExecutionCategory: executionCategory,
		AssignedAgent:     assignedAgent,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
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
		Message:      fmt.Sprintf("Task created with priority '%s', agent '%s', category '%s'.", t.Priority, t.AssignedAgent, t.ExecutionCategory),
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

	latestProgress := ""
	// Detail and list share the same attempt-bound, bounded progress read model.
	if reader, ok := s.store.(interface {
		LatestProgresses(context.Context, []string) (map[string]string, error)
	}); ok {
		if progresses, err := reader.LatestProgresses(ctx, []string{id}); err == nil {
			latestProgress = progresses[id]
		}
	} else {
		var latestProgressTime time.Time
		for _, a := range activities {
			if latestAttempt != nil && a.AttemptID != nil && *a.AttemptID == latestAttempt.ID && a.ActivityType == "agent_report" && a.NewStatus == "progress" {
				if a.CreatedAt.After(latestProgressTime) {
					latestProgressTime = a.CreatedAt
					msg := a.Message
					if idx := strings.Index(msg, "\n"); idx != -1 {
						msg = msg[:idx]
					}
					cleaned := CleanTerminalStatusLine(msg)
					if cleaned != "" {
						latestProgress = cleaned
					}
				}
			}
		}
		if latestProgress == "" && latestAttempt != nil && latestAttempt.Status == AttemptStatusRunning {
			if logs, err := s.GetAttemptLogs(ctx, latestAttempt.ID, 0, 50); err == nil && len(logs) > 0 {
				lines := make([]string, len(logs))
				for i, l := range logs {
					lines[i] = l.Line
				}
				latestProgress = ExtractRunnerLiveActivity(lines)
			}
		}
	}

	return &TaskDetailResponse{
		ResultKind:     resultKindForAttempt(classifyResult(activities), latestAttempt),
		Launch:         s.launchInfo(ctx, id),
		Task:           *t,
		ProjectName:    projectName,
		ProjectNames:   projectNamesList,
		RepoURL:        repoURL,
		LatestAttempt:  latestAttempt,
		LatestProgress: latestProgress,
		Attempts:       attempts,
		Activities:     activities,
	}, nil
}

func (s *Service) ListTasks(ctx context.Context, filter TaskFilter) ([]*TaskItemResponse, error) {
	tasks, err := s.store.ListTasks(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	classifications := map[string]ResultClassification{}
	if reader, ok := s.store.(interface {
		LatestResultClassifications(context.Context, []string) (map[string]ResultClassification, error)
	}); ok {
		ids := make([]string, 0, len(tasks))
		for _, t := range tasks {
			ids = append(ids, t.ID)
		}
		classifications, err = reader.LatestResultClassifications(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("list task result classifications: %w", err)
		}
	}
	progresses := map[string]string{}
	if reader, ok := s.store.(interface {
		LatestProgresses(context.Context, []string) (map[string]string, error)
	}); ok {
		ids := make([]string, 0, len(tasks))
		for _, t := range tasks {
			ids = append(ids, t.ID)
		}
		progresses, _ = reader.LatestProgresses(ctx, ids)
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
			ResultKind:     resultKindForAttempt(classifications[t.ID], latestAttempt),
			Task:           *t,
			ProjectName:    pName,
			ProjectNames:   pNames,
			LatestAttempt:  latestAttempt,
			LatestProgress: progresses[t.ID],
		})
	}

	return items, nil
}

func (s *Service) UpdateTask(ctx context.Context, id string, author string, req UpdateTaskRequest) (*Task, error) {
	s.taskMu.Lock()
	var transitionEvent *Task
	var previousStatus string
	defer func() {
		s.taskMu.Unlock()
		if transitionEvent != nil {
			s.extensionTransition(ctx, previousStatus, transitionEvent)
		}
	}()
	t, err := s.store.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.ExpectedUpdatedAt != nil && !t.UpdatedAt.Equal(*req.ExpectedUpdatedAt) {
		return nil, ErrRequestConflict
	}

	statusChanged := false
	oldStatus := t.Status
	newStatus := oldStatus

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" || len([]rune(title)) > 200 {
			return nil, fmt.Errorf("%w: task title must contain 1–200 characters", ErrInvalidTaskInput)
		}
		t.Title = title
	}

	if req.Instructions != nil {
		if len([]rune(*req.Instructions)) > 24000 {
			return nil, fmt.Errorf("%w: instructions may contain up to 24000 characters", ErrInvalidTaskInput)
		}
		t.Instructions = project.PreserveProjectGuidance(t.Instructions, *req.Instructions)
		if len([]rune(t.Instructions)) > 24000 {
			return nil, fmt.Errorf("%w: instructions and saved project guidance exceed 24000 characters", ErrInvalidTaskInput)
		}
	}

	if req.Summary != nil {
		if len([]rune(*req.Summary)) > 12000 {
			return nil, fmt.Errorf("%w: summary may contain up to 12000 characters", ErrInvalidTaskInput)
		}
		t.Summary = strings.TrimSpace(*req.Summary)
	}
	if req.CompleteNote != nil {
		if len([]rune(*req.CompleteNote)) > 16000 {
			return nil, fmt.Errorf("%w: completion notes may contain up to 16000 characters", ErrInvalidTaskInput)
		}
		t.CompleteNote = strings.TrimSpace(*req.CompleteNote)
	}
	if req.DoDChecklist != nil {
		if err := validateChecklist(*req.DoDChecklist); err != nil {
			return nil, err
		}
		t.DoDChecklist = append([]DoDItem{}, (*req.DoDChecklist)...)
	}
	if req.ProjectIDs != nil {
		t.ProjectIDs = *req.ProjectIDs
		t.ProjectID = ""
		if len(t.ProjectIDs) > 0 {
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
		t.AssignedAgent = agent
	}

	if req.Status != nil {
		st := strings.ToLower(strings.TrimSpace(*req.Status))
		switch st {
		case TaskStatusBacklog, "todo", TaskStatusQueued, TaskStatusPending, TaskStatusDoing, TaskStatusReview, TaskStatusDone, TaskStatusBlocked:
			if st != oldStatus {
				latest, e := s.store.GetLatestAttempt(ctx, id)
				if e != nil && !errors.Is(e, ErrAttemptNotFound) {
					return nil, e
				}
				if latest != nil {
					if latest.Status == AttemptStatusRunning && st != TaskStatusDoing {
						return nil, ErrTaskActive
					}
					if latest.Status == AttemptStatusQueued && st != TaskStatusDoing && st != TaskStatusQueued {
						return nil, ErrTaskActive
					}
				}
				if st == TaskStatusDone {
					if oldStatus != TaskStatusReview {
						return nil, fmt.Errorf("%w: move the task to Review before approving it", ErrInvalidTaskInput)
					}
					if t.CompleteNote == "" {
						return nil, fmt.Errorf("%w: add completion notes and verification evidence before approval", ErrInvalidTaskInput)
					}
					for _, item := range t.DoDChecklist {
						if !item.Completed {
							return nil, fmt.Errorf("%w: verify every acceptance criterion before approval", ErrInvalidTaskInput)
						}
					}
					if latest != nil && latest.Status != AttemptStatusSucceeded {
						return nil, fmt.Errorf("%w: this execution has no successful result to approve", ErrInvalidTaskInput)
					}
				}
				statusChanged = true
				newStatus = st
				t.Status = st
			}
		default:
			return nil, fmt.Errorf("%w: invalid task status '%s'", ErrInvalidTaskInput, st)
		}
	}

	if req.ExecutionCategory != nil {
		cat := NormalizeExecutionCategory(*req.ExecutionCategory)
		if cat != "" {
			t.ExecutionCategory = cat
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

	if statusChanged {
		if newStatus == TaskStatusDone || newStatus == TaskStatusBlocked || newStatus == TaskStatusReview || newStatus == "cancelled" || newStatus == "failed" {
			_ = s.FinishTaskLaunches(ctx, t.ID, "stopped")
		}
		actMsg := fmt.Sprintf("Task status changed from '%s' to '%s'.", oldStatus, newStatus)
		if oldStatus == TaskStatusReview && newStatus == TaskStatusDoing {
			actMsg = "Task returned from Review to Doing. Execution is tracked separately; changing status does not launch an agent."
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
			Message:      "Task details updated by the owner.",
			CreatedAt:    t.UpdatedAt,
		}
		_ = s.store.CreateActivity(ctx, act)
	}

	snapshot := *t
	transitionEvent, previousStatus = &snapshot, oldStatus
	return t, nil
}

func (s *Service) DeleteTask(ctx context.Context, id string) error {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	latest, err := s.store.GetLatestAttempt(ctx, id)
	if err != nil && !errors.Is(err, ErrAttemptNotFound) {
		return err
	}
	if latest != nil && (latest.Status == AttemptStatusRunning || latest.Status == AttemptStatusQueued) {
		return ErrTaskActive
	}
	_ = s.FinishTaskLaunches(ctx, id, "stopped")
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
	s.taskMu.Lock()
	defer s.taskMu.Unlock()
	t, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	if req.RequestID != "" {
		if existing, err := s.FindRequestAttempt(ctx, taskID, req.RequestID, req.RequestHash); err != nil {
			return nil, err
		} else if existing != nil {
			return existing, nil
		}
	}
	if t.Status == TaskStatusDone {
		return nil, fmt.Errorf("%w: reopen the completed task before starting a new session", ErrInvalidTaskInput)
	}
	runnerID := strings.TrimSpace(req.RunnerID)
	if runnerID == "" {
		runnerID = "cuan-runner"
	}

	agentName := strings.TrimSpace(req.AgentName)
	if agentName == "" {
		agentName = t.AssignedAgent
		if agentName == "" {
			agentName = "agy"
		}
	}

	att := &TaskAttempt{
		ID:           attemptRequestID(t.ID, req.RequestID),
		RequestHash:  req.RequestHash,
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
		Message:      fmt.Sprintf("Attempt %s queued (agent: %s, runner: %s).", att.ID, att.AgentName, att.RunnerID),
		CreatedAt:    att.CreatedAt,
	}
	_ = s.store.CreateActivity(ctx, act)

	return att, nil
}

func (s *Service) UpdateAttemptStatus(ctx context.Context, attemptID string, author string, req UpdateAttemptRequest) (*TaskAttempt, error) {
	return s.updateAttemptStatus(ctx, attemptID, author, req, false)
}

// FinishReportedAttempt is an internal runner action, never a public status
// override. A validated final report intentionally closes an interactive PTY;
// its actual signal exit code is retained without claiming a provider failure.
func (s *Service) FinishReportedAttempt(ctx context.Context, attemptID string, exitCode int) (*TaskAttempt, error) {
	return s.updateAttemptStatus(ctx, attemptID, "runner", UpdateAttemptRequest{Status: AttemptStatusSucceeded, ExitCode: &exitCode}, true)
}

func (s *Service) updateAttemptStatus(ctx context.Context, attemptID string, author string, req UpdateAttemptRequest, reportedStop bool) (*TaskAttempt, error) {
	s.attemptMu.Lock()
	defer s.attemptMu.Unlock()
	att, err := s.store.GetAttempt(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	if reportedStop {
		latest, e := s.store.GetLatestAttempt(ctx, att.TaskID)
		if e != nil {
			return nil, e
		}
		t, e := s.store.GetTask(ctx, att.TaskID)
		if e != nil {
			return nil, e
		}
		acts, e := s.store.ListActivities(ctx, att.TaskID)
		if e != nil {
			return nil, e
		}
		var final *TaskActivity
		for _, a := range acts {
			if a.AttemptID != nil && *a.AttemptID == attemptID && a.ActivityType == "agent_report" && (final == nil || a.CreatedAt.After(final.CreatedAt)) {
				final = a
			}
		}
		if latest.ID != attemptID || final == nil || (final.NewStatus != "review" && final.NewStatus != "blocked" && final.NewStatus != "done") || t.Summary == "" || t.CompleteNote == "" {
			return nil, ErrRequestConflict
		}
	}
	oldStatus := att.Status
	if att.Status != "queued" && att.Status != "running" {
		return att, nil
	}
	newStatus := strings.ToLower(strings.TrimSpace(req.Status))

	// CRITICAL DoD Rule:
	// "Kegagalan attempt tidak ditampilkan sebagai sukses. Agent tidak otomatis menganggap hasil accepted hanya karena exit code nol."
	if req.ExitCode != nil && *req.ExitCode != 0 && !reportedStop {
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
		if t, e := s.store.GetTask(ctx, att.TaskID); e == nil {
			deadline := now.Add(CalculateTaskTimeout(t.ExecutionCategory, att.AttemptNumber, t.Title, t.Instructions))
			att.ExecutionDeadline = &deadline
		}
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

	if newStatus == AttemptStatusSucceeded || newStatus == AttemptStatusFailed || newStatus == AttemptStatusCanceled || newStatus == AttemptStatusInterrupted {
		_ = s.FinishTaskLaunches(ctx, att.TaskID, "stopped")
	}

	// Audit activity log
	msg := fmt.Sprintf("Attempt %s changed state: '%s' → '%s'", att.ID, oldStatus, newStatus)
	if reportedStop {
		msg += " (interactive session closed after validated agent report)"
	}
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
	var transitionEvent *Task
	var previousStatus string
	defer func() {
		s.attemptMu.Unlock()
		if transitionEvent != nil {
			s.extensionTransition(ctx, previousStatus, transitionEvent)
		}
	}()
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
	oldTaskStatus := t.Status
	t.Status = TaskStatusBlocked
	t.UpdatedAt = now
	_ = s.FinishTaskLaunches(ctx, taskID, "stopped")
	if err := s.store.UpdateTask(ctx, t); err == nil {
		snapshot := *t
		transitionEvent, previousStatus = &snapshot, oldTaskStatus
	}

	// Record activity
	act := &TaskActivity{
		ID:           generateID("act"),
		TaskID:       taskID,
		AttemptID:    &latest.ID,
		ActivityType: ActivityTypeCancelRequested,
		Author:       author,
		OldStatus:    oldTaskStatus,
		NewStatus:    TaskStatusBlocked,
		Message:      fmt.Sprintf("Cancellation of attempt %s requested by %s. Process termination has been requested.", latest.ID, author),
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
		attempts, _ := s.store.ListAttempts(ctx, taskID)
		businessCount := 0
		for _, a := range attempts {
			if a == nil || a.Status == AttemptStatusCanceled || a.CancelRequested || a.CanceledAt != nil {
				continue
			}
			if a.Status == AttemptStatusInterrupted {
				continue
			}
			var ec int
			if a.ExitCode != nil {
				ec = *a.ExitCode
			}
			if ec == 137 || ec == 143 {
				continue
			}
			errLower := strings.ToLower(a.ErrorMessage)
			if strings.Contains(errLower, "lease expired") || strings.Contains(errLower, "heartbeat lost") || strings.Contains(errLower, "runner killed") || strings.Contains(errLower, "runner restart") || strings.Contains(errLower, "connection lost") || strings.Contains(errLower, "interrupted") {
				continue
			}
			businessCount++
		}
		nextAttemptNum = businessCount + 1
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
		Message:      fmt.Sprintf("Attempt %s explicitly retried (attempt #%d) by %s.", newAttempt.ID, nextAttemptNum, author),
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

// RenewLocalAttemptLease keeps a known live local PTY valid without extending
// leases for a crashed process, another attempt or an already-final result.
func (s *Service) RenewLocalAttemptLease(ctx context.Context, attemptID string) error {
	s.attemptMu.Lock()
	defer s.attemptMu.Unlock()
	a, err := s.store.GetAttempt(ctx, attemptID)
	if err != nil {
		return err
	}
	if a.RunnerID != "local_terminal" || a.Status != AttemptStatusRunning {
		return nil
	}
	latest, err := s.store.GetLatestAttempt(ctx, a.TaskID)
	if err != nil {
		return err
	}
	if latest.ID != a.ID {
		return nil
	}
	now := time.Now().UTC()
	if a.LeaseExpiresAt != nil && a.LeaseExpiresAt.After(now.Add(10*time.Minute)) {
		return nil
	}
	lease := now.Add(11 * time.Minute)
	a.LeaseExpiresAt = &lease
	return s.store.UpdateAttempt(ctx, a)
}

// RecoverStartupLocalAttempts recovers any in-flight local_terminal attempts
// left behind by a server restart or crash without waiting for lease expiration,
// since local terminal PTY processes cannot survive across server restarts.
func (s *Service) RecoverStartupLocalAttempts(ctx context.Context) error {
	tasks, err := s.store.ListTasks(ctx, TaskFilter{})
	if err != nil {
		return err
	}
	for _, t := range tasks {
		a, err := s.store.GetLatestAttempt(ctx, t.ID)
		if err != nil || a == nil || a.RunnerID != "local_terminal" {
			continue
		}
		if a.Status == AttemptStatusRunning || a.Status == AttemptStatusQueued {
			if _, err = s.UpdateAttemptStatus(ctx, a.ID, "recovery", UpdateAttemptRequest{
				Status:       AttemptStatusInterrupted,
				ErrorMessage: "Execution was interrupted during server restart.",
			}); err != nil {
				return err
			}
			if err = s.ReconcileLocalTask(ctx, t.ID, a.ID); err != nil {
				return err
			}
		}
	}
	return nil
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
		if err != nil || a == nil || a.RunnerID != "local_terminal" {
			continue
		}
		if a.Status != "running" && a.Status != "queued" {
			if t.Status == TaskStatusDoing {
				if err = s.ReconcileLocalTask(ctx, t.ID, a.ID); err != nil {
					return err
				}
			}
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

// ReconcileZombieAttempts sweeps task attempts and launches whose OS process
// is dead or has exceeded the hard execution timeout. It marks them failed / stopped
// and frees the project lane so subsequent tasks can run automatically.
func (s *Service) ReconcileZombieAttempts(ctx context.Context, timeout time.Duration, isTaskActiveOnHost func(taskID string) bool) error {
	if timeout <= 0 {
		timeout = 20 * time.Minute
	}
	tasks, err := s.store.ListTasks(ctx, TaskFilter{})
	if err != nil {
		return err
	}
	now := time.Now().UTC()

	for _, t := range tasks {
		a, err := s.store.GetLatestAttempt(ctx, t.ID)
		if err != nil || a == nil {
			continue
		}

		if a.Status != "running" && a.Status != "queued" {
			_ = s.ReconcileLocalTask(ctx, t.ID, a.ID)
			continue
		}

		// Queue age is not execution time. Claiming the attempt assigns its
		// immutable execution deadline; queued work has no process to reap.
		if a.Status == AttemptStatusQueued {
			continue
		}
		deadline := AttemptDeadline(t, a)
		exceededTimeout := now.After(deadline.Add(GetGracePeriod()))

		// Check 2: OS process is dead (only for local_terminal attempts currently running;
		// decoupled runner daemon attempts are tracked via lease heartbeats, not local terminalManager)
		processDead := false
		if a.Status == "running" && (a.RunnerID == "local_terminal" || a.RunnerID == "") {
			if isTaskActiveOnHost != nil && !isTaskActiveOnHost(t.ID) {
				processDead = true
			}
		}

		if exceededTimeout || processDead {
			msg := "Task runner process terminated or inactive (zombie attempt reaped)."
			if exceededTimeout {
				msg = fmt.Sprintf("Task execution deadline exceeded (%s).", deadline.UTC().Format(time.RFC3339))
			}

			expected := t.UpdatedAt
			a.Status = AttemptStatusFailed
			a.ErrorMessage = msg
			a.FinishedAt = &now
			exitCode := 124
			a.ExitCode = &exitCode
			t.Status = TaskStatusBlocked
			t.UpdatedAt = now
			completion, ok := s.store.(CompletionStore)
			if !ok {
				continue
			}
			if err := completion.CompleteExecution(ctx, a, t, nil, expected, a.RunnerID, false); err != nil {
				continue
			}
			_ = s.store.CreateActivity(ctx, &TaskActivity{ID: generateID("act"), TaskID: t.ID, AttemptID: &a.ID, ActivityType: ActivityTypeAttempt, Author: "zombie_reaper", NewStatus: TaskStatusBlocked, Message: msg, CreatedAt: now})

			// Mark task launches stopped
			_ = s.FinishTaskLaunches(ctx, t.ID, "stopped")
		}
	}

	return nil
}

func validateChecklist(items []DoDItem) error {
	if len(items) > 50 {
		return fmt.Errorf("%w: use up to 50 acceptance criteria", ErrInvalidTaskInput)
	}
	for i := range items {
		items[i].Text = strings.TrimSpace(items[i].Text)
		if len([]rune(items[i].Evidence)) > 4000 {
			return fmt.Errorf("%w: criterion evidence must be at most 4000 characters", ErrInvalidTaskInput)
		}
		if items[i].Text == "" || len([]rune(items[i].Text)) > 500 {
			return fmt.Errorf("%w: acceptance criteria must contain 1–500 characters", ErrInvalidTaskInput)
		}
	}
	return nil
}
func attemptRequestID(taskID, requestID string) string {
	if requestID == "" {
		return generateID("att")
	}
	sum := sha256.Sum256([]byte(taskID + "\x00" + strings.ToLower(requestID)))
	return "att_" + hex.EncodeToString(sum[:16])
}

// FindRequestAttempt supports durable retries, including after a fast process exit
// or server restart. A replay returns recorded state and never launches work.
func (s *Service) FindRequestAttempt(ctx context.Context, taskID, requestID, hash string) (*TaskAttempt, error) {
	if requestID == "" {
		return nil, nil
	}
	if !validRequestUUID(requestID) {
		return nil, fmt.Errorf("%w: request_id must be a UUID", ErrInvalidTaskInput)
	}
	att, err := s.store.GetAttempt(ctx, attemptRequestID(taskID, requestID))
	if errors.Is(err, ErrAttemptNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if att.TaskID != taskID || att.RequestHash != hash {
		return nil, ErrRequestConflict
	}
	return att, nil
}

func validRequestUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	raw := strings.ReplaceAll(value, "-", "")
	if len(raw) != 32 {
		return false
	}
	_, err := hex.DecodeString(raw)
	return err == nil
}

func taskCreationHash(req CreateTaskRequest) string {
	req.RequestID = ""
	req.Title = strings.TrimSpace(req.Title)
	req.Instructions = strings.TrimSpace(req.Instructions)
	if req.Status == "" {
		req.Status = TaskStatusBacklog
	}
	if req.Priority == "" {
		req.Priority = PriorityNormal
	}
	data, _ := json.Marshal(req)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
