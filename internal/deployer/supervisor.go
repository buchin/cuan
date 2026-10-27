package deployer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/crypto"
	cuanDB "github.com/buchin/cuan/internal/db"
)

var (
	ErrProcessAlreadyRunning = errors.New("process already running for this project")
	ErrProcessNotRunning     = errors.New("no active process found for this project")
	ErrHealthCheckFailed     = errors.New("health check poller failed to verify HTTP 200 response")
	ErrInvalidPort           = errors.New("invalid port allocated")
)

// ProcessLauncher allows intercepting/mocking process execution for testing.
type ProcessLauncher interface {
	Start(ctx context.Context, dir string, env []string, stdout, stderr io.Writer, name string, args ...string) (*exec.Cmd, int, error)
}

// DefaultProcessLauncher spawns real host OS child processes with process group isolation.
type DefaultProcessLauncher struct{}

func (l *DefaultProcessLauncher) Start(ctx context.Context, dir string, env []string, stdout, stderr io.Writer, name string, args ...string) (*exec.Cmd, int, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	} else {
		cmd.Env = os.Environ()
	}

	// Set process group for subtree isolation
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if stdout != nil {
		cmd.Stdout = stdout
	}
	if stderr != nil {
		cmd.Stderr = stderr
	}

	if err := cmd.Start(); err != nil {
		return nil, 0, err
	}

	return cmd, cmd.Process.Pid, nil
}

// SupervisedProcess tracks a live running process instance managed by ProcessSupervisor.
type SupervisedProcess struct {
	DeploymentID string            `json:"deployment_id"`
	ProjectID    string            `json:"project_id"`
	ServerID     *string           `json:"server_id,omitempty"`
	PID          int               `json:"pid"`
	Port         int               `json:"port"`
	Domain       string            `json:"domain"`
	Runtime      string            `json:"runtime"`
	Status       string            `json:"status"` // running, stopped, failed, crashed
	Health       string            `json:"health"` // healthy, unhealthy, unknown
	StartedAt    time.Time         `json:"started_at"`
	StoppedAt    *time.Time        `json:"stopped_at,omitempty"`
	Cmd          *exec.Cmd         `json:"-"`
	Streamer     *LogStreamer      `json:"-"`
	Done           chan struct{}     `json:"-"`
	ExitErr        error             `json:"-"`
	Stopped        bool              `json:"-"`
	SkipHealthWait bool              `json:"-"`
	EnvVars        map[string]string `json:"-"`
	mu             sync.Mutex        `json:"-"`
}

// SupervisedStatus represents runtime status for public reporting.
type SupervisedStatus struct {
	ProjectID     string    `json:"project_id"`
	DeploymentID  string    `json:"deployment_id"`
	PID           int       `json:"pid"`
	Port          int       `json:"port"`
	Domain        string    `json:"domain"`
	Runtime       string    `json:"runtime"`
	Status        string    `json:"status"`
	Health        string    `json:"health"`
	StartedAt     time.Time `json:"started_at"`
	UptimeSeconds int64     `json:"uptime_seconds"`
}

// SupervisorStartRequest holds configuration for starting a supervised service.
type SupervisorStartRequest struct {
	ProjectID      string            `json:"project_id"`
	DeploymentID   string            `json:"deployment_id,omitempty"`
	ProjectDir     string            `json:"project_dir"`
	ServerID       *string           `json:"server_id,omitempty"`
	Port           int               `json:"port,omitempty"` // 0 = auto-allocate from PortPool
	Domain         string            `json:"domain,omitempty"`
	Stack          string            `json:"stack,omitempty"`
	CustomCommand  string            `json:"custom_command,omitempty"`
	EnvVars        map[string]string `json:"env_vars,omitempty"`
	HealthPath     string            `json:"health_path,omitempty"` // default "/"
	HealthTimeout  time.Duration     `json:"health_timeout,omitempty"`
	HealthInterval time.Duration     `json:"health_interval,omitempty"`
	SkipHealthWait bool              `json:"skip_health_wait,omitempty"`
}

