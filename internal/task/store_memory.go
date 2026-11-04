package task

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

type MemoryStore struct {
	launches         map[string]*TaskLaunch
	launchRecoveries map[string]bool
	mu               sync.RWMutex
	tasks            map[string]*Task
	attempts         map[string]*TaskAttempt
	activities       map[string]*TaskActivity
	logs             map[string][]*LogRecord
	nextLogID        int64
	taskProjects     map[string][]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		launches:         make(map[string]*TaskLaunch),
		launchRecoveries: make(map[string]bool),
		tasks:            make(map[string]*Task),
		attempts:         make(map[string]*TaskAttempt),
		activities:       make(map[string]*TaskActivity),
		logs:             make(map[string][]*LogRecord),
		nextLogID:        0,
		taskProjects:     make(map[string][]string),
	}
}

func (m *MemoryStore) CreateTask(ctx context.Context, t *Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[t.ID]; exists {
		return ErrTaskDuplicate
	}
	cp := *t
	cp.DoDChecklist = append([]DoDItem{}, t.DoDChecklist...)
	if cp.ExecutionCategory == "" {
		cp.ExecutionCategory = ExecutionCategoryCode
	}
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now().UTC()
	}
	cp.UpdatedAt = cp.CreatedAt

	pids := t.ProjectIDs
	if len(pids) == 0 && t.ProjectID != "" {
		pids = []string{t.ProjectID}
	}
	if len(pids) > 0 {
		m.taskProjects[cp.ID] = append([]string{}, pids...)
		cp.ProjectIDs = append([]string{}, pids...)
	}

	m.tasks[cp.ID] = &cp
	return nil
}

func (m *MemoryStore) GetTask(ctx context.Context, id string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, exists := m.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}
	cp := *t
	cp.DoDChecklist = append([]DoDItem{}, t.DoDChecklist...)
	if pids, ok := m.taskProjects[cp.ID]; ok && len(pids) > 0 {
		cp.ProjectIDs = append([]string{}, pids...)
	} else if cp.ProjectID != "" {
		cp.ProjectIDs = []string{cp.ProjectID}
	}
	return &cp, nil
}

