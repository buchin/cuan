package campaign

import (
	"context"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/db"
)

const (
	DefaultHeartbeatInterval = 15 * time.Second

	EventCampaignProgress  = "campaign.progress"
	EventTaskStatusChanged = "task.status_changed"
	EventCampaignCompleted = "campaign.completed"
)

// RollupMetrics represents the aggregated progress metrics for a batch campaign.
type RollupMetrics struct {
	CampaignID        string    `json:"campaign_id"`
	BatchID           string    `json:"batch_id,omitempty"`
	TotalTarget       int       `json:"total_target"`
	TotalTasks        int       `json:"total_tasks"`
	PendingTasks      int       `json:"pending_tasks"`
	RunningTasks      int       `json:"running_tasks"`
	CompletedTasks    int       `json:"completed_tasks"`
	ReviewTasks       int       `json:"review_tasks"`
	FailedTasks       int       `json:"failed_tasks"`
	SuccessPercentage float64   `json:"success_percentage"`
	Status            string    `json:"status"` // "draft", "pending", "running", "completed", "failed", "review"
	UpdatedAt         time.Time `json:"updated_at"`
}

// CampaignEvent represents an event payload sent over SSE streaming.
type CampaignEvent struct {
	Type       string      `json:"type"`
	CampaignID string      `json:"campaign_id"`
	Timestamp  time.Time   `json:"timestamp"`
	Data       interface{} `json:"data"`
}

// TaskStatusChangedData contains payload for 'task.status_changed' event.
type TaskStatusChangedData struct {
	CampaignID string    `json:"campaign_id"`
	TaskID     string    `json:"task_id"`
	ProjectID  string    `json:"project_id"`
	Title      string    `json:"title"`
	OldStatus  string    `json:"old_status,omitempty"`
	NewStatus  string    `json:"new_status"`
	Timestamp  time.Time `json:"timestamp"`
}

// CampaignCompletedData contains payload for 'campaign.completed' event.
type CampaignCompletedData struct {
	CampaignID string             `json:"campaign_id"`
	Status     string             `json:"status"`
	Rollup     *RollupMetrics     `json:"rollup"`
	Result     *CampaignRunResult `json:"result,omitempty"`
	Timestamp  time.Time          `json:"timestamp"`
}

// CampaignProgressData contains payload for 'campaign.progress' event.
type CampaignProgressData struct {
	CampaignID string         `json:"campaign_id"`
	Rollup     *RollupMetrics `json:"rollup"`
	Timestamp  time.Time      `json:"timestamp"`
}

// ClassifyTaskStatus standardizes any status string into one of the rollup buckets:
// "completed", "running", "review", "failed", or "pending".
func ClassifyTaskStatus(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "done", "completed", "success":
		return "completed"
	case "doing", "running", "in_progress":
		return "running"
	case "review", "in_review":
		return "review"
	case "failed", "blocked", "error":
		return "failed"
	case "backlog", "pending", "draft", "queued", "todo":
		return "pending"
	default:
		return "pending"
	}
}

// AggregatorOption configures RollupAggregator.
type AggregatorOption func(*RollupAggregator)

// WithAggregatorStudioRepo sets the project studio repository.
func WithAggregatorStudioRepo(repo db.ProjectStudioRepository) AggregatorOption {
	return func(a *RollupAggregator) {
		a.studioRepo = repo
	}
}

// WithAggregatorBatchRepo sets the batch campaign repository.
func WithAggregatorBatchRepo(repo db.BatchCampaignRepository) AggregatorOption {
	return func(a *RollupAggregator) {
		a.batchRepo = repo
	}
}

// WithAggregatorHeartbeat sets the SSE heartbeat interval.
func WithAggregatorHeartbeat(interval time.Duration) AggregatorOption {
	return func(a *RollupAggregator) {
		if interval > 0 {
			a.heartbeatInterval = interval
		}
	}
}

// RollupAggregator computes real-time aggregate campaign metrics and broadcasts SSE events.
type RollupAggregator struct {
	studioRepo        db.ProjectStudioRepository
	batchRepo         db.BatchCampaignRepository
	heartbeatInterval time.Duration

	mu          sync.RWMutex
	rollups     map[string]*RollupMetrics
	cachedTasks map[string]map[string]*db.ProjectTask // campaignID -> taskID -> task
	subscribers map[string][]chan CampaignEvent       // campaignID -> slice of client channels
}

