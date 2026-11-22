package scheduler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ScheduledTask represents an automated routine defined by a user or agent.
type ScheduledTask struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Prompt       string       `json:"prompt"`
	ProjectID    string       `json:"project_id,omitempty"`
	ScheduleType ScheduleType `json:"schedule_type"`
	TimeOfDay    string       `json:"time_of_day,omitempty"` // e.g. "08:00"
	DayOfWeek    int          `json:"day_of_week,omitempty"` // 0=Sunday, 1=Monday... 6=Saturday
	IntervalMins int          `json:"interval_mins,omitempty"`
	Enabled      bool         `json:"enabled"`
	CreatedAt    time.Time    `json:"created_at"`
	NextRunAt    *time.Time   `json:"next_run_at,omitempty"`
	LastRunAt    *time.Time   `json:"last_run_at,omitempty"`
	LastStatus   JobStatus    `json:"last_status"`
	LastError    string       `json:"last_error,omitempty"`
}

// TaskExecutor is a callback invoked when a ScheduledTask runs.
type TaskExecutor func(ctx context.Context, task *ScheduledTask) error

// TaskManager coordinates user-defined persistent scheduled tasks.
type TaskManager struct {
	mu          sync.RWMutex
	storagePath string
	tasks       map[string]*ScheduledTask
	executor    TaskExecutor
}

// NewTaskManager creates a manager that loads and saves tasks from storagePath.
func NewTaskManager(storagePath string, executor ...TaskExecutor) *TaskManager {
	var exec TaskExecutor
	if len(executor) > 0 && executor[0] != nil {
		exec = executor[0]
	} else {
		exec = func(ctx context.Context, task *ScheduledTask) error {
			// default mock runner
			return nil
		}
	}

	tm := &TaskManager{
		storagePath: storagePath,
		tasks:       make(map[string]*ScheduledTask),
		executor:    exec,
	}

	tm.load()
	return tm
}

func (tm *TaskManager) load() {
	if tm.storagePath == "" {
		return
	}
	data, err := os.ReadFile(tm.storagePath)
	if err != nil {
		return
	}
	var list []*ScheduledTask
	if err := json.Unmarshal(data, &list); err == nil {
		for _, t := range list {
			tm.tasks[t.ID] = t
		}
	}
}

func (tm *TaskManager) save() error {
	if tm.storagePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(tm.storagePath), 0755); err != nil {
		return err
	}
	list := make([]*ScheduledTask, 0, len(tm.tasks))
	for _, t := range tm.tasks {
		list = append(list, t)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tm.storagePath, data, 0644)
}

// CreateTask registers a new routine and calculates its next run time.
func (tm *TaskManager) CreateTask(t *ScheduledTask) (*ScheduledTask, error) {
	if t == nil {
		return nil, errors.New("task cannot be nil")
	}
	title := strings.TrimSpace(t.Title)
	if title == "" {
		return nil, errors.New("title is required")
	}
	prompt := strings.TrimSpace(t.Prompt)
	if prompt == "" {
		return nil, errors.New("prompt instruction is required")
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	if t.ID == "" {
		bytes := make([]byte, 8)
		_, _ = rand.Read(bytes)
		t.ID = "sch_" + hex.EncodeToString(bytes)
	}

	t.Title = title
	t.Prompt = prompt
	if t.ScheduleType == "" {
		t.ScheduleType = ScheduleDaily
	}
	if t.TimeOfDay == "" {
		t.TimeOfDay = "08:00"
	}
	t.Enabled = true
	t.CreatedAt = time.Now().UTC()
	t.LastStatus = StatusIdle

	now := time.Now()
	next := calculateTaskNextRun(t, now)
	t.NextRunAt = &next

	tm.tasks[t.ID] = t
	_ = tm.save()

	return t, nil
}

// ListTasks returns all scheduled routines sorted by created time.
func (tm *TaskManager) ListTasks() []*ScheduledTask {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]*ScheduledTask, 0, len(tm.tasks))
	for _, t := range tm.tasks {
		cpy := *t
		result = append(result, &cpy)
	}
	return result
}

// GetTask returns a copy of a scheduled task by ID.
func (tm *TaskManager) GetTask(id string) (*ScheduledTask, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	t, exists := tm.tasks[id]
	if !exists {
		return nil, false
	}
	cpy := *t
	return &cpy, true
}

