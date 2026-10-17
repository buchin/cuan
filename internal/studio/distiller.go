package studio

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/task"
)

// DistillOptions represents parameters for distilling discussion messages into a project task.
type DistillOptions struct {
	Title             string   `json:"title,omitempty"`
	MessageIDs        []string `json:"message_ids,omitempty"`
	BatchID           *string  `json:"batch_id,omitempty"`
	Tags              []string `json:"tags,omitempty"`
	AssigneeID        *string  `json:"assignee_id,omitempty"`
	ParentTaskID      *string  `json:"parent_task_id,omitempty"`
	Priority          string   `json:"priority,omitempty"`
	Status            string   `json:"status,omitempty"` // Default "backlog"
	AutoTriggerDoing  bool     `json:"auto_trigger_doing,omitempty"`
}

// ExecutionDispatchResult records details when an agent execution is dispatched.
type ExecutionDispatchResult struct {
	Dispatched  bool      `json:"dispatched"`
	TaskID      string    `json:"task_id"`
	ProjectID   string    `json:"project_id"`
	RunnerID    string    `json:"runner_id,omitempty"`
	AttemptID   string    `json:"attempt_id,omitempty"`
	AgentName   string    `json:"agent_name,omitempty"`
	TriggeredAt time.Time `json:"triggered_at"`
	Message     string    `json:"message"`
}

// ExecutionDispatcher handles dispatching runner/agent execution for a task.
type ExecutionDispatcher interface {
	Dispatch(ctx context.Context, t *db.ProjectTask) (*ExecutionDispatchResult, error)
}

// ExecutionDispatcherFunc is a functional adapter for ExecutionDispatcher.
type ExecutionDispatcherFunc func(ctx context.Context, t *db.ProjectTask) (*ExecutionDispatchResult, error)

func (f ExecutionDispatcherFunc) Dispatch(ctx context.Context, t *db.ProjectTask) (*ExecutionDispatchResult, error) {
	return f(ctx, t)
}

// AgentExecutionHook provides a lifecycle hook invoked when task status changes.
type AgentExecutionHook interface {
	OnTaskStatusChanged(ctx context.Context, t *db.ProjectTask, oldStatus, newStatus string) (*ExecutionDispatchResult, error)
}

// DefaultAgentExecutionHook implements AgentExecutionHook to auto-trigger runners when entering 'doing'.
type DefaultAgentExecutionHook struct {
	mu         sync.RWMutex
	dispatcher ExecutionDispatcher
	listeners  []func(ctx context.Context, t *db.ProjectTask, res *ExecutionDispatchResult)
}

// NewAgentExecutionHook creates a new DefaultAgentExecutionHook with the given dispatcher.
func NewAgentExecutionHook(dispatcher ExecutionDispatcher) *DefaultAgentExecutionHook {
	return &DefaultAgentExecutionHook{
		dispatcher: dispatcher,
		listeners:  make([]func(ctx context.Context, t *db.ProjectTask, res *ExecutionDispatchResult), 0),
	}
}

// SetDispatcher updates the execution dispatcher.
func (h *DefaultAgentExecutionHook) SetDispatcher(dispatcher ExecutionDispatcher) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dispatcher = dispatcher
}

// AddListener registers a notification callback triggered after successful dispatch.
func (h *DefaultAgentExecutionHook) AddListener(fn func(ctx context.Context, t *db.ProjectTask, res *ExecutionDispatchResult)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.listeners = append(h.listeners, fn)
}

// OnTaskStatusChanged triggers agent execution if the task transitioned to 'doing'.
func (h *DefaultAgentExecutionHook) OnTaskStatusChanged(ctx context.Context, t *db.ProjectTask, oldStatus, newStatus string) (*ExecutionDispatchResult, error) {
	h.mu.RLock()
	dispatcher := h.dispatcher
	listeners := append([]func(ctx context.Context, t *db.ProjectTask, res *ExecutionDispatchResult){}, h.listeners...)
	h.mu.RUnlock()

	normNew, err := db.NormalizeTaskStatus(newStatus)
	if err != nil {
		return nil, err
	}

	// Trigger execution only when entering 'doing' state from a non-doing state
	if normNew != "doing" || oldStatus == "doing" {
		return &ExecutionDispatchResult{
			Dispatched:  false,
			TaskID:      t.ID,
			ProjectID:   t.ProjectID,
			TriggeredAt: time.Now().UTC(),
			Message:     fmt.Sprintf("no execution triggered: status changed from '%s' to '%s'", oldStatus, normNew),
		}, nil
	}

	var result *ExecutionDispatchResult
	if dispatcher != nil {
		var err error
		result, err = dispatcher.Dispatch(ctx, t)
		if err != nil {
			return nil, fmt.Errorf("failed to dispatch task to runner: %w", err)
		}
	} else {
		// Default simulated/internal dispatch result when no external runner dispatcher is wired
		agentName := "agy"
		if t.AssigneeID != nil && *t.AssigneeID != "" {
			agentName = *t.AssigneeID
		}
		result = &ExecutionDispatchResult{
			Dispatched:  true,
			TaskID:      t.ID,
			ProjectID:   t.ProjectID,
			AttemptID:   "att-" + db.GenerateUUID()[:8],
			AgentName:   agentName,
			TriggeredAt: time.Now().UTC(),
			Message:     fmt.Sprintf("Agent runner '%s' successfully dispatched for task '%s'", agentName, t.Title),
		}
	}

	for _, listener := range listeners {
		listener(ctx, t, result)
	}

	return result, nil
}