// HealthCheckOutcome records the details of health check verification.
type HealthCheckOutcome struct {
	TargetURL    string        `json:"target_url"`
	StatusCode   int           `json:"status_code"`
	Healthy      bool          `json:"healthy"`
	Attempts     int           `json:"attempts"`
	Duration     time.Duration `json:"duration"`
	ResponseBody string        `json:"response_body,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// ProcessSupervisorConfig holds dependencies for ProcessSupervisor.
type ProcessSupervisorConfig struct {
	PortPool          *PortPool
	Router            Router
	Repo              cuanDB.ScriptDeploymentRepository
	ProvisionedDBRepo cuanDB.ProvisionedDatabaseRepository
	ServerRepo        cuanDB.ServerRepository
	SSHManager        SSHManager
	KeyVault          *crypto.KeyVault
	Launcher          ProcessLauncher
	HTTPClient        *http.Client
	ProjectBaseDir    string
	DeploymentsDir    string
	VaultSecret       string
}

// ProcessSupervisor manages process lifecycles, dynamic port pool, Caddy routing, and health checks.
type ProcessSupervisor struct {
	cfg             ProcessSupervisorConfig
	activeProcesses map[string]*SupervisedProcess // projectID -> process
	streamers       map[string]*LogStreamer       // projectID -> streamer
	mu              sync.RWMutex
}

// NewProcessSupervisor creates a new supervisor engine.
func NewProcessSupervisor(cfg ProcessSupervisorConfig) *ProcessSupervisor {
	if cfg.PortPool == nil {
		cfg.PortPool = NewPortPool()
	}
	if cfg.Router == nil {
		cfg.Router = NewCaddyRouter(DefaultCaddyConfig())
	}
	if cfg.Launcher == nil {
		cfg.Launcher = &DefaultProcessLauncher{}
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{
			Timeout: 3 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return nil // follow redirects
			},
		}
	}
	if cfg.ProjectBaseDir == "" {
		cfg.ProjectBaseDir = filepath.Join("data", "projects")
	}
	if cfg.VaultSecret == "" {
		cfg.VaultSecret = "cuan-default-database-provisioner-vault-key"
	}

	return &ProcessSupervisor{
		cfg:             cfg,
		activeProcesses: make(map[string]*SupervisedProcess),
		streamers:       make(map[string]*LogStreamer),
	}
}

// PortPool returns the underlying PortPool.
func (s *ProcessSupervisor) PortPool() *PortPool {
	return s.cfg.PortPool
}

// Router returns the underlying Router.
func (s *ProcessSupervisor) Router() Router {
	return s.cfg.Router
}

// GetOrCreateStreamer retrieves or instantiates a LogStreamer for the project.
func (s *ProcessSupervisor) GetOrCreateStreamer(projectID string) *LogStreamer {
	s.mu.Lock()
	defer s.mu.Unlock()

	if st, ok := s.streamers[projectID]; ok {
		return st
	}

	st := NewLogStreamer(projectID, nil, 2000)
	s.streamers[projectID] = st
	return st
}

// GetStreamer returns an existing streamer if active.
func (s *ProcessSupervisor) GetStreamer(projectID string) *LogStreamer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.streamers[projectID]
}

// StartProcess launches a process, allocates dynamic port, configures Caddy, and polls health.
func (s *ProcessSupervisor) StartProcess(ctx context.Context, req SupervisorStartRequest) (*SupervisedProcess, error) {
	s.mu.Lock()

	// If already running, terminate old instance cleanly first
	if oldProc, exists := s.activeProcesses[req.ProjectID]; exists && oldProc != nil && oldProc.Status == cuanDB.BuildStatusRunning {
		s.mu.Unlock()
		_ = s.StopProcess(ctx, req.ProjectID)
		s.mu.Lock()
	}

	projectDir := req.ProjectDir
	if projectDir == "" {
		projectDir = filepath.Join(s.cfg.ProjectBaseDir, req.ProjectID)
	}

	deploymentID := req.DeploymentID
	if deploymentID == "" {
		deploymentID = "sd_" + req.ProjectID
	}

	// 1. Dynamic Port Allocation
	port := req.Port
	if port <= 0 {
		allocPort, err := s.cfg.PortPool.Allocate(deploymentID)
		if err != nil {
			s.mu.Unlock()
			return nil, fmt.Errorf("dynamic port allocation failed: %w", err)
		}
		port = allocPort
	}
	s.mu.Unlock()

	// Setup streamer
	streamer := s.GetOrCreateStreamer(req.ProjectID)
	s.log(streamer, StreamSystem, fmt.Sprintf("[Supervisor] Allocated port %d for project %s", port, req.ProjectID))

	// 2. Resolve Domain
	domain := strings.TrimSpace(req.Domain)
	slug := strings.ToLower(strings.TrimSpace(req.ProjectID))
	if domain == "" {
		domain = s.cfg.Router.FormatDomain(slug)
	}

	// 3. Resolve Stack & Command
	stack := strings.ToLower(strings.TrimSpace(req.Stack))
	if stack == "" {
		stack = DetectStack(projectDir)
	}

	binary, args := s.resolveLaunchCommand(projectDir, stack, port, req.CustomCommand)
	s.log(streamer, StreamSystem, fmt.Sprintf("[Supervisor] Spawning %s runtime command: %s %s", stack, binary, strings.Join(args, " ")))

	// Prepare environment variables
	envMap := make(map[string]string)
	for k, v := range req.EnvVars {
		envMap[k] = v
	}
	envMap["PORT"] = fmt.Sprintf("%d", port)
	envMap["APP_PORT"] = fmt.Sprintf("%d", port)
	envMap["HOST"] = "127.0.0.1"

	var envSlice []string
	for k, v := range envMap {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", k, v))
	}

	stdoutPipe := newLogStreamWriter(streamer, StreamStdout)
	stderrPipe := newLogStreamWriter(streamer, StreamStderr)

	// 4. Launch child process
	cmd, pid, err := s.cfg.Launcher.Start(ctx, projectDir, envSlice, stdoutPipe, stderrPipe, binary, args...)
	if err != nil {
		s.cfg.PortPool.Release(deploymentID)
		s.log(streamer, StreamStderr, fmt.Sprintf("[Supervisor] Failed to launch process: %v", err))
		return nil, fmt.Errorf("spawn process (%s): %w", binary, err)
	}

	proc := &SupervisedProcess{
		DeploymentID: deploymentID,
		ProjectID:    req.ProjectID,
		ServerID:     req.ServerID,
		PID:          pid,
		Port:         port,
		Domain:       domain,
		Runtime:      stack,
		Status:       cuanDB.BuildStatusRunning,
		Health:       cuanDB.HealthStatusUnknown,
		StartedAt:    time.Now().UTC(),
		Cmd:          cmd,
		Streamer:     streamer,
		Done:           make(chan struct{}),
		SkipHealthWait: req.SkipHealthWait,
		EnvVars:        envMap,
	}

	s.mu.Lock()
	s.activeProcesses[req.ProjectID] = proc
	s.mu.Unlock()

	s.log(streamer, StreamSystem, fmt.Sprintf("[Supervisor] Process spawned with PID %d on port %d", pid, port))

	// 5. Background exit watcher (Crash detection)
	go s.watchProcessExit(proc)

	// 6. Caddy Reverse Proxy Registration
	var regErr error
	if domain != "" && domain != s.cfg.Router.FormatDomain(slug) {
		regErr = s.cfg.Router.AddCustomDomainRoute(ctx, domain, LocalTarget(port))
	} else {
		regErr = s.cfg.Router.RegisterRoute(ctx, slug, port)
	}
	if regErr != nil {
		s.log(streamer, StreamStderr, fmt.Sprintf("[Supervisor] Warning: Caddy route registration returned: %v", regErr))
	} else {
		s.log(streamer, StreamSystem, fmt.Sprintf("[Supervisor] Reverse proxy route mapped: %s -> 127.0.0.1:%d", domain, port))
	}

	// 7. Health Check Poller
	if !req.SkipHealthWait {
		healthPath := req.HealthPath
		if healthPath == "" {
			healthPath = "/"
		}
		targetURL := fmt.Sprintf("http://127.0.0.1:%d%s", port, healthPath)
		s.log(streamer, StreamSystem, fmt.Sprintf("[HealthCheck] Probing target URL %s...", targetURL))

		healthTimeout := req.HealthTimeout
		if healthTimeout <= 0 {
			healthTimeout = 20 * time.Second
		}
		healthInterval := req.HealthInterval
		if healthInterval <= 0 {
			healthInterval = 400 * time.Millisecond
		}

		outcome, err := PollHTTPHealth(ctx, targetURL, WithHealthTimeout(healthTimeout), WithHealthInterval(healthInterval), WithHTTPClient(s.cfg.HTTPClient))
		if err != nil || !outcome.Healthy {
			proc.mu.Lock()
			proc.Health = cuanDB.HealthStatusUnhealthy
			proc.mu.Unlock()
			s.log(streamer, StreamStderr, fmt.Sprintf("[HealthCheck] Health check failed or timed out: %v", err))
		} else {
			proc.mu.Lock()
			proc.Health = cuanDB.HealthStatusHealthy
			proc.mu.Unlock()
			s.log(streamer, StreamSystem, fmt.Sprintf("[HealthCheck] Health check passed: HTTP %d received in %s", outcome.StatusCode, outcome.Duration.Round(time.Millisecond)))
		}
	} else {
		proc.mu.Lock()
		proc.Health = cuanDB.HealthStatusHealthy
		proc.mu.Unlock()
	}

	// 8. Update DB repository record if configured
	if s.cfg.Repo != nil {
		s.updateDBRecord(ctx, proc)
	}

	return proc, nil
}

// StopProcess terminates a running supervised process and cleans up resources.
func (s *ProcessSupervisor) StopProcess(ctx context.Context, projectID string) error {
	s.mu.Lock()
	proc, exists := s.activeProcesses[projectID]
	if !exists || proc == nil {
		s.mu.Unlock()
		return ErrProcessNotRunning
	}
	if proc.Status == cuanDB.BuildStatusStopped {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	proc.mu.Lock()
	proc.Stopped = true
	now := time.Now().UTC()
	proc.StoppedAt = &now
	proc.Status = cuanDB.BuildStatusStopped
	proc.Health = cuanDB.HealthStatusUnknown
	pid := proc.PID
	port := proc.Port
	depID := proc.DeploymentID
	slug := strings.ToLower(strings.TrimSpace(projectID))
	domain := proc.Domain
	cmd := proc.Cmd
	proc.mu.Unlock()

	s.log(proc.Streamer, StreamSystem, fmt.Sprintf("[Supervisor] Stopping process (PID %d)...", pid))

	// Kill process group cleanly
	if cmd != nil && cmd.Process != nil && pid > 0 {
		_ = syscall.Kill(-pid, syscall.SIGTERM)
		// Give short grace period then SIGKILL
		go func(p int) {
			time.Sleep(1 * time.Second)
			_ = syscall.Kill(-p, syscall.SIGKILL)
		}(pid)
	}

	// Release port from pool
	s.cfg.PortPool.Release(depID)
	s.cfg.PortPool.ReleasePort(port)

	// Revoke Caddy route
	if domain != "" && domain != s.cfg.Router.FormatDomain(slug) {
		_ = s.cfg.Router.RemoveCustomDomainRoute(ctx, domain)
	} else {
		_ = s.cfg.Router.UnregisterRoute(ctx, slug)
	}

	// Update DB record
	if s.cfg.Repo != nil {
		dep, err := s.cfg.Repo.GetDeploymentByProjectID(ctx, projectID)
		if err == nil && dep != nil {
			dep.BuildStatus = cuanDB.BuildStatusStopped
			dep.Health = cuanDB.HealthStatusUnknown
			dep.UpdatedAt = time.Now().UTC()
			_ = s.cfg.Repo.UpdateDeployment(ctx, dep)
		}
	}

	s.log(proc.Streamer, StreamSystem, fmt.Sprintf("[Supervisor] Process (PID %d) stopped and port %d released.", pid, port))
	return nil
}

// RestartProcess stops and relaunches the service.
func (s *ProcessSupervisor) RestartProcess(ctx context.Context, projectID string) (*SupervisedProcess, error) {
	s.mu.RLock()
	existing, exists := s.activeProcesses[projectID]
	s.mu.RUnlock()

	var req SupervisorStartRequest
	if exists && existing != nil {
		req = SupervisorStartRequest{
			ProjectID:     projectID,
			DeploymentID:  existing.DeploymentID,
			ServerID:      existing.ServerID,
			Port:          existing.Port, // reuse port if possible
			Domain:        existing.Domain,
			Stack:          existing.Runtime,
			EnvVars:        existing.EnvVars,
			SkipHealthWait: existing.SkipHealthWait,
			HealthTimeout:  15 * time.Second,
		}
	} else {
		req = SupervisorStartRequest{
			ProjectID: projectID,
		}
	}

	// If repository has deployment record, populate missing fields
	if s.cfg.Repo != nil {
		if dep, err := s.cfg.Repo.GetDeploymentByProjectID(ctx, projectID); err == nil && dep != nil {
			if req.DeploymentID == "" {
				req.DeploymentID = dep.ID
			}
			if req.Port <= 0 {
				req.Port = dep.Port
			}
			if req.Domain == "" {
				req.Domain = dep.Domain
			}
			if req.ServerID == nil {
				req.ServerID = dep.ServerID
			}
			if len(req.EnvVars) == 0 && dep.EnvEncrypted != "" {
				if plainEnv, err := dep.DecryptEnvMap(s.cfg.VaultSecret); err == nil {
					req.EnvVars = plainEnv
				}
			}
		}
	}

	_ = s.StopProcess(ctx, projectID)
	return s.StartProcess(ctx, req)
}

// GetStatus returns the live status of the supervised service.
func (s *ProcessSupervisor) GetStatus(ctx context.Context, projectID string) (*SupervisedStatus, error) {
	s.mu.RLock()
	proc, exists := s.activeProcesses[projectID]
	s.mu.RUnlock()

	if !exists || proc == nil {
		// Check repository for inactive/stopped record
		if s.cfg.Repo != nil {
			dep, err := s.cfg.Repo.GetDeploymentByProjectID(ctx, projectID)
			if err == nil && dep != nil {
				return &SupervisedStatus{
					ProjectID:    projectID,
					DeploymentID: dep.ID,
					Port:         dep.Port,
					Domain:       dep.Domain,
					Status:       dep.BuildStatus,
					Health:       dep.Health,
				}, nil
			}
		}
		return nil, ErrProcessNotRunning
	}

	proc.mu.Lock()
	defer proc.mu.Unlock()

	uptime := int64(0)
	if proc.Status == cuanDB.BuildStatusRunning {
		uptime = int64(time.Since(proc.StartedAt).Seconds())
	}

	return &SupervisedStatus{
		ProjectID:     proc.ProjectID,
		DeploymentID:  proc.DeploymentID,
		PID:           proc.PID,
		Port:          proc.Port,
		Domain:        proc.Domain,
		Runtime:       proc.Runtime,
		Status:        proc.Status,
		Health:        proc.Health,
		StartedAt:     proc.StartedAt,
		UptimeSeconds: uptime,
	}, nil
}

// watchProcessExit monitors child process exit and detects crashes.
func (s *ProcessSupervisor) watchProcessExit(proc *SupervisedProcess) {
	if proc.Cmd == nil {
		return
	}

	err := proc.Cmd.Wait()

	proc.mu.Lock()
	proc.ExitErr = err
	wasStopped := proc.Stopped
	pid := proc.PID
	port := proc.Port
	depID := proc.DeploymentID
	projectID := proc.ProjectID
	proc.mu.Unlock()

	close(proc.Done)

	if !wasStopped {
		// Process crashed or terminated unexpectedly!
		proc.mu.Lock()
		proc.Status = cuanDB.BuildStatusFailed
		proc.Health = cuanDB.HealthStatusUnhealthy
		proc.mu.Unlock()

		s.mu.Lock()
		delete(s.activeProcesses, projectID)
		s.mu.Unlock()

		// Release port
		s.cfg.PortPool.Release(depID)
		s.cfg.PortPool.ReleasePort(port)

		s.log(proc.Streamer, StreamStderr, fmt.Sprintf("[Supervisor] CRASH DETECTED: Process (PID %d) terminated unexpectedly: %v", pid, err))

		// Update DB
		if s.cfg.Repo != nil {
			dep, dbErr := s.cfg.Repo.GetDeploymentByProjectID(context.Background(), projectID)
			if dbErr == nil && dep != nil {
				dep.BuildStatus = cuanDB.BuildStatusFailed
				dep.Health = cuanDB.HealthStatusUnhealthy
				dep.UpdatedAt = time.Now().UTC()
				_ = s.cfg.Repo.UpdateDeployment(context.Background(), dep)
			}
		}
	}
}

// resolveLaunchCommand chooses the best application server command.
func (s *ProcessSupervisor) resolveLaunchCommand(dir, stack string, port int, custom string) (string, []string) {
	if custom != "" {
		parts := strings.Fields(custom)
		if len(parts) > 0 {
			return parts[0], parts[1:]
		}
	}

	switch stack {
	case "laravel":
		if fileExists(filepath.Join(dir, "artisan")) {
			return "php", []string{"artisan", "serve", "--host=127.0.0.1", fmt.Sprintf("--port=%d", port)}
		}
		if fileExists(filepath.Join(dir, "public", "index.php")) {
			return "php", []string{"-S", fmt.Sprintf("127.0.0.1:%d", port), "-t", filepath.Join(dir, "public")}
		}
		return "php", []string{"-S", fmt.Sprintf("127.0.0.1:%d", port), "-t", dir}

	case "php":
		docRoot := dir
		if fileExists(filepath.Join(dir, "public", "index.php")) {
			docRoot = filepath.Join(dir, "public")
		}
		return "php", []string{"-S", fmt.Sprintf("127.0.0.1:%d", port), "-t", docRoot}

	case "node", "next", "react", "express":
		if fileExists(filepath.Join(dir, "package.json")) {
			content, err := os.ReadFile(filepath.Join(dir, "package.json"))
			if err == nil {
				var pkg map[string]interface{}
				if json.Unmarshal(content, &pkg) == nil {
					if scripts, ok := pkg["scripts"].(map[string]interface{}); ok {
						if _, hasStart := scripts["start"]; hasStart {
							return "npm", []string{"start"}
						}
					}
				}
			}
		}
		if fileExists(filepath.Join(dir, "server.js")) {
			return "node", []string{"server.js"}
		}
		if fileExists(filepath.Join(dir, "index.js")) {
			return "node", []string{"index.js"}
		}
		if fileExists(filepath.Join(dir, "app.js")) {
			return "node", []string{"app.js"}
		}
		return "node", []string{"index.js"}

	case "python", "django", "flask":
		if fileExists(filepath.Join(dir, "manage.py")) {
			return "python3", []string{"manage.py", "runserver", fmt.Sprintf("127.0.0.1:%d", port)}
		}
		if fileExists(filepath.Join(dir, "app.py")) {
			return "python3", []string{"app.py"}
		}
		return "python3", []string{"-m", "http.server", fmt.Sprintf("%d", port), "--directory", dir}

	case "html", "static":
		docRoot := dir
		if fileExists(filepath.Join(dir, "public", "index.html")) {
			docRoot = filepath.Join(dir, "public")
		}
		// Prefer Python's standard http.server or PHP's built-in server
		return "php", []string{"-S", fmt.Sprintf("127.0.0.1:%d", port), "-t", docRoot}

	default:
		return "php", []string{"-S", fmt.Sprintf("127.0.0.1:%d", port), "-t", dir}
	}
}

// updateDBRecord persists deployment status, port, domain, and health.
func (s *ProcessSupervisor) updateDBRecord(ctx context.Context, proc *SupervisedProcess) {
	dep, err := s.cfg.Repo.GetDeploymentByProjectID(ctx, proc.ProjectID)
	if err != nil || dep == nil {
		dep = &cuanDB.ScriptDeployment{
			ID:          proc.DeploymentID,
			ProjectID:   proc.ProjectID,
			ServerID:    proc.ServerID,
			Port:        proc.Port,
			Domain:      proc.Domain,
			BuildStatus: proc.Status,
			Health:      proc.Health,
			CreatedAt:   proc.StartedAt,
			UpdatedAt:   time.Now().UTC(),
		}
		_ = s.cfg.Repo.CreateDeployment(ctx, dep)
	} else {
		dep.Port = proc.Port
		dep.Domain = proc.Domain
		dep.BuildStatus = proc.Status
		dep.Health = proc.Health
		dep.UpdatedAt = time.Now().UTC()
		_ = s.cfg.Repo.UpdateDeployment(ctx, dep)
	}
}

func (s *ProcessSupervisor) log(streamer *LogStreamer, stream, msg string) {
	if streamer != nil {
		streamer.Append(context.Background(), LogEntry{
			Stream:    stream,
			Message:   msg,
			Timestamp: time.Now().UTC(),
		})
	}
}

// HealthCheckOption configures PollHTTPHealth.
type HealthCheckOption func(*healthConfig)

type healthConfig struct {
	timeout    time.Duration
	interval   time.Duration
	httpClient *http.Client
}

// WithHealthTimeout sets maximum polling duration.
func WithHealthTimeout(d time.Duration) HealthCheckOption {
	return func(c *healthConfig) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// WithHealthInterval sets polling interval between attempts.
func WithHealthInterval(d time.Duration) HealthCheckOption {
	return func(c *healthConfig) {
		if d > 0 {
			c.interval = d
		}
	}
}

// WithHTTPClient sets custom HTTP client for health checks.
func WithHTTPClient(client *http.Client) HealthCheckOption {
	return func(c *healthConfig) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// PollHTTPHealth probes targetURL periodically until HTTP 200 (or 2xx/3xx) is returned.
func PollHTTPHealth(ctx context.Context, targetURL string, opts ...HealthCheckOption) (*HealthCheckOutcome, error) {
	cfg := &healthConfig{
		timeout:  20 * time.Second,
		interval: 400 * time.Millisecond,
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return nil
			},
		},
	}
	for _, opt := range opts {
		opt(cfg)
	}

	startTime := time.Now()
	pollCtx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	attempts := 0
	ticker := time.NewTicker(cfg.interval)
	defer ticker.Stop()

	outcome := &HealthCheckOutcome{
		TargetURL: targetURL,
	}

	for {
		attempts++
		outcome.Attempts = attempts

		req, err := http.NewRequestWithContext(pollCtx, http.MethodGet, targetURL, nil)
		if err == nil {
			resp, reqErr := cfg.httpClient.Do(req)
			if reqErr == nil {
				outcome.StatusCode = resp.StatusCode
				_ = resp.Body.Close()

				// If HTTP 200 or successful 2xx/3xx response
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					outcome.Healthy = true
					outcome.Duration = time.Since(startTime)
					return outcome, nil
				}
			} else {
				outcome.Error = reqErr.Error()
			}
		}

		select {
		case <-pollCtx.Done():
			outcome.Duration = time.Since(startTime)
			if errors.Is(pollCtx.Err(), context.DeadlineExceeded) {
				return outcome, fmt.Errorf("%w: %s after %d attempts (%s)", ErrHealthCheckFailed, targetURL, attempts, outcome.Duration.Round(time.Millisecond))
			}
			return outcome, fmt.Errorf("health check cancelled: %w", pollCtx.Err())
		case <-ticker.C:
			// Continue next attempt
		}
	}
}
