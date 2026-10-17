package campaign

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/buchin/cuan/internal/db"
)

const (
	DefaultMaxWorkers = 5
)

// TaskExecutionResult records the detailed execution outcome for a single project task.
type TaskExecutionResult struct {
	TaskID     string    `json:"task_id"`
	ProjectID  string    `json:"project_id"`
	Status     string    `json:"status"` // "completed", "failed"
	Output     string    `json:"output,omitempty"`
	Error      string    `json:"error,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	DurationMs int64     `json:"duration_ms"`
}

// CampaignRunResult aggregates execution metrics and per-task outcomes for a batch campaign.
type CampaignRunResult struct {
	CampaignID      string                 `json:"campaign_id"`
	BatchID         string                 `json:"batch_id"`
	TotalTasks      int                    `json:"total_tasks"`
	CompletedTasks  int                    `json:"completed_tasks"`
	FailedTasks     int                    `json:"failed_tasks"`
	RunningTasks    int                    `json:"running_tasks"`
	PeakConcurrency int                    `json:"peak_concurrency"`
	Results         []*TaskExecutionResult `json:"results"`
	StartedAt       time.Time              `json:"started_at"`
	FinishedAt      time.Time              `json:"finished_at"`
	DurationMs      int64                  `json:"duration_ms"`
	Success         bool                   `json:"success"`
}

// TaskExecutor executes work on a single ProjectTask.
type TaskExecutor interface {
	ExecuteTask(ctx context.Context, task *db.ProjectTask) (*TaskExecutionResult, error)
}

// TaskExecutorFunc adapts a plain function to TaskExecutor.
type TaskExecutorFunc func(ctx context.Context, task *db.ProjectTask) (*TaskExecutionResult, error)

func (f TaskExecutorFunc) ExecuteTask(ctx context.Context, task *db.ProjectTask) (*TaskExecutionResult, error) {
	return f(ctx, task)
}

// DefaultTaskExecutor simulates or triggers execution of a project task by completing its DoD checklist.
type DefaultTaskExecutor struct {
	studioRepo db.ProjectStudioRepository
}

// NewDefaultTaskExecutor creates a DefaultTaskExecutor.
func NewDefaultTaskExecutor(studioRepo db.ProjectStudioRepository) *DefaultTaskExecutor {
	return &DefaultTaskExecutor{studioRepo: studioRepo}
}

// ExecuteTask updates status to doing, marks DoD checklist completed, and transitions to done.
func (e *DefaultTaskExecutor) ExecuteTask(ctx context.Context, t *db.ProjectTask) (*TaskExecutionResult, error) {
	start := time.Now().UTC()

	// 1. Transition to 'doing'
	if e.studioRepo != nil {
		_ = e.studioRepo.UpdateTaskStatus(ctx, t.ID, "doing")
	}
	t.Status = "doing"

	// 2. Mark all DoD items completed
	for i := range t.DoDChecklist {
		t.DoDChecklist[i].Completed = true
	}

	// 3. Transition to 'done'
	if e.studioRepo != nil {
		_ = e.studioRepo.UpdateTask(ctx, t)
		_ = e.studioRepo.UpdateTaskStatus(ctx, t.ID, "done")
	}
	t.Status = "done"

	finish := time.Now().UTC()
	return &TaskExecutionResult{
		TaskID:     t.ID,
		ProjectID:  t.ProjectID,
		Status:     "completed",
		Output:     fmt.Sprintf("Successfully executed task '%s' for project '%s'", t.Title, t.ProjectID),
		StartedAt:  start,
		FinishedAt: finish,
		DurationMs: finish.Sub(start).Milliseconds(),
	}, nil
}

// RunnerListener receives notifications during campaign and task execution lifecycles.
type RunnerListener interface {
	OnTaskStart(task *db.ProjectTask)
	OnTaskComplete(task *db.ProjectTask, res *TaskExecutionResult)
	OnTaskFail(task *db.ProjectTask, err error)
	OnCampaignComplete(campaignID string, result *CampaignRunResult)
}

// RunnerOption configures the MultiProjectRunner.
type RunnerOption func(*MultiProjectRunner)

// WithMaxWorkers sets the maximum parallel worker concurrency limit (default: 5).
func WithMaxWorkers(n int) RunnerOption {
	return func(r *MultiProjectRunner) {
		if n > 0 {
			r.maxWorkers = n
		}
	}
}

// WithTaskExecutor injects a custom TaskExecutor implementation.
func WithTaskExecutor(executor TaskExecutor) RunnerOption {
	return func(r *MultiProjectRunner) {
		r.executor = executor
	}
}

// WithRunnerStudioRepo sets the project studio repository.
func WithRunnerStudioRepo(repo db.ProjectStudioRepository) RunnerOption {
	return func(r *MultiProjectRunner) {
		r.studioRepo = repo
	}
}

// WithRunnerBatchRepo sets the batch campaign repository.
func WithRunnerBatchRepo(repo db.BatchCampaignRepository) RunnerOption {
	return func(r *MultiProjectRunner) {
		r.batchRepo = repo
	}
}

// WithDecompiler sets the decompiler engine for automatic decompile & execute.
func WithDecompiler(decompiler *DecompilerEngine) RunnerOption {
	return func(r *MultiProjectRunner) {
		r.decompiler = decompiler
	}
}

// MultiProjectRunner executes tasks across multiple projects in isolated parallel workers.
type MultiProjectRunner struct {
	maxWorkers int
	executor   TaskExecutor
	studioRepo db.ProjectStudioRepository
	batchRepo  db.BatchCampaignRepository
	decompiler *DecompilerEngine
	listeners  []RunnerListener

	mu            sync.RWMutex
	activeWorkers int64
	peakWorkers   int64
}

// NewMultiProjectRunner creates a new MultiProjectRunner with controlled concurrency.
func NewMultiProjectRunner(opts ...RunnerOption) *MultiProjectRunner {
	r := &MultiProjectRunner{
		maxWorkers: DefaultMaxWorkers,
		listeners:  make([]RunnerListener, 0),
	}

	for _, opt := range opts {
		opt(r)
	}

	if r.executor == nil {
		r.executor = NewDefaultTaskExecutor(r.studioRepo)
	}

	return r
}

// AddListener registers a lifecycle event listener.
func (r *MultiProjectRunner) AddListener(l RunnerListener) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listeners = append(r.listeners, l)
}

// MaxWorkers returns the configured maximum concurrency.
func (r *MultiProjectRunner) MaxWorkers() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.maxWorkers
}

// PeakWorkers returns the highest number of concurrently running workers observed.
func (r *MultiProjectRunner) PeakWorkers() int {
	return int(atomic.LoadInt64(&r.peakWorkers))
}

// ResetMetrics resets peak concurrency and active worker counters.
func (r *MultiProjectRunner) ResetMetrics() {
	atomic.StoreInt64(&r.peakWorkers, 0)
	atomic.StoreInt64(&r.activeWorkers, 0)
}

func (r *MultiProjectRunner) trackWorkerStart() {
	curr := atomic.AddInt64(&r.activeWorkers, 1)
	for {
		peak := atomic.LoadInt64(&r.peakWorkers)
		if curr <= peak {
			break
		}
		if atomic.CompareAndSwapInt64(&r.peakWorkers, peak, curr) {
			break
		}
	}
}

func (r *MultiProjectRunner) trackWorkerEnd() {
	atomic.AddInt64(&r.activeWorkers, -1)
}

// ExecuteTasks runs a set of subtasks concurrently across isolated workers up to maxWorkers.
// Fault isolation guarantees that a failure or panic in one task never aborts the execution of other tasks.
func (r *MultiProjectRunner) ExecuteTasks(ctx context.Context, tasks []*db.ProjectTask, batchID ...string) (*CampaignRunResult, error) {
	startTime := time.Now().UTC()

	var bID string
	if len(batchID) > 0 {
		bID = batchID[0]
	} else if len(tasks) > 0 && tasks[0].BatchID != nil {
		bID = *tasks[0].BatchID
	}

	r.mu.RLock()
	maxWorkers := r.maxWorkers
	executor := r.executor
	listeners := append([]RunnerListener{}, r.listeners...)
	studioRepo := r.studioRepo
	r.mu.RUnlock()

	total := len(tasks)
	results := make([]*TaskExecutionResult, total)
	var resMu sync.Mutex

	var completedCount int64
	var failedCount int64

	// Concurrency semaphore
	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i, t := range tasks {
		idx := i
		taskCopy := t

		wg.Add(1)
		sem <- struct{}{} // acquire concurrency slot

		go func(task *db.ProjectTask, resultIndex int) {
			defer wg.Done()
			defer func() { <-sem }() // release concurrency slot

			r.trackWorkerStart()
			defer r.trackWorkerEnd()

			taskStart := time.Now().UTC()

			// Lifecycle listener start notification
			for _, l := range listeners {
				l.OnTaskStart(task)
			}

			// Fault isolation recovery guard against panics
			defer func() {
				if rec := recover(); rec != nil {
					taskFinish := time.Now().UTC()
					err := fmt.Errorf("task panicked: %v", rec)

					atomic.AddInt64(&failedCount, 1)

					// Update task to failed in studioRepo
					if studioRepo != nil {
						_ = studioRepo.UpdateTaskStatus(ctx, task.ID, "failed")
					}
					task.Status = "failed"

					execRes := &TaskExecutionResult{
						TaskID:     task.ID,
						ProjectID:  task.ProjectID,
						Status:     "failed",
						Error:      err.Error(),
						StartedAt:  taskStart,
						FinishedAt: taskFinish,
						DurationMs: taskFinish.Sub(taskStart).Milliseconds(),
					}

					resMu.Lock()
					results[resultIndex] = execRes
					resMu.Unlock()

					for _, l := range listeners {
						l.OnTaskFail(task, err)
					}
				}
			}()

			// Check context cancellation
			if err := ctx.Err(); err != nil {
				taskFinish := time.Now().UTC()
				atomic.AddInt64(&failedCount, 1)

				if studioRepo != nil {
					_ = studioRepo.UpdateTaskStatus(ctx, task.ID, "failed")
				}
				task.Status = "failed"

				execRes := &TaskExecutionResult{
					TaskID:     task.ID,
					ProjectID:  task.ProjectID,
					Status:     "failed",
					Error:      err.Error(),
					StartedAt:  taskStart,
					FinishedAt: taskFinish,
					DurationMs: taskFinish.Sub(taskStart).Milliseconds(),
				}

				resMu.Lock()
				results[resultIndex] = execRes
				resMu.Unlock()

				for _, l := range listeners {
					l.OnTaskFail(task, err)
				}
				return
			}

			// Execute task via executor
			res, err := executor.ExecuteTask(ctx, task)
			taskFinish := time.Now().UTC()

			if err != nil {
				atomic.AddInt64(&failedCount, 1)

				if studioRepo != nil {
					_ = studioRepo.UpdateTaskStatus(ctx, task.ID, "failed")
				}
				task.Status = "failed"

				if res == nil {
					res = &TaskExecutionResult{
						TaskID:     task.ID,
						ProjectID:  task.ProjectID,
						Status:     "failed",
						Error:      err.Error(),
						StartedAt:  taskStart,
						FinishedAt: taskFinish,
						DurationMs: taskFinish.Sub(taskStart).Milliseconds(),
					}
				}

				resMu.Lock()
				results[resultIndex] = res
				resMu.Unlock()

				for _, l := range listeners {
					l.OnTaskFail(task, err)
				}
			} else {
				atomic.AddInt64(&completedCount, 1)

				if res == nil {
					res = &TaskExecutionResult{
						TaskID:     task.ID,
						ProjectID:  task.ProjectID,
						Status:     "completed",
						StartedAt:  taskStart,
						FinishedAt: taskFinish,
						DurationMs: taskFinish.Sub(taskStart).Milliseconds(),
					}
				}

				resMu.Lock()
				results[resultIndex] = res
				resMu.Unlock()

				for _, l := range listeners {
					l.OnTaskComplete(task, res)
				}
			}
		}(taskCopy, idx)
	}

	wg.Wait()

	finishTime := time.Now().UTC()
	completed := int(atomic.LoadInt64(&completedCount))
	failed := int(atomic.LoadInt64(&failedCount))

	runResult := &CampaignRunResult{
		CampaignID:      bID,
		BatchID:         bID,
		TotalTasks:      total,
		CompletedTasks:  completed,
		FailedTasks:     failed,
		RunningTasks:    0,
		PeakConcurrency: r.PeakWorkers(),
		Results:         results,
		StartedAt:       startTime,
		FinishedAt:      finishTime,
		DurationMs:      finishTime.Sub(startTime).Milliseconds(),
		Success:         failed == 0,
	}

	for _, l := range listeners {
		l.OnCampaignComplete(bID, runResult)
	}

	return runResult, nil
}

// ExecuteCampaign retrieves a campaign by ID, updates its status to running, ensures subtasks exist,
// executes them using the multi-project worker pool, and updates the campaign status to completed/failed.
func (r *MultiProjectRunner) ExecuteCampaign(ctx context.Context, campaignID string) (*CampaignRunResult, error) {
	if r.batchRepo == nil {
		return nil, fmt.Errorf("batch campaign repository is required")
	}

	campaign, err := r.batchRepo.GetByID(ctx, campaignID)
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign '%s': %w", campaignID, err)
	}

	// 1. Mark campaign as running
	_ = r.batchRepo.UpdateStatus(ctx, campaignID, db.CampaignStatusRunning)

	// 2. Discover or decompile subtasks
	var subtasks []*db.ProjectTask

	if r.studioRepo != nil {
		existing, err := r.studioRepo.ListTasksByBatchID(ctx, campaignID)
		if err == nil && len(existing) > 0 {
			// Filter out parent epic if present (execute only child subtasks)
			for i := range existing {
				t := existing[i]
				if t.ParentTaskID != nil || len(existing) == 1 {
					subtasks = append(subtasks, &t)
				}
			}
			if len(subtasks) == 0 {
				for i := range existing {
					subtasks = append(subtasks, &existing[i])
				}
			}
		}
	}

	// If no subtasks exist yet, fan-out using decompiler engine
	if len(subtasks) == 0 && r.decompiler != nil {
		var decompileRes *DecompileResult
		if r.studioRepo != nil {
			decompileRes, err = r.decompiler.DecompileAndPersist(ctx, campaign)
		} else {
			decompileRes, err = r.decompiler.Decompile(ctx, campaign)
		}
		if err != nil {
			_ = r.batchRepo.UpdateStatus(ctx, campaignID, db.CampaignStatusFailed)
			return nil, fmt.Errorf("failed to decompile campaign subtasks: %w", err)
		}
		subtasks = decompileRes.Subtasks
	}

	if len(subtasks) == 0 {
		_ = r.batchRepo.UpdateStatus(ctx, campaignID, db.CampaignStatusFailed)
		return nil, ErrNoTargetProjectsFound
	}

	// 3. Execute subtasks in parallel with isolated workers
	runResult, err := r.ExecuteTasks(ctx, subtasks, campaignID)
	if err != nil {
		_ = r.batchRepo.UpdateStatus(ctx, campaignID, db.CampaignStatusFailed)
		return nil, err
	}

	// 4. Update campaign status based on completion
	finalStatus := db.CampaignStatusCompleted
	if runResult.FailedTasks > 0 && runResult.CompletedTasks == 0 {
		finalStatus = db.CampaignStatusFailed
	}
	_ = r.batchRepo.UpdateStatus(ctx, campaignID, finalStatus)

	return runResult, nil
}
