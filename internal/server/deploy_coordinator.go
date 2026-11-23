package server

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/project"
)

// DeployJob represents a requested auto-deployment execution.
type DeployJob struct {
	ProjectID      string
	TaskID         string
	LocalPath      string
	DeployScript   string
	Environment    string
	CommitSHA      string
	CoalescedCount int
	EnqueuedAt     time.Time
}

type projectDeployState struct {
	busy           bool
	pending        *DeployJob
	lastDeploySHA  string
	lastDeployTime time.Time
	lastErr        error
}

// DeployCoordinator coordinates and coalesces auto-deploys per project,
// preventing concurrent execution of deploy scripts and eliminating build pile-up.
// Inspired by Fural's two-layer coalescing deployment architecture.
type DeployCoordinator struct {
	mu       sync.Mutex
	projects map[string]*projectDeployState
	execute  func(ctx context.Context, job DeployJob) error
}

// NewDeployCoordinator creates a new deployment coordinator with the default shell executor.
func NewDeployCoordinator() *DeployCoordinator {
	c := &DeployCoordinator{
		projects: make(map[string]*projectDeployState),
	}
	c.execute = c.defaultExecute
	return c
}

// SetExecutor replaces the execution function (used for testing or custom executors).
func (c *DeployCoordinator) SetExecutor(fn func(ctx context.Context, job DeployJob) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.execute = fn
}

// Enqueue submits a deployment job. If a deployment is already in-flight for this project,
// the request is non-blocking and coalesced into the single pending follow-up slot (latest SHA wins).
func (c *DeployCoordinator) Enqueue(ctx context.Context, job DeployJob) {
	if job.ProjectID == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	state, exists := c.projects[job.ProjectID]
	if !exists {
		state = &projectDeployState{}
		c.projects[job.ProjectID] = state
	}

	if job.CoalescedCount < 1 {
		job.CoalescedCount = 1
	}
	if job.EnqueuedAt.IsZero() {
		job.EnqueuedAt = time.Now().UTC()
	}

	if !state.busy {
		state.busy = true
		go c.runLeader(job)
		log.Printf("[DeployCoordinator] Enqueued deploy leader for project %s (task %s)", job.ProjectID, job.TaskID)
		return
	}

	// Busy: coalesce into pending follow-up slot
	if state.pending == nil {
		cp := job
		state.pending = &cp
		log.Printf("[DeployCoordinator] Project %s busy; queued pending follow-up (task %s)", job.ProjectID, job.TaskID)
	} else {
		state.pending.TaskID = job.TaskID
		if job.CommitSHA != "" {
			state.pending.CommitSHA = job.CommitSHA
		}
		state.pending.CoalescedCount++
		log.Printf("[DeployCoordinator] Coalesced deploy for project %s (latest task %s, count %d)",
			job.ProjectID, job.TaskID, state.pending.CoalescedCount)
	}
}

func (c *DeployCoordinator) runLeader(initialJob DeployJob) {
	currentJob := initialJob
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
		execFn := c.getExecutor()
		err := execFn(ctx, currentJob)
		cancel()

		c.mu.Lock()
		state := c.projects[currentJob.ProjectID]
		if state != nil {
			state.lastErr = err
			if err == nil {
				state.lastDeploySHA = currentJob.CommitSHA
				state.lastDeployTime = time.Now().UTC()
			}
		}

		if err != nil {
			log.Printf("[DeployCoordinator] Deploy failed for project %s: %v", currentJob.ProjectID, err)
		} else {
			log.Printf("[DeployCoordinator] Deploy succeeded for project %s (task %s, coalesced %d)",
				currentJob.ProjectID, currentJob.TaskID, currentJob.CoalescedCount)
		}

		if state != nil && state.pending != nil {
			// There is a pending coalesced job; pick it up as the next run
			nextJob := *state.pending
			state.pending = nil
			c.mu.Unlock()
			log.Printf("[DeployCoordinator] Picking up coalesced pending deploy for project %s (task %s)", nextJob.ProjectID, nextJob.TaskID)
			currentJob = nextJob
			continue
		}

		if state != nil {
			state.busy = false
		}
		c.mu.Unlock()
		break
	}
}

func (c *DeployCoordinator) getExecutor() func(ctx context.Context, job DeployJob) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.execute != nil {
		return c.execute
	}
	return c.defaultExecute
}

// IsBusy reports whether a deployment is currently in flight for the given project.
func (c *DeployCoordinator) IsBusy(projectID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, ok := c.projects[projectID]
	return ok && state.busy
}

// PendingCount reports how many deploy requests are coalesced in the pending slot for the project.
func (c *DeployCoordinator) PendingCount(projectID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, ok := c.projects[projectID]
	if !ok || state.pending == nil {
		return 0
	}
	return state.pending.CoalescedCount
}

func (c *DeployCoordinator) defaultExecute(ctx context.Context, job DeployJob) error {
	if strings.TrimSpace(job.DeployScript) != "" && job.LocalPath != "" {
		log.Printf("[DeployCoordinator] Executing project deploy script for %s in %s", job.ProjectID, job.LocalPath)
		cmd := exec.CommandContext(ctx, "bash", "-c", job.DeployScript)
		cmd.Dir = job.LocalPath
		baseEnv := append(os.Environ(),
			"CI=true",
			"npm_config_yes=true",
			"DEBIAN_FRONTEND=noninteractive",
			"GIT_OPTIONAL_LOCKS=0",
		)
		if job.Environment != "" {
			envMap := project.ParseEnvironment(job.Environment)
			var envSlice []string
			for k, v := range envMap {
				envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
			}
			cmd.Env = append(baseEnv, envSlice...)
		} else {
			cmd.Env = baseEnv
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("[DeployCoordinator] Project deploy script failed for %s: %v: %s", job.ProjectID, err, string(out))
			return fmt.Errorf("deploy script failed: %w: %s", err, string(out))
		}
		log.Printf("[DeployCoordinator] Project deploy script succeeded for %s", job.ProjectID)
		return nil
	}

	if job.LocalPath != "" {
		deployScript := filepath.Join(job.LocalPath, "scripts", "deploy.sh")
		if info, err := os.Stat(deployScript); err == nil && !info.IsDir() {
			log.Printf("[DeployCoordinator] Executing native Cuan auto-deploy in %s", job.LocalPath)
			cmd := exec.CommandContext(ctx, "bash", deployScript, "--local", "--native-sync")
			cmd.Dir = job.LocalPath
			cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
			out, err := cmd.CombinedOutput()
			if err != nil {
				log.Printf("[DeployCoordinator] Native Cuan deploy failed for %s: %v: %s", job.ProjectID, err, string(out))
				return fmt.Errorf("native deploy failed: %w: %s", err, string(out))
			}
			log.Printf("[DeployCoordinator] Native Cuan deploy succeeded for %s", job.ProjectID)
			return nil
		}
	}
	return nil
}