// NewRollupAggregator creates a new RollupAggregator instance.
func NewRollupAggregator(opts ...AggregatorOption) *RollupAggregator {
	a := &RollupAggregator{
		heartbeatInterval: DefaultHeartbeatInterval,
		rollups:           make(map[string]*RollupMetrics),
		cachedTasks:       make(map[string]map[string]*db.ProjectTask),
		subscribers:       make(map[string][]chan CampaignEvent),
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

// HeartbeatInterval returns the configured SSE heartbeat interval.
func (a *RollupAggregator) HeartbeatInterval() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.heartbeatInterval
}

// SetHeartbeatInterval overrides the SSE heartbeat interval (useful for targeted tests).
func (a *RollupAggregator) SetHeartbeatInterval(d time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if d > 0 {
		a.heartbeatInterval = d
	}
}

// CalculateRollup computes RollupMetrics from a slice of subtasks.
// If subtasks include both parent epics and child tasks, it filters out parent epics
// to ensure accurate target metric calculation.
func (a *RollupAggregator) CalculateRollup(tasks []*db.ProjectTask, campaignID ...string) *RollupMetrics {
	cID := ""
	if len(campaignID) > 0 {
		cID = campaignID[0]
	}

	m := &RollupMetrics{
		CampaignID: cID,
		UpdatedAt:  time.Now().UTC(),
	}

	// Detect if child subtasks exist
	hasChildren := false
	for _, t := range tasks {
		if t.ParentTaskID != nil && *t.ParentTaskID != "" {
			hasChildren = true
			break
		}
	}

	var targetTasks []*db.ProjectTask
	for _, t := range tasks {
		if hasChildren && (t.ParentTaskID == nil || *t.ParentTaskID == "") {
			// Skip parent epic when counting target subtasks
			continue
		}
		targetTasks = append(targetTasks, t)
	}

	m.TotalTasks = len(targetTasks)
	m.TotalTarget = m.TotalTasks

	if m.CampaignID == "" && len(targetTasks) > 0 && targetTasks[0].BatchID != nil {
		m.CampaignID = *targetTasks[0].BatchID
		m.BatchID = *targetTasks[0].BatchID
	}

	for _, t := range targetTasks {
		switch ClassifyTaskStatus(t.Status) {
		case "completed":
			m.CompletedTasks++
		case "running":
			m.RunningTasks++
		case "review":
			m.ReviewTasks++
		case "failed":
			m.FailedTasks++
		default:
			m.PendingTasks++
		}
	}

	if m.TotalTasks > 0 {
		pct := (float64(m.CompletedTasks) / float64(m.TotalTasks)) * 100.0
		m.SuccessPercentage = math.Round(pct*100) / 100
	} else {
		m.SuccessPercentage = 0.0
	}

	// Overall status resolution
	if m.TotalTasks == 0 {
		m.Status = "draft"
	} else if m.CompletedTasks == m.TotalTasks {
		m.Status = "completed"
	} else if m.FailedTasks == m.TotalTasks {
		m.Status = "failed"
	} else if m.FailedTasks > 0 && (m.CompletedTasks+m.FailedTasks) == m.TotalTasks {
		m.Status = "failed"
	} else if m.RunningTasks > 0 || m.CompletedTasks > 0 {
		m.Status = "running"
	} else if m.ReviewTasks > 0 {
		m.Status = "review"
	} else {
		m.Status = "pending"
	}

	return m
}

// GetRollup returns the cached rollup metrics for a campaign ID, or nil if not yet computed.
func (a *RollupAggregator) GetRollup(campaignID string) *RollupMetrics {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.rollups[campaignID]
}

// UpdateRollupInMemory updates in-memory cached subtasks and recomputes the rollup metrics.
func (a *RollupAggregator) UpdateRollupInMemory(campaignID string, tasks []*db.ProjectTask) *RollupMetrics {
	a.mu.Lock()
	if a.cachedTasks[campaignID] == nil {
		a.cachedTasks[campaignID] = make(map[string]*db.ProjectTask)
	}

	for _, t := range tasks {
		taskCopy := *t
		a.cachedTasks[campaignID][t.ID] = &taskCopy
	}

	var taskList []*db.ProjectTask
	for _, t := range a.cachedTasks[campaignID] {
		taskList = append(taskList, t)
	}
	metrics := a.CalculateRollup(taskList, campaignID)
	a.rollups[campaignID] = metrics
	a.mu.Unlock()

	a.Broadcast(campaignID, EventCampaignProgress, CampaignProgressData{
		CampaignID: campaignID,
		Rollup:     metrics,
		Timestamp:  time.Now().UTC(),
	})

	return metrics
}

// RefreshFromRepository loads child subtasks for a campaign from db.ProjectStudioRepository and updates the rollup.
func (a *RollupAggregator) RefreshFromRepository(ctx context.Context, campaignID string) (*RollupMetrics, error) {
	if a.studioRepo == nil {
		return a.GetRollup(campaignID), nil
	}

	tasks, err := a.studioRepo.ListTasksByBatchID(ctx, campaignID)
	if err != nil || len(tasks) == 0 {
		tasksByParent, errParent := a.studioRepo.ListTasksByParentID(ctx, campaignID)
		if errParent == nil && len(tasksByParent) > 0 {
			tasks = tasksByParent
		}
	}

	taskPtrs := make([]*db.ProjectTask, len(tasks))
	for i := range tasks {
		taskPtrs[i] = &tasks[i]
	}

	metrics := a.UpdateRollupInMemory(campaignID, taskPtrs)
	return metrics, nil
}

// Subscribe registers a new SSE subscriber channel for the given campaign ID.
func (a *RollupAggregator) Subscribe(campaignID string) chan CampaignEvent {
	ch := make(chan CampaignEvent, 64)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.subscribers[campaignID] = append(a.subscribers[campaignID], ch)
	return ch
}

// Unsubscribe removes a client channel and cleans up memory without leaks.
func (a *RollupAggregator) Unsubscribe(campaignID string, ch chan CampaignEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	subs := a.subscribers[campaignID]
	for i, sub := range subs {
		if sub == ch {
			a.subscribers[campaignID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}

	if len(a.subscribers[campaignID]) == 0 {
		delete(a.subscribers, campaignID)
	}
}

// SubscriberCount returns the current number of active listeners for a campaign.
func (a *RollupAggregator) SubscriberCount(campaignID string) int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.subscribers[campaignID])
}

// Broadcast dispatches an event to all active subscribers of a campaign non-blockingly.
func (a *RollupAggregator) Broadcast(campaignID string, evType string, data interface{}) {
	ev := CampaignEvent{
		Type:       evType,
		CampaignID: campaignID,
		Timestamp:  time.Now().UTC(),
		Data:       data,
	}

	a.mu.RLock()
	subs := append([]chan CampaignEvent{}, a.subscribers[campaignID]...)
	a.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
			// Non-blocking drop on full subscriber buffer to preserve throughput
		}
	}
}

