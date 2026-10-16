package task

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/project"
)

type ProjectFinder interface {
	Get(ctx context.Context, id string) (*project.ProjectResponse, error)
}

type Service struct {
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
		{"T1", "Tetapkan fondasi monorepo dan kelayakan adapter agent", "Spike adapter agy, monorepo Go + React Vite TS, Apache-2.0", TaskStatusDone, PriorityHigh},
		{"T2", "Sediakan instalasi persisten dan akses owner", "Mode single-owner, bcrypt, proteksi takeover 403, PostgreSQL persistent", TaskStatusDone, PriorityHigh},
		{"T3", "Hubungkan repository GitHub existing", "Akses GitHub via PAT aman AES-256-GCM, validasi 404/401, non-destructive delete", TaskStatusDone, PriorityHigh},
		{"T4", "Bangun task board dan histori pekerjaan", "Task CRUD, pemisahan status task vs attempt, activity history, UI Kanban & detail", TaskStatusDoing, PriorityNormal},
		{"T5", "Jalankan task melalui runner dan satu agent", "Runner terautentikasi, lease/heartbeat, cancel/timeout, monitoring CPU/RAM", TaskStatusBacklog, PriorityHigh},
		{"T6", "Hasilkan perubahan Git dan PR yang dapat direview", "Workspace per task, targeted validation, commit/push, PR di repo Cuan", TaskStatusBacklog, PriorityHigh},
		{"T7", "Buktikan staging, update, backup, dan pemulihan", "Deploy berversi, staging terpisah, backup/restore, health check, rollback drill", TaskStatusBacklog, PriorityNormal},
		{"T8", "Demo dogfooding dan siapkan release OSS", "Develop Cuan using Cuan, demo dogfooding, OSS readiness checklist", TaskStatusBacklog, PriorityNormal},
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
			Message:      fmt.Sprintf("Task S1 '%s' diinisialisasi dalam status '%s'.", t.ID, t.Status),
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
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: judul task wajib diisi", ErrInvalidTaskInput)
	}

	priority := strings.ToLower(strings.TrimSpace(req.Priority))
	if priority == "" {
		priority = PriorityNormal
	}
	switch priority {
	case PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent:
	default:
		return nil, fmt.Errorf("%w: priority tidak valid ('%s')", ErrInvalidTaskInput, priority)
	}

	assignedAgent := strings.TrimSpace(req.AssignedAgent)
	if assignedAgent == "" {
		assignedAgent = "agy"
	}

	t := &Task{
		ID:            generateID("task"),
		ProjectID:     strings.TrimSpace(req.ProjectID),
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
		Message:      fmt.Sprintf("Task dibuat dengan prioritas '%s' dan agent '%s'.", t.Priority, t.AssignedAgent),
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

	return &TaskDetailResponse{
		Task:          *t,
		ProjectName:   projectName,
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

	if len(tasks) == 0 && filter.ProjectID == "" && filter.Status == "" && filter.Search == "" {
		_ = s.SeedSprint1Tasks(ctx)
		tasks, _ = s.store.ListTasks(ctx, filter)
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

		latestAttempt, _ := s.store.GetLatestAttempt(ctx, t.ID)
		items = append(items, &TaskItemResponse{
			Task:          *t,
			ProjectName:   pName,
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
			return nil, fmt.Errorf("%w: judul task tidak boleh kosong", ErrInvalidTaskInput)
		}
		t.Title = title
	}

	if req.Instructions != nil {
		t.Instructions = strings.TrimSpace(*req.Instructions)
	}

	if req.Priority != nil {
		p := strings.ToLower(strings.TrimSpace(*req.Priority))
		switch p {
		case PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent:
			t.Priority = p
		default:
			return nil, fmt.Errorf("%w: priority tidak valid ('%s')", ErrInvalidTaskInput, p)
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
		case TaskStatusBacklog, TaskStatusDoing, TaskStatusReview, TaskStatusDone, TaskStatusBlocked:
			if st != oldStatus {
				statusChanged = true
				newStatus = st
				t.Status = st
			}
		default:
			return nil, fmt.Errorf("%w: status task '%s' tidak valid", ErrInvalidTaskInput, st)
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
		return nil, fmt.Errorf("%w: pesan komentar tidak boleh kosong", ErrInvalidTaskInput)
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
	att, err := s.store.GetAttempt(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	oldStatus := att.Status
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
		return nil, fmt.Errorf("%w: status attempt '%s' tidak valid", ErrInvalidTaskInput, newStatus)
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
		return nil, fmt.Errorf("update attempt: %w", err)
	}

	// Audit activity log
	msg := fmt.Sprintf("Attempt %s berubah status: '%s' → '%s'", att.ID, oldStatus, newStatus)
	if att.ExitCode != nil {
		msg += fmt.Sprintf(" (exit code: %d)", *att.ExitCode)
	}
	if att.ErrorMessage != "" {
		msg += fmt.Sprintf(" [Pesan: %s]", att.ErrorMessage)
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
	t, err := s.store.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	latest, err := s.store.GetLatestAttempt(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if latest == nil {
		return nil, fmt.Errorf("%w: tidak ada attempt yang dapat dibatalkan", ErrAttemptNotFound)
	}

	if latest.Status != AttemptStatusRunning && latest.Status != AttemptStatusQueued {
		return nil, fmt.Errorf("%w: attempt berstatus '%s' tidak dapat dibatalkan", ErrInvalidTaskInput, latest.Status)
	}

	now := time.Now().UTC()
	latest.CancelRequested = true
	latest.Status = AttemptStatusCanceled
	latest.CanceledAt = &now
	latest.FinishedAt = &now
	exitCanceled := 130 // SIGINT exit code standard
	latest.ExitCode = &exitCanceled
	latest.ErrorMessage = fmt.Sprintf("Dibatalkan oleh %s", author)

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
		Message:      fmt.Sprintf("Pembatalan attempt %s diminta oleh %s. Seluruh process tree agent dihentikan.", latest.ID, author),
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
			return nil, fmt.Errorf("%w: task sedang berjalan atau sudah dalam antrean", ErrInvalidTaskInput)
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