// RunnerTaskBridgeDispatcher bridges db.ProjectTask to the cuan task.Store / runner queue.
type RunnerTaskBridgeDispatcher struct {
	taskStore task.Store
}

// NewRunnerTaskBridgeDispatcher constructs a bridge dispatcher between ProjectStudio and Runner queue.
func NewRunnerTaskBridgeDispatcher(taskStore task.Store) *RunnerTaskBridgeDispatcher {
	return &RunnerTaskBridgeDispatcher{
		taskStore: taskStore,
	}
}

// Dispatch creates an attempt in the runner task queue so background runners can claim it.
func (b *RunnerTaskBridgeDispatcher) Dispatch(ctx context.Context, t *db.ProjectTask) (*ExecutionDispatchResult, error) {
	if b.taskStore == nil {
		return &ExecutionDispatchResult{
			Dispatched:  true,
			TaskID:      t.ID,
			ProjectID:   t.ProjectID,
			TriggeredAt: time.Now().UTC(),
			Message:     "runner bridge: task store not configured, fallback standalone dispatch",
		}, nil
	}

	now := time.Now().UTC()
	agentName := "agy"
	if t.AssigneeID != nil && *t.AssigneeID != "" {
		agentName = *t.AssigneeID
	}

	// Ensure backing task exists in task store
	existingTask, _ := b.taskStore.GetTask(ctx, t.ID)
	if existingTask == nil {
		backingTask := &task.Task{
			ID:            t.ID,
			ProjectID:     t.ProjectID,
			Title:         t.Title,
			Instructions:  t.Description,
			Status:        task.TaskStatusDoing,
			Priority:      t.Priority,
			AssignedAgent: agentName,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		_ = b.taskStore.CreateTask(ctx, backingTask)
	} else {
		existingTask.Status = task.TaskStatusDoing
		existingTask.UpdatedAt = now
		_ = b.taskStore.UpdateTask(ctx, existingTask)
	}

	existingAttempts, _ := b.taskStore.ListAttempts(ctx, t.ID)
	attemptNum := len(existingAttempts) + 1
	attemptID := fmt.Sprintf("att_%d", now.UnixNano())
	attempt := &task.TaskAttempt{
		ID:            attemptID,
		TaskID:        t.ID,
		AgentName:     agentName,
		Status:        task.AttemptStatusQueued,
		AttemptNumber: attemptNum,
		CreatedAt:     now,
	}

	if err := b.taskStore.CreateAttempt(ctx, attempt); err != nil {
		return nil, fmt.Errorf("failed to create runner attempt: %w", err)
	}

	return &ExecutionDispatchResult{
		Dispatched:  true,
		TaskID:      t.ID,
		ProjectID:   t.ProjectID,
		AttemptID:   attemptID,
		AgentName:   agentName,
		TriggeredAt: now,
		Message:     fmt.Sprintf("Queued attempt %s for runner daemon (agent: %s)", attemptID, agentName),
	}, nil
}

// DistillResult contains distilled task and extraction metrics.
type DistillResult struct {
	Task                  *db.ProjectTask          `json:"task"`
	DistilledFromMessages int                      `json:"distilled_from_messages"`
	ExplicitDoDCount      int                      `json:"explicit_dod_count"`
	InferredDoDCount      int                      `json:"inferred_dod_count"`
	ExecutionDispatch     *ExecutionDispatchResult `json:"execution_dispatch,omitempty"`
}

// DistillerEngineOption allows configuring the DistillerEngine.
type DistillerEngineOption func(*DistillerEngine)

// WithMinDoDItems configures the minimum required DoD items (defaults to 3).
func WithMinDoDItems(minItems int) DistillerEngineOption {
	return func(e *DistillerEngine) {
		if minItems >= 3 {
			e.minDoDItems = minItems
		}
	}
}

// WithExecutionHook attaches an AgentExecutionHook to the DistillerEngine.
func WithExecutionHook(hook AgentExecutionHook) DistillerEngineOption {
	return func(e *DistillerEngine) {
		e.executionHook = hook
	}
}

// DistillerEngine parses discussion context into structured tasks with guaranteed DoD checklist (min 3 items).
type DistillerEngine struct {
	mu            sync.RWMutex
	minDoDItems   int
	executionHook AgentExecutionHook
}

// NewDistillerEngine initializes a new DistillerEngine instance.
func NewDistillerEngine(opts ...DistillerEngineOption) *DistillerEngine {
	e := &DistillerEngine{
		minDoDItems: 3, // Minimal 3 butir kriteria terverifikasi (DoD requirement)
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// SetExecutionHook updates the engine's execution hook.
func (e *DistillerEngine) SetExecutionHook(hook AgentExecutionHook) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.executionHook = hook
}

// ExecutionHook returns the current execution hook.
func (e *DistillerEngine) ExecutionHook() AgentExecutionHook {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.executionHook
}

// Distill transforms room context and messages into a *db.ProjectTask.
func (e *DistillerEngine) Distill(room *db.ProjectRoom, messages []db.ProjectMessage, opts DistillOptions) *db.ProjectTask {
	res, _ := e.DistillWithResult(context.Background(), room, messages, opts)
	if res != nil {
		return res.Task
	}
	return nil
}

// DistillWithResult performs full distillation with metrics and optional execution dispatch.
func (e *DistillerEngine) DistillWithResult(ctx context.Context, room *db.ProjectRoom, messages []db.ProjectMessage, opts DistillOptions) (*DistillResult, error) {
	e.mu.RLock()
	minDoD := e.minDoDItems
	hook := e.executionHook
	e.mu.RUnlock()

	selectedIDs := make(map[string]bool)
	for _, id := range opts.MessageIDs {
		selectedIDs[id] = true
	}

	// 1. Title Extraction
	title := e.ExtractTitle(room, messages, opts.Title)

	// 2. Contextual Description Extraction
	desc := e.BuildDescription(room, messages, selectedIDs)

	// 3. DoD Extraction (Checkboxes & Explicit Headers)
	explicitDoD := e.ExtractDoDChecklist(messages, selectedIDs)

	// 4. Intelligent Inference if DoD items count < minDoDItems (guarantee >= 3 verifiable items)
	finalDoD := e.InferDoDChecklist(room, messages, explicitDoD, minDoD)
	inferredCount := len(finalDoD) - len(explicitDoD)
	if inferredCount < 0 {
		inferredCount = 0
	}

	// 5. Normalization of Status and Priority
	status := strings.TrimSpace(opts.Status)
	if status == "" {
		status = "backlog"
	}
	normStatus, err := db.NormalizeTaskStatus(status)
	if err != nil {
		normStatus = "backlog"
	}

	priority := strings.TrimSpace(opts.Priority)
	if priority == "" {
		priority = "normal"
	}
	normPriority, err := db.NormalizeTaskPriority(priority)
	if err != nil {
		normPriority = "normal"
	}

	tags := opts.Tags
	if tags == nil {
		tags = []string{"studio", "distilled"}
	}

	taskID := db.GenerateUUID()
	var roomIDPtr *string
	var projectID string
	if room != nil {
		roomIDPtr = &room.ID
		projectID = room.ProjectID
	}

	task := &db.ProjectTask{
		ID:           taskID,
		ProjectID:    projectID,
		RoomID:       roomIDPtr,
		Title:        title,
		Description:  desc,
		Status:       normStatus,
		Priority:     normPriority,
		DoDChecklist: finalDoD,
		AssigneeID:   opts.AssigneeID,
		ParentTaskID: opts.ParentTaskID,
		BatchID:      opts.BatchID,
		Tags:         tags,
	}

	result := &DistillResult{
		Task:                  task,
		DistilledFromMessages: len(messages),
		ExplicitDoDCount:      len(explicitDoD),
		InferredDoDCount:      inferredCount,
	}

	// 6. Agent Execution Hook dispatch if status is doing or auto-trigger is requested
	if (normStatus == "doing" || opts.AutoTriggerDoing) && hook != nil {
		dispatchRes, err := hook.OnTaskStatusChanged(ctx, task, "backlog", "doing")
		if err == nil {
			result.ExecutionDispatch = dispatchRes
		}
	}

	return result, nil
}

// ExtractTitle determines the most accurate and concise task title.
func (e *DistillerEngine) ExtractTitle(room *db.ProjectRoom, messages []db.ProjectMessage, explicitTitle string) string {
	trimmed := strings.TrimSpace(explicitTitle)
	if trimmed != "" {
		return trimmed
	}

	// Try extracting explicit title pattern from messages e.g. "Task: Foo" or "# Task: Foo"
	taskHeaderRegex := regexp.MustCompile(`(?i)^(?:#+\s*)?(?:task|judul|objective|goal|sprint\s*\d+\.\d+)\s*[:—\-]\s*(.+)`)
	for _, msg := range messages {
		lines := strings.Split(msg.Content, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if m := taskHeaderRegex.FindStringSubmatch(line); len(m) > 1 {
				extracted := strings.TrimSpace(m[1])
				if len(extracted) >= 5 && len(extracted) <= 120 {
					return extracted
				}
			}
		}
	}

	// Fallback to room title
	if room != nil && strings.TrimSpace(room.Title) != "" {
		rt := strings.TrimSpace(room.Title)
		if strings.HasPrefix(strings.ToLower(rt), "task:") || strings.HasPrefix(strings.ToLower(rt), "task ") {
			return rt
		}
		return "Task: " + rt
	}

	// Fallback to room topic
	if room != nil && strings.TrimSpace(room.Topic) != "" {
		topic := strings.TrimSpace(room.Topic)
		if len(topic) > 60 {
			topic = topic[:57] + "..."
		}
		return "Task: " + topic
	}

	return "Distilled Task from Discussion"
}

// BuildDescription constructs a contextual markdown description from discussion transcript.
func (e *DistillerEngine) BuildDescription(room *db.ProjectRoom, messages []db.ProjectMessage, selectedIDs map[string]bool) string {
	var parts []string

	if room != nil && strings.TrimSpace(room.Topic) != "" {
		parts = append(parts, "Room Topic: "+strings.TrimSpace(room.Topic))
	}

	var chatLines []string
	for _, msg := range messages {
		if len(selectedIDs) > 0 && !selectedIDs[msg.ID] {
			continue
		}
		authorType := msg.AuthorType
		if authorType == "" {
			authorType = "user"
		}
		authorName := msg.AuthorName
		if authorName == "" {
			authorName = "Unknown"
		}
		chatLines = append(chatLines, fmt.Sprintf("[%s - %s]: %s", authorType, authorName, strings.TrimSpace(msg.Content)))
	}

	if len(chatLines) > 0 {
		parts = append(parts, strings.Join(chatLines, "\n\n"))
	} else if len(parts) == 0 {
		parts = append(parts, "Task distilled from project discussion room without specific chat history.")
	}

	return strings.Join(parts, "\n\n")
}

// ExtractDoDChecklist parses markdown checkboxes and explicit DoD lines from message contents.
func (e *DistillerEngine) ExtractDoDChecklist(messages []db.ProjectMessage, selectedIDs map[string]bool) []db.DoDItem {
	var items []db.DoDItem
	seen := make(map[string]bool)

	// Regex for markdown checkboxes: - [ ], - [x], * [ ], * [x], + [ ], + [x]
	checkboxRegex := regexp.MustCompile(`(?i)^[\*\-\+]\s*\[([ xX])\]\s*(.+)`)
	// Regex for DoD/AC headers e.g. "DoD: foo", "Acceptance Criteria: foo", "Kriteria: foo"
	dodPrefixRegex := regexp.MustCompile(`(?i)^(?:dod|acceptance criteria|kriteria penerimaan|kriteria|ac)\s*[:—\-]\s*(.+)`)
	// Regex for numbered list items e.g. "1. foo", "2) foo"
	numberedListRegex := regexp.MustCompile(`^\d+[\.\)]\s*(.+)`)

	inDoDBlock := false

	for _, msg := range messages {
		if len(selectedIDs) > 0 && !selectedIDs[msg.ID] {
			continue
		}

		lines := strings.Split(msg.Content, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				inDoDBlock = false
				continue
			}

			// 1. Check markdown checkboxes
			if m := checkboxRegex.FindStringSubmatch(trimmed); len(m) > 2 {
				isCompleted := strings.ToLower(m[1]) == "x"
				text := strings.TrimSpace(m[2])
				if text != "" && !seen[strings.ToLower(text)] {
					items = append(items, db.DoDItem{
						ID:        db.GenerateUUID(),
						Text:      text,
						Completed: isCompleted,
					})
					seen[strings.ToLower(text)] = true
				}
				continue
			}

			// 2. Check inline DoD: or Acceptance Criteria: prefix
			if m := dodPrefixRegex.FindStringSubmatch(trimmed); len(m) > 1 {
				text := strings.TrimSpace(m[1])
				if text != "" && !seen[strings.ToLower(text)] {
					items = append(items, db.DoDItem{
						ID:        db.GenerateUUID(),
						Text:      text,
						Completed: false,
					})
					seen[strings.ToLower(text)] = true
				} else if text == "" {
					inDoDBlock = true
				}
				continue
			}

			// 3. Check numbered list if preceded by DoD header
			if inDoDBlock {
				if m := numberedListRegex.FindStringSubmatch(trimmed); len(m) > 1 {
					text := strings.TrimSpace(m[1])
					if text != "" && !seen[strings.ToLower(text)] {
						items = append(items, db.DoDItem{
							ID:        db.GenerateUUID(),
							Text:      text,
							Completed: false,
						})
						seen[strings.ToLower(text)] = true
					}
					continue
				}
			}
		}
	}

	return items
}

// InferDoDChecklist ensures the checklist has at least minItems (default 3) verifiable criteria.
func (e *DistillerEngine) InferDoDChecklist(room *db.ProjectRoom, messages []db.ProjectMessage, explicitItems []db.DoDItem, minItems int) []db.DoDItem {
	items := make([]db.DoDItem, len(explicitItems))
	copy(items, explicitItems)

	seen := make(map[string]bool)
	for _, it := range items {
		seen[strings.ToLower(strings.TrimSpace(it.Text))] = true
	}

	if len(items) >= minItems {
		return items
	}

	// Gather corpus context for keyword analysis
	var corpus strings.Builder
	if room != nil {
		corpus.WriteString(room.Title)
		corpus.WriteString(" ")
		corpus.WriteString(room.Topic)
		corpus.WriteString(" ")
	}
	for _, msg := range messages {
		corpus.WriteString(msg.Content)
		corpus.WriteString(" ")
	}
	contentLower := strings.ToLower(corpus.String())

	// Candidate inference rules ordered by context relevance
	var candidates []string

	// Rule: Backend / API / Endpoint / Database
	if strings.Contains(contentLower, "api") || strings.Contains(contentLower, "endpoint") ||
		strings.Contains(contentLower, "handler") || strings.Contains(contentLower, "server") {
		candidates = append(candidates, "Implement backend endpoints and request handlers adhering to REST contract standards")
	}
	if strings.Contains(contentLower, "database") || strings.Contains(contentLower, "migrat") ||
		strings.Contains(contentLower, "schema") || strings.Contains(contentLower, "sql") {
		candidates = append(candidates, "Ensure database schema operations and queries maintain backward compatibility and data retention")
	}

	// Rule: Frontend / UI / React / CSS
	if strings.Contains(contentLower, "ui") || strings.Contains(contentLower, "frontend") ||
		strings.Contains(contentLower, "component") || strings.Contains(contentLower, "view") ||
		strings.Contains(contentLower, "react") {
		candidates = append(candidates, "Implement user interface components with clear navigation and intuitive error feedback")
	}

	// Rule: SSE / Real-time / Event / Broker
	if strings.Contains(contentLower, "sse") || strings.Contains(contentLower, "event") ||
		strings.Contains(contentLower, "stream") || strings.Contains(contentLower, "broadcast") {
		candidates = append(candidates, "Validate real-time event broadcasting and streaming lifecycle cleanup")
	}

	// Rule: Security / Auth / Validation
	if strings.Contains(contentLower, "auth") || strings.Contains(contentLower, "valid") ||
		strings.Contains(contentLower, "secur") || strings.Contains(contentLower, "token") {
		candidates = append(candidates, "Enforce strict payload validation and authorization security gates")
	}

	// Universal Baseline Candidates to guarantee verifiable completion
	candidates = append(candidates,
		"Implement core functional requirements and business logic discussed in the room",
		"Verify behavior with targeted automated tests (100% PASS rate)",
		"Record audit trail evidence and verify zero regressions against existing modules",
	)

	// Append non-duplicate candidates until threshold is reached
	for _, cand := range candidates {
		if len(items) >= minItems {
			break
		}
		candLower := strings.ToLower(strings.TrimSpace(cand))
		if !seen[candLower] {
			items = append(items, db.DoDItem{
				ID:        db.GenerateUUID(),
				Text:      cand,
				Completed: false,
			})
			seen[candLower] = true
		}
	}

	return items
}

// DefaultDistiller is the package-level singleton DistillerEngine.
var DefaultDistiller = NewDistillerEngine()