// DeleteTask removes a scheduled task.
func (tm *TaskManager) DeleteTask(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.tasks[id]; !exists {
		return errors.New("task not found")
	}
	delete(tm.tasks, id)
	return tm.save()
}

// ToggleTask pauses or resumes a scheduled routine.
func (tm *TaskManager) ToggleTask(id string, enabled *bool) (*ScheduledTask, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	t, exists := tm.tasks[id]
	if !exists {
		return nil, errors.New("task not found")
	}

	if enabled != nil {
		t.Enabled = *enabled
	} else {
		t.Enabled = !t.Enabled
	}

	if t.Enabled {
		now := time.Now()
		next := calculateTaskNextRun(t, now)
		t.NextRunAt = &next
	} else {
		t.NextRunAt = nil
	}

	_ = tm.save()
	cpy := *t
	return &cpy, nil
}

// UpdateTask modifies an existing scheduled routine.
func (tm *TaskManager) UpdateTask(id string, update *ScheduledTask) (*ScheduledTask, error) {
	if update == nil {
		return nil, errors.New("update cannot be nil")
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	t, exists := tm.tasks[id]
	if !exists {
		return nil, errors.New("task not found")
	}

	if title := strings.TrimSpace(update.Title); title != "" {
		t.Title = title
	}
	if prompt := strings.TrimSpace(update.Prompt); prompt != "" {
		t.Prompt = prompt
	}
	if update.ProjectID != "" {
		if update.ProjectID == "-" || update.ProjectID == "none" {
			t.ProjectID = ""
		} else {
			t.ProjectID = strings.TrimSpace(update.ProjectID)
		}
	}
	if update.ScheduleType != "" {
		t.ScheduleType = update.ScheduleType
	}
	if update.TimeOfDay != "" {
		t.TimeOfDay = strings.TrimSpace(update.TimeOfDay)
	}
	if update.DayOfWeek >= 0 && update.DayOfWeek <= 6 {
		t.DayOfWeek = update.DayOfWeek
	}
	if update.IntervalMins > 0 {
		t.IntervalMins = update.IntervalMins
	}

	if t.Enabled {
		now := time.Now()
		next := calculateTaskNextRun(t, now)
		t.NextRunAt = &next
	}

	if err := tm.save(); err != nil {
		return nil, err
	}

	cpy := *t
	return &cpy, nil
}

// TriggerTask executes a task immediately in the background.
func (tm *TaskManager) TriggerTask(ctx context.Context, id string) error {
	tm.mu.Lock()
	t, exists := tm.tasks[id]
	if !exists {
		tm.mu.Unlock()
		return errors.New("task not found")
	}
	if t.LastStatus == StatusRunning {
		tm.mu.Unlock()
		return errors.New("task is already running")
	}
	t.LastStatus = StatusRunning
	now := time.Now().UTC()
	t.LastRunAt = &now
	_ = tm.save()
	exec := tm.executor
	tm.mu.Unlock()

	go func() {
		err := exec(ctx, t)
		tm.mu.Lock()
		defer tm.mu.Unlock()
		if err != nil {
			t.LastStatus = StatusFailed
			t.LastError = err.Error()
		} else {
			t.LastStatus = StatusSuccess
			t.LastError = ""
		}
		_ = tm.save()
	}()

	return nil
}

func calculateTaskNextRun(t *ScheduledTask, ref time.Time) time.Time {
	switch t.ScheduleType {
	case ScheduleInterval:
		mins := t.IntervalMins
		if mins <= 0 {
			mins = 60
		}
		return ref.Add(time.Duration(mins) * time.Minute)

	case ScheduleDaily:
		h, m := parseTimeOfDay(t.TimeOfDay)
		target := time.Date(ref.Year(), ref.Month(), ref.Day(), h, m, 0, 0, ref.Location())
		if !target.After(ref) {
			target = target.AddDate(0, 0, 1)
		}
		return target

	case ScheduleWeekly:
		h, m := parseTimeOfDay(t.TimeOfDay)
		targetDay := t.DayOfWeek
		if targetDay < 0 || targetDay > 6 {
			targetDay = 1
		}
		currentDay := int(ref.Weekday())
		daysUntil := (targetDay - currentDay + 7) % 7
		target := time.Date(ref.Year(), ref.Month(), ref.Day(), h, m, 0, 0, ref.Location()).AddDate(0, 0, daysUntil)
		if !target.After(ref) {
			target = target.AddDate(0, 0, 7)
		}
		return target

	default:
		return ref.Add(24 * time.Hour)
	}
}