func (m *MemoryStore) ListTasks(ctx context.Context, filter TaskFilter) ([]*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Task
	searchLower := strings.ToLower(strings.TrimSpace(filter.Search))

	for _, t := range m.tasks {
		if filter.IDs != nil && !oneOf(t.ID, filter.IDs...) {
			continue
		}
		if filter.SourceRoomID != "" && t.SourceRoomID != filter.SourceRoomID {
			continue
		}
		pids := m.taskProjects[t.ID]
		if len(pids) == 0 && t.ProjectID != "" {
			pids = []string{t.ProjectID}
		}

		if filter.ProjectID != "" {
			matched := t.ProjectID == filter.ProjectID
			if !matched {
				for _, pid := range pids {
					if pid == filter.ProjectID {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		if len(filter.ProjectIDs) > 0 {
			matched := false
			for _, reqPid := range filter.ProjectIDs {
				if t.ProjectID == reqPid {
					matched = true
					break
				}
				for _, pid := range pids {
					if pid == reqPid {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				continue
			}
		}

		if filter.Status != "" && t.Status != filter.Status {
			continue
		}
		if searchLower != "" {
			titleMatch := strings.Contains(strings.ToLower(t.Title), searchLower)
			instrMatch := strings.Contains(strings.ToLower(t.Instructions), searchLower)
			if !titleMatch && !instrMatch {
				continue
			}
		}
		cp := *t
		cp.DoDChecklist = append([]DoDItem{}, t.DoDChecklist...)
		cp.ProjectIDs = append([]string{}, pids...)
		result = append(result, &cp)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, nil
}

func (m *MemoryStore) UpdateTask(ctx context.Context, t *Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.tasks[t.ID]
	if !exists {
		return ErrTaskNotFound
	}

	cp := *t
	cp.DoDChecklist = append([]DoDItem{}, t.DoDChecklist...)
	cp.CreatedAt = existing.CreatedAt
	cp.UpdatedAt = time.Now().UTC()

	if t.ProjectIDs != nil {
		m.taskProjects[t.ID] = append([]string{}, t.ProjectIDs...)
		cp.ProjectIDs = append([]string{}, t.ProjectIDs...)
	} else if pids, ok := m.taskProjects[t.ID]; ok {
		cp.ProjectIDs = append([]string{}, pids...)
	}

	m.tasks[t.ID] = &cp
	return nil
}

func (m *MemoryStore) DeleteTask(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.tasks[id]; !exists {
		return ErrTaskNotFound
	}

	for _, a := range m.attempts {
		if a.TaskID == id && (a.Status == AttemptStatusRunning || a.Status == AttemptStatusQueued) {
			return ErrTaskActive
		}
	}
	delete(m.tasks, id)
	delete(m.taskProjects, id)

	for launchID, j := range m.launches {
		if j.TaskID == id {
			delete(m.launches, launchID)
			for key := range m.launchRecoveries {
				if strings.HasPrefix(key, launchID+"\x00") {
					delete(m.launchRecoveries, key)
				}
			}
		}
	}

	// Cascade delete attempts and activities
	for attID, att := range m.attempts {
		if att.TaskID == id {
			delete(m.attempts, attID)
		}
	}
	for actID, act := range m.activities {
		if act.TaskID == id {
			delete(m.activities, actID)
		}
	}

	return nil
}

func (m *MemoryStore) CreateAttempt(ctx context.Context, a *TaskAttempt) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.attempts[a.ID]; exists {
		return ErrTaskDuplicate
	}
	for _, j := range m.launches {
		if attemptRequestID(j.TaskID, j.RequestID) == a.ID {
			t := m.tasks[a.TaskID]
			if j.State != "claimed" || j.LeaseUntil == nil || !j.LeaseUntil.After(time.Now()) || t == nil || j.ProjectID != t.ProjectID || (j.AgentID != a.AgentName && t.AssignedAgent != a.AgentName) || j.InstructionsHash != LaunchInstructionsHash(t.Instructions) {
				return ErrInvalidTaskInput
			}
		}
	}
	if a.AttemptNumber <= 0 {
		a.AttemptNumber = 1
	}
	if a.RunnerID == "local_terminal" || a.RunnerID == "cuan-runner" {
		maxAttempt := 0
		for _, old := range m.attempts {
			if old.TaskID == a.TaskID && old.AttemptNumber > maxAttempt {
				maxAttempt = old.AttemptNumber
			}
			if old.TaskID == a.TaskID && (old.Status == "running" || old.Status == "queued") {
				return ErrTaskActive
			}
		}
		a.AttemptNumber = maxAttempt + 1
	}
	cp := *a
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now().UTC()
	}
	m.attempts[cp.ID] = &cp
	return nil
}

func (m *MemoryStore) GetAttempt(ctx context.Context, id string) (*TaskAttempt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	a, exists := m.attempts[id]
	if !exists {
		return nil, ErrAttemptNotFound
	}
	cp := *a
	return &cp, nil
}

func (m *MemoryStore) GetLatestAttempt(ctx context.Context, taskID string) (*TaskAttempt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var latest *TaskAttempt
	for _, a := range m.attempts {
		if a.TaskID == taskID {
			if latest == nil || a.CreatedAt.After(latest.CreatedAt) || (a.CreatedAt.Equal(latest.CreatedAt) && a.ID > latest.ID) {
				latest = a
			}
		}
	}

	if latest == nil {
		return nil, nil
	}
	cp := *latest
	return &cp, nil
}

func (m *MemoryStore) ListAttempts(ctx context.Context, taskID string) ([]*TaskAttempt, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*TaskAttempt
	for _, a := range m.attempts {
		if a.TaskID == taskID {
			cp := *a
			result = append(result, &cp)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, nil
}

func (m *MemoryStore) UpdateAttempt(ctx context.Context, a *TaskAttempt) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.attempts[a.ID]
	if !exists {
		return ErrAttemptNotFound
	}

	if existing.Status != "queued" && existing.Status != "running" && a.Status != existing.Status {
		return ErrAttemptFinal
	}
	cp := *a
	cp.CreatedAt = existing.CreatedAt
	m.attempts[a.ID] = &cp
	return nil
}

func (m *MemoryStore) CreateActivity(ctx context.Context, act *TaskActivity) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cp := *act
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now().UTC()
	}
	m.activities[cp.ID] = &cp
	return nil
}

func (m *MemoryStore) ListActivities(ctx context.Context, taskID string) ([]*TaskActivity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*TaskActivity
	for _, act := range m.activities {
		if act.TaskID == taskID {
			cp := *act
			result = append(result, &cp)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt) // Chronological order
	})

	return result, nil
}

func (m *MemoryStore) AppendLogs(ctx context.Context, attemptID string, logs []LogRecord, maxLines int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing := m.logs[attemptID]
	if existing == nil {
		existing = []*LogRecord{}
	}

	for _, entry := range logs {
		m.nextLogID++
		rec := &LogRecord{
			ID:        m.nextLogID,
			AttemptID: attemptID,
			Stream:    entry.Stream,
			Line:      boundedLogLine(entry.Line),
			LoggedAt:  entry.LoggedAt,
		}
		if rec.LoggedAt.IsZero() {
			rec.LoggedAt = time.Now().UTC()
		}
		existing = append(existing, rec)
	}

	if maxLines > 0 {
		start, size := len(existing), 0
		for start > 0 && len(existing)-start < maxLines {
			n := len(existing[start-1].Line) + 1
			if size+n > maxRetainedLogBytes {
				break
			}
			size += n
			start--
		}
		// Copy so discarded records are not held by the backing array.
		existing = append([]*LogRecord(nil), existing[start:]...)
	}

	m.logs[attemptID] = existing
	return nil
}

func (m *MemoryStore) GetRecentLogs(ctx context.Context, attemptID string, limit int) ([]*LogRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	all := m.logs[attemptID]
	if len(all) > limit {
		all = all[len(all)-limit:]
	}
	result := make([]*LogRecord, 0, len(all))
	for _, rec := range all {
		cp := *rec
		result = append(result, &cp)
	}
	return result, nil
}

func (m *MemoryStore) GetLogs(ctx context.Context, attemptID string, afterID int64, limit int) ([]*LogRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := m.logs[attemptID]
	if all == nil {
		return []*LogRecord{}, nil
	}

	if limit <= 0 || limit > 1000 {
		limit = 1000
	}

	var result []*LogRecord
	for _, rec := range all {
		if rec.ID > afterID {
			cp := *rec
			result = append(result, &cp)
			if len(result) >= limit {
				break
			}
		}
	}

	return result, nil
}

type TaskSnapshot struct {
	Tasks      []*Task
	Attempts   []*TaskAttempt
	Activities []*TaskActivity
}

func (m *MemoryStore) Snapshot() *TaskSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snap := &TaskSnapshot{}
	for _, t := range m.tasks {
		cp := *t
		cp.DoDChecklist = append([]DoDItem{}, t.DoDChecklist...)
		snap.Tasks = append(snap.Tasks, &cp)
	}
	for _, a := range m.attempts {
		cp := *a
		snap.Attempts = append(snap.Attempts, &cp)
	}
	for _, act := range m.activities {
		cp := *act
		snap.Activities = append(snap.Activities, &cp)
	}
	return snap
}

func (m *MemoryStore) Restore(snap *TaskSnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tasks = make(map[string]*Task)
	m.attempts = make(map[string]*TaskAttempt)
	m.activities = make(map[string]*TaskActivity)

	if snap == nil {
		return
	}

	for _, t := range snap.Tasks {
		cp := *t
		cp.DoDChecklist = append([]DoDItem{}, t.DoDChecklist...)
		m.tasks[cp.ID] = &cp
	}
	for _, a := range snap.Attempts {
		cp := *a
		m.attempts[cp.ID] = &cp
	}
	for _, act := range snap.Activities {
		cp := *act
		m.activities[act.ID] = &cp
	}
}

func (m *MemoryStore) Wipe() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tasks = make(map[string]*Task)
	m.attempts = make(map[string]*TaskAttempt)
	m.activities = make(map[string]*TaskActivity)
	m.logs = make(map[string][]*LogRecord)
}

func (m *MemoryStore) LatestProgresses(ctx context.Context, ids []string) (map[string]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := map[string]string{}
	latest := map[string]*TaskAttempt{}
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	for _, att := range m.attempts {
		if !idSet[att.TaskID] {
			continue
		}
		prev := latest[att.TaskID]
		if prev == nil || att.CreatedAt.After(prev.CreatedAt) || (att.CreatedAt.Equal(prev.CreatedAt) && att.ID > prev.ID) {
			latest[att.TaskID] = att
		}
	}
	progress := map[string]*TaskActivity{}
	for _, act := range m.activities {
		att := latest[act.TaskID]
		if att == nil || act.AttemptID == nil || *act.AttemptID != att.ID || act.ActivityType != "agent_report" || act.NewStatus != "progress" {
			continue
		}
		prev := progress[act.TaskID]
		if prev == nil || act.CreatedAt.After(prev.CreatedAt) || (act.CreatedAt.Equal(prev.CreatedAt) && act.ID > prev.ID) {
			progress[act.TaskID] = act
		}
	}
	for id, act := range progress {
		msg := strings.SplitN(act.Message, "\n", 2)[0]
		result[id] = CleanTerminalStatusLine(msg)
	}
	for _, id := range ids {
		att := latest[id]
		if result[id] != "" || att == nil || att.Status != AttemptStatusRunning {
			continue
		}
		var lines []string
		for _, rec := range m.logs[att.ID] {
			if rec.Stream == "stdout" {
				lines = append(lines, rec.Line)
			}
		}
		if len(lines) > 30 {
			lines = lines[len(lines)-30:]
		}
		result[id] = ExtractRunnerLiveActivity(lines)
	}
	return result, nil
}

func (m *MemoryStore) IsProjectActive(ctx context.Context, projectID string, excludeTaskID string) (bool, error) {
	return m.IsProjectLaneActive(ctx, projectID, ExecutionCategoryCode, excludeTaskID)
}

func (m *MemoryStore) IsProjectLaneActive(_ context.Context, projectID string, lane string, excludeTaskID string) (bool, error) {
	if projectID == "" {
		return false, nil
	}
	normalizedLane := NormalizeExecutionCategory(lane)
	if normalizedLane == "" {
		normalizedLane = ExecutionCategoryCode
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.tasks {
		if (t.ProjectID == projectID || containsString(t.ProjectIDs, projectID)) && (excludeTaskID == "" || t.ID != excludeTaskID) {
			if t.Status == TaskStatusDone || t.Status == TaskStatusBlocked || t.Status == TaskStatusReview || t.Status == "failed" || t.Status == "cancelled" || t.Status == TaskStatusQueued {
				continue
			}

			taskActive := false
			if t.Status == TaskStatusDoing || t.Status == "working" {
				taskActive = true
			}
			if !taskActive {
				for _, a := range m.attempts {
					if a.TaskID == t.ID && a.Status == AttemptStatusRunning {
						taskActive = true
						break
					}
				}
			}
			if !taskActive {
				for _, l := range m.launches {
					if l.TaskID == t.ID && l.State == "started" {
						taskActive = true
						break
					}
				}
			}
			if !taskActive {
				continue
			}

			// Active database schema migration blocks ALL lanes
			if IsTaskSchemaMigration(t) {
				return true, nil
			}

			if normalizedLane == ExecutionCategoryReadOnly {
				// Read-only lane only blocked by active migration
				continue
			}

			tLane := t.ExecutionCategory
			if tLane == "" {
				tLane = ExecutionCategoryCode
			}
			if tLane == normalizedLane {
				return true, nil
			}
		}
	}
	return false, nil
}

func containsString(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