func (a *RollupAggregator) resolveCampaignID(task *db.ProjectTask) string {
	if task.BatchID != nil && *task.BatchID != "" {
		return *task.BatchID
	}
	if task.ParentTaskID != nil && *task.ParentTaskID != "" {
		return *task.ParentTaskID
	}

	a.mu.RLock()
	defer a.mu.RUnlock()
	for cID, taskMap := range a.cachedTasks {
		if _, exists := taskMap[task.ID]; exists {
			return cID
		}
	}
	return ""
}

// OnCampaignStart implements CampaignStartListener to register tasks when runner begins.
func (a *RollupAggregator) OnCampaignStart(campaignID string, tasks []*db.ProjectTask) {
	a.UpdateRollupInMemory(campaignID, tasks)
}

// OnTaskStart implements RunnerListener and notifies when a subtask transitions to running.
func (a *RollupAggregator) OnTaskStart(task *db.ProjectTask) {
	if task == nil {
		return
	}
	cID := a.resolveCampaignID(task)
	if cID == "" {
		return
	}

	oldStatus := task.Status
	newStatus := "doing"

	a.mu.Lock()
	if a.cachedTasks[cID] == nil {
		a.cachedTasks[cID] = make(map[string]*db.ProjectTask)
	}
	taskCopy := *task
	taskCopy.Status = newStatus
	a.cachedTasks[cID][task.ID] = &taskCopy

	var taskList []*db.ProjectTask
	for _, t := range a.cachedTasks[cID] {
		taskList = append(taskList, t)
	}
	metrics := a.CalculateRollup(taskList, cID)
	a.rollups[cID] = metrics
	a.mu.Unlock()

	a.Broadcast(cID, EventTaskStatusChanged, TaskStatusChangedData{
		CampaignID: cID,
		TaskID:     task.ID,
		ProjectID:  task.ProjectID,
		Title:      task.Title,
		OldStatus:  oldStatus,
		NewStatus:  newStatus,
		Timestamp:  time.Now().UTC(),
	})

	a.Broadcast(cID, EventCampaignProgress, CampaignProgressData{
		CampaignID: cID,
		Rollup:     metrics,
		Timestamp:  time.Now().UTC(),
	})
}

