package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ScheduleType specifies the timing strategy for a recurring job.
type ScheduleType string

const (
	ScheduleInterval ScheduleType = "interval"
	ScheduleDaily    ScheduleType = "daily"
	ScheduleWeekly   ScheduleType = "weekly"
)

// JobStatus represents the outcome of the most recent execution.
type JobStatus string

const (
	StatusIdle    JobStatus = "idle"
	StatusRunning JobStatus = "running"
	StatusSuccess JobStatus = "success"
	StatusFailed  JobStatus = "failed"
)

// HandlerFunc is the signature of a scheduled task callback.
type HandlerFunc func(ctx context.Context) error

// Job defines a registered scheduled task.
type Job struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	ScheduleType ScheduleType  `json:"schedule_type"`
	Interval     time.Duration `json:"interval,omitempty"`
	TimeOfDay    string        `json:"time_of_day,omitempty"` // "HH:MM" in 24h format (e.g. "03:00")
	DayOfWeek    int           `json:"day_of_week,omitempty"` // 0 = Sunday, 1 = Monday, ... 6 = Saturday
	Enabled      bool          `json:"enabled"`
	Timeout      time.Duration `json:"timeout,omitempty"`

	LastRunAt  *time.Time `json:"last_run_at,omitempty"`
	NextRunAt  *time.Time `json:"next_run_at,omitempty"`
	LastStatus JobStatus  `json:"last_status"`
	LastError  string     `json:"last_error,omitempty"`

	Handler HandlerFunc `json:"-"`
}

// Scheduler coordinates timed background task executions.
type Scheduler struct {
	mu      sync.RWMutex
	jobs    map[string]*Job
	stopCh  chan struct{}
	running bool
}

// New instantiates an empty background scheduler.
func New() *Scheduler {
	return &Scheduler{
		jobs:   make(map[string]*Job),
		stopCh: make(chan struct{}),
	}
}

// Register adds a job to the scheduler. If next run time is not set, it is calculated automatically.
func (s *Scheduler) Register(job *Job) error {
	if job == nil || job.ID == "" {
		return errors.New("job and job ID are required")
	}
	if job.Handler == nil {
		return errors.New("job handler is required")
	}
	if job.Timeout == 0 {
		job.Timeout = 5 * time.Minute
	}
	if job.LastStatus == "" {
		job.LastStatus = StatusIdle
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	next := calculateNextRun(job, now)
	job.NextRunAt = &next

	s.jobs[job.ID] = job
	return nil
}

// Start launches the scheduler loop in a background goroutine.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	go s.runLoop(ctx)
}

// Stop halts the scheduler loop cleanly.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)
}

// Trigger forces an immediate run of a registered job by ID.
func (s *Scheduler) Trigger(ctx context.Context, jobID string) error {
	s.mu.RLock()
	job, exists := s.jobs[jobID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("job %q not found", jobID)
	}

	return s.executeJob(ctx, job)
}

// Request advances an enabled job's next run without starting another worker.
// Bursts retain the earliest requested run; an active job already owns execution.
func (s *Scheduler) Request(jobID string, delay time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, exists := s.jobs[jobID]
	if !exists {
		return fmt.Errorf("job %q not found", jobID)
	}
	if !job.Enabled || job.LastStatus == StatusRunning {
		return nil
	}
	if delay < 0 {
		delay = 0
	}
	requested := time.Now().Add(delay)
	if job.NextRunAt == nil || requested.Before(*job.NextRunAt) {
		job.NextRunAt = &requested
	}
	return nil
}

// ListJobs returns a snapshot of all registered jobs.
func (s *Scheduler) ListJobs() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		// Return shallow copy
		cpy := *j
		result = append(result, &cpy)
	}
	return result
}

// GetJob returns a copy of a specific job by ID.
func (s *Scheduler) GetJob(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	j, exists := s.jobs[id]
	if !exists {
		return nil, false
	}
	cpy := *j
	return &cpy, true
}

func (s *Scheduler) runLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			s.checkAndRunDueJobs(ctx, now)
		}
	}
}

func (s *Scheduler) checkAndRunDueJobs(ctx context.Context, now time.Time) {
	s.mu.Lock()
	var dueJobs []*Job
	for _, j := range s.jobs {
		if !j.Enabled {
			continue
		}
		if j.NextRunAt != nil && !now.Before(*j.NextRunAt) {
			dueJobs = append(dueJobs, j)
			next := calculateNextRun(j, now)
			j.NextRunAt = &next
		}
	}
	s.mu.Unlock()

	for _, j := range dueJobs {
		go func(job *Job) {
			_ = s.executeJob(ctx, job)
		}(j)
	}
}

func (s *Scheduler) executeJob(ctx context.Context, j *Job) (err error) {
	s.mu.Lock()
	if j.LastStatus == StatusRunning {
		s.mu.Unlock()
		return errors.New("job is already running")
	}
	j.LastStatus = StatusRunning
	now := time.Now()
	j.LastRunAt = &now
	timeout := j.Timeout
	s.mu.Unlock()

	jobCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("job panicked: %v", r)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if err != nil {
			j.LastStatus = StatusFailed
			j.LastError = err.Error()
		} else {
			j.LastStatus = StatusSuccess
			j.LastError = ""
		}
	}()

	err = j.Handler(jobCtx)
	return err
}

// calculateNextRun computes the next execution time based on schedule type and current reference time.
func calculateNextRun(j *Job, ref time.Time) time.Time {
	switch j.ScheduleType {
	case ScheduleInterval:
		interval := j.Interval
		if interval <= 0 {
			interval = 1 * time.Hour
		}
		return ref.Add(interval)

	case ScheduleDaily:
		hour, min := parseTimeOfDay(j.TimeOfDay)
		target := time.Date(ref.Year(), ref.Month(), ref.Day(), hour, min, 0, 0, ref.Location())
		if !target.After(ref) {
			target = target.AddDate(0, 0, 1)
		}
		return target

	case ScheduleWeekly:
		hour, min := parseTimeOfDay(j.TimeOfDay)
		targetDay := j.DayOfWeek
		if targetDay < 0 || targetDay > 6 {
			targetDay = 1 // default Monday
		}

		currentDay := int(ref.Weekday())
		daysUntil := (targetDay - currentDay + 7) % 7
		target := time.Date(ref.Year(), ref.Month(), ref.Day(), hour, min, 0, 0, ref.Location()).AddDate(0, 0, daysUntil)
		if !target.After(ref) {
			target = target.AddDate(0, 0, 7)
		}
		return target

	default:
		return ref.Add(24 * time.Hour)
	}
}

func parseTimeOfDay(tod string) (hour, min int) {
	if tod == "" {
		return 3, 0 // Default 03:00 WIB
	}
	var h, m int
	if n, _ := fmt.Sscanf(tod, "%d:%d", &h, &m); n == 2 {
		if h >= 0 && h <= 23 && m >= 0 && m <= 59 {
			return h, m
		}
	}
	return 3, 0
}