// OnTaskComplete implements RunnerListener and notifies when a subtask finishes successfully.
func (a *RollupAggregator) OnTaskComplete(task *db.ProjectTask, res *TaskExecutionResult) {
	if task == nil {
		return
	}
	cID := a.resolveCampaignID(task)
	if cID == "" {
		return
	}

	oldStatus := task.Status
	newStatus := "done"

	a.mu.Lock()
	if a.cachedTasks[cID] == nil {
		a.cachedTasks[cID] = make(map[string]*db.ProjectTask)
	}
	taskCopy := *task
	taskCopy.Status = newStatus
	a.cachedTasks[cID][task.ID] = &taskCopy

	var taskList []*db.ProjectTask
	for _, t := range a.cachedTasks[cID] {
		taskList = append(taskList, t)
	}
	metrics := a.CalculateRollup(taskList, cID)
	a.rollups[cID] = metrics
	a.mu.Unlock()

	a.Broadcast(cID, EventTaskStatusChanged, TaskStatusChangedData{
		CampaignID: cID,
		TaskID:     task.ID,
		ProjectID:  task.ProjectID,
		Title:      task.Title,
		OldStatus:  oldStatus,
		NewStatus:  newStatus,
		Timestamp:  time.Now().UTC(),
	})

	a.Broadcast(cID, EventCampaignProgress, CampaignProgressData{
		CampaignID: cID,
		Rollup:     metrics,
		Timestamp:  time.Now().UTC(),
	})
}

// OnTaskFail implements RunnerListener and notifies when a subtask fails or panics.
func (a *RollupAggregator) OnTaskFail(task *db.ProjectTask, err error) {
	if task == nil {
		return
	}
	cID := a.resolveCampaignID(task)
	if cID == "" {
		return
	}

	oldStatus := task.Status
	newStatus := "failed"

	a.mu.Lock()
	if a.cachedTasks[cID] == nil {
		a.cachedTasks[cID] = make(map[string]*db.ProjectTask)
	}
	taskCopy := *task
	taskCopy.Status = newStatus
	a.cachedTasks[cID][task.ID] = &taskCopy

	var taskList []*db.ProjectTask
	for _, t := range a.cachedTasks[cID] {
		taskList = append(taskList, t)
	}
	metrics := a.CalculateRollup(taskList, cID)
	a.rollups[cID] = metrics
	a.mu.Unlock()

	a.Broadcast(cID, EventTaskStatusChanged, TaskStatusChangedData{
		CampaignID: cID,
		TaskID:     task.ID,
		ProjectID:  task.ProjectID,
		Title:      task.Title,
		OldStatus:  oldStatus,
		NewStatus:  newStatus,
		Timestamp:  time.Now().UTC(),
	})

	a.Broadcast(cID, EventCampaignProgress, CampaignProgressData{
		CampaignID: cID,
		Rollup:     metrics,
		Timestamp:  time.Now().UTC(),
	})
}

// OnCampaignComplete implements RunnerListener and notifies when the full batch run finishes.
func (a *RollupAggregator) OnCampaignComplete(campaignID string, result *CampaignRunResult) {
	a.mu.Lock()
	metrics := a.rollups[campaignID]
	if metrics == nil {
		var taskList []*db.ProjectTask
		if a.cachedTasks[campaignID] != nil {
			for _, t := range a.cachedTasks[campaignID] {
				taskList = append(taskList, t)
			}
		}
		metrics = a.CalculateRollup(taskList, campaignID)
		a.rollups[campaignID] = metrics
	}

	status := metrics.Status
	if result != nil {
		if result.Success {
			status = "completed"
		} else if result.FailedTasks > 0 && result.CompletedTasks == 0 {
			status = "failed"
		}
	}
	metrics.Status = status
	metrics.UpdatedAt = time.Now().UTC()
	a.mu.Unlock()

	a.Broadcast(campaignID, EventCampaignCompleted, CampaignCompletedData{
		CampaignID: campaignID,
		Status:     status,
		Rollup:     metrics,
		Result:     result,
		Timestamp:  time.Now().UTC(),
	})

	a.Broadcast(campaignID, EventCampaignProgress, CampaignProgressData{
		CampaignID: campaignID,
		Rollup:     metrics,
		Timestamp:  time.Now().UTC(),
	})
}
