package deployer

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ManagedProcess tracks a live running process instance managed by LocalDriver.
type ManagedProcess struct {
	DeploymentID string
	Cmd          *exec.Cmd
	Port         int
	Runtime      string
	PID          int
	Streamer     *LogStreamer
	Done         chan struct{}
	ExitErr      error
	Stopped      bool
	mu           sync.Mutex
}

// LocalDriver implements DeployDriver for local/co-located process execution (Sprint 3).
type LocalDriver struct {
	storage         *StorageEngine
	store           Store
	portPool        *PortPool
	router          Router
	staticBinPath   string
	activeProcesses map[string]*ManagedProcess
	mu              sync.RWMutex
}

// NewLocalDriver instantiates a LocalDriver with storage and persistent store.
func NewLocalDriver(storage *StorageEngine, store Store) *LocalDriver {
	return NewLocalDriverWithRouter(storage, store, NewPortPool(), NewCaddyRouter(DefaultCaddyConfig()))
}

// NewLocalDriverWithPortPool instantiates a LocalDriver with a custom PortPool.
func NewLocalDriverWithPortPool(storage *StorageEngine, store Store, pool *PortPool) *LocalDriver {
	return NewLocalDriverWithRouter(storage, store, pool, NewCaddyRouter(DefaultCaddyConfig()))
}

// NewLocalDriverWithRouter instantiates a LocalDriver with custom PortPool and dynamic Router.
func NewLocalDriverWithRouter(storage *StorageEngine, store Store, pool *PortPool, router Router) *LocalDriver {
	return &LocalDriver{
		storage:         storage,
		store:           store,
		portPool:        pool,
		router:          router,
		activeProcesses: make(map[string]*ManagedProcess),
	}
}

// Storage returns the underlying StorageEngine.
func (d *LocalDriver) Storage() *StorageEngine {
	return d.storage
}

// Store returns the underlying Store.
func (d *LocalDriver) Store() Store {
	return d.store
}

// PortPool returns the underlying PortPool.
func (d *LocalDriver) PortPool() *PortPool {
	return d.portPool
}

// Router returns the underlying dynamic virtual host Router engine.
func (d *LocalDriver) Router() Router {
	return d.router
}

// GetStreamer returns the active LogStreamer for a deployment if currently running.
func (d *LocalDriver) GetStreamer(id string) *LogStreamer {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if proc, exists := d.activeProcesses[id]; exists {
		return proc.Streamer
	}
	return nil
}

// Deploy extracts/stores project files, runs auto-detection, and persists deployment metadata.
func (d *LocalDriver) Deploy(ctx context.Context, req DeploymentRequest) (*DeploymentResult, error) {
	if req.ProjectName == "" {
		return nil, fmt.Errorf("project_name is required")
	}

	if req.ID == "" {
		req.ID = generateDeploymentID()
	}

	if req.Slug == "" {
		req.Slug = slugify(req.ProjectName)
	}

	if req.SourceType == "" {
		req.SourceType = SourceTypeZip
	}

	appDir := d.storage.GetAppDir(req.ID)
	detectedRuntime := req.Runtime

	// Process ZIP archive if supplied
	if req.ZipReader != nil {
		var readerAt io.ReaderAt
		var size int64

		if ra, ok := req.ZipReader.(io.ReaderAt); ok && req.ZipSize > 0 {
			readerAt = ra
			size = req.ZipSize
		} else {
			// Buffer into memory
			buf, err := io.ReadAll(req.ZipReader)
			if err != nil {
				return nil, fmt.Errorf("read uploaded zip stream: %w", err)
			}
			readerAt = bytes.NewReader(buf)
			size = int64(len(buf))
		}

		rt, extracted, err := d.storage.ExtractArchive(req.ID, readerAt, size)
		if err != nil {
			return nil, err
		}
		detectedRuntime = rt
		appDir = extracted
	} else if req.SourceType == SourceTypeGit && req.SourceRef != "" {
		appDir = d.storage.GetAppDir(req.ID)
		if err := cloneGitRepository(ctx, req.SourceRef, req.Branch, appDir); err != nil {
			return nil, err
		}
		rt, err := DetectRuntime(appDir)
		if err == nil && rt != "" {
			detectedRuntime = rt
		}
	} else {
		// Just ensure app directory exists
		if _, err := d.storage.PrepareDeploymentDir(req.ID); err != nil {
			return nil, err
		}
		rt, err := DetectRuntime(appDir)
		if err == nil && rt != "" {
			detectedRuntime = rt
		}
	}

	if req.Runtime == "" || req.Runtime == RuntimeCustom {
		if detectedRuntime != "" {
			req.Runtime = detectedRuntime
		} else {
			req.Runtime = RuntimeCustom
		}
	}

	if req.Domain == "" && d.router != nil {
		req.Domain = d.router.FormatDomain(req.Slug)
	}

	dep := &Deployment{
		ID:           req.ID,
		ProjectName:  req.ProjectName,
		Slug:         req.Slug,
		SourceType:   req.SourceType,
		SourceRef:    req.SourceRef,
		Runtime:      req.Runtime,
		Status:       StatusPending,
		InternalPort: req.InternalPort,
		Domain:       req.Domain,
		EnvVars:      req.EnvVars,
		ProcessPID:   0,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	// Persist to store
	existing, err := d.store.GetDeployment(ctx, req.ID)
	if err == nil && existing != nil {
		dep.CreatedAt = existing.CreatedAt
		if err := d.store.UpdateDeployment(ctx, dep); err != nil {
			return nil, fmt.Errorf("update deployment: %w", err)
		}
	} else {
		if err := d.store.CreateDeployment(ctx, dep); err != nil {
			return nil, fmt.Errorf("create deployment: %w", err)
		}
	}

	// Record initial system log
	_ = d.store.AppendLog(ctx, &LogEntry{
		DeploymentID: dep.ID,
		Stream:       StreamSystem,
		Message:      fmt.Sprintf("Deployment %s (%s) initialized with runtime %s", dep.ProjectName, dep.ID, dep.Runtime),
		Timestamp:    time.Now().UTC(),
	})

	return &DeploymentResult{
		Deployment:      dep,
		DetectedRuntime: detectedRuntime,
		ExtractedPath:   appDir,
		Status:          dep.Status,
	}, nil
}

// Start dynamically allocates a port, spawns the sandboxed child process,
// records PID and port to Store, and starts real-time log streaming.
func (d *LocalDriver) Start(ctx context.Context, id string) error {
	dep, err := d.store.GetDeployment(ctx, id)
	if err != nil {
		return err
	}

	// If already marked running, verify health
	if dep.Status == StatusRunning && dep.ProcessPID > 0 {
		hs, _ := d.Health(ctx, id)
		if hs != nil && hs.Alive {
			return nil // already running and responsive
		}
		// Clean up dead previous process
		_ = d.Stop(ctx, id)
		dep, err = d.store.GetDeployment(ctx, id)
		if err != nil {
			return err
		}
	}

	// 1. Allocate dynamic port from pool (9000-9999)
	port, err := d.portPool.Allocate(id)
	if err != nil {
		return fmt.Errorf("allocate dynamic port: %w", err)
	}
	dep.InternalPort = port

	appDir := d.storage.GetAppDir(id)
	if !DirExists(appDir) {
		if _, err := d.storage.PrepareDeploymentDir(id); err != nil {
			d.portPool.Release(id)
			return fmt.Errorf("prepare app directory: %w", err)
		}
	}

	// 2. Build runtime command (PHP, Node.js, Static)
	cmd, err := d.buildProcessCommand(dep.Runtime, appDir, port, dep.EnvVars)
	if err != nil {
		d.portPool.Release(id)
		return fmt.Errorf("build process command: %w", err)
	}

	// 3. Attach stdout and stderr pipes for LogStreamer
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		d.portPool.Release(id)
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		d.portPool.Release(id)
		return fmt.Errorf("stderr pipe: %w", err)
	}

	// Set process group for clean subtree termination
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// 4. Spawn child process
	if err := cmd.Start(); err != nil {
		d.portPool.Release(id)
		dep.Status = StatusFailed
		_ = d.store.UpdateDeployment(ctx, dep)
		_ = d.store.AppendLog(ctx, &LogEntry{
			DeploymentID: id,
			Stream:       StreamSystem,
			Message:      fmt.Sprintf("Failed to spawn process (%s): %v", dep.Runtime, err),
			Timestamp:    time.Now().UTC(),
		})
		return fmt.Errorf("start process (%s): %w", dep.Runtime, err)
	}

	dep.ProcessPID = cmd.Process.Pid
	dep.Status = StatusRunning
	dep.UpdatedAt = time.Now().UTC()

	// Update deployment record in DB
	if err := d.store.UpdateDeployment(ctx, dep); err != nil {
		_ = cmd.Process.Kill()
		d.portPool.Release(id)
		return fmt.Errorf("update deployment record: %w", err)
	}

	// 5. Initialize LogStreamer and register active process
	streamer := NewLogStreamer(id, d.store, 1000)
	proc := &ManagedProcess{
		DeploymentID: id,
		Cmd:          cmd,
		Port:         port,
		Runtime:      dep.Runtime,
		PID:          cmd.Process.Pid,
		Streamer:     streamer,
		Done:         make(chan struct{}),
	}

	d.mu.Lock()
	d.activeProcesses[id] = proc
	d.mu.Unlock()

	// 6. Launch background log streamer goroutines
	go func() {
		_ = streamer.Stream(context.Background(), StreamStdout, stdoutPipe)
	}()
	go func() {
		_ = streamer.Stream(context.Background(), StreamStderr, stderrPipe)
	}()

	// 7. Launch background exit watcher
	go func() {
		waitErr := cmd.Wait()
		proc.mu.Lock()
		proc.ExitErr = waitErr
		wasStopped := proc.Stopped
		proc.mu.Unlock()
		close(proc.Done)

		if !wasStopped {
			// Process exited unexpectedly or crashed
			d.mu.Lock()
			delete(d.activeProcesses, id)
			d.mu.Unlock()
			d.portPool.Release(id)

			_ = d.store.AppendLog(context.Background(), &LogEntry{
				DeploymentID: id,
				Stream:       StreamSystem,
				Message:      fmt.Sprintf("Process exited unexpectedly (PID %d): %v", proc.PID, waitErr),
				Timestamp:    time.Now().UTC(),
			})

			current, err := d.store.GetDeployment(context.Background(), id)
			if err == nil && current.Status == StatusRunning {
				current.Status = StatusStopped
				current.ProcessPID = 0
				_ = d.store.UpdateDeployment(context.Background(), current)
			}
		}
	}()

	// 8. Record system start log
	_ = d.store.AppendLog(ctx, &LogEntry{
		DeploymentID: id,
		Stream:       StreamSystem,
		Message:      fmt.Sprintf("Deployment %s started on port %d (PID %d)", id, port, dep.ProcessPID),
		Timestamp:    time.Now().UTC(),
	})

	// Wait briefly for process to bind socket
	waitForSocket(port, 2*time.Second)

	// 9. Register dynamic virtual host route in Caddy
	if d.router != nil {
		domain := d.router.FormatDomain(dep.Slug)
		if dep.Domain != domain {
			dep.Domain = domain
			_ = d.store.UpdateDeployment(ctx, dep)
		}
		if err := d.router.RegisterRoute(ctx, dep.Slug, port); err != nil {
			_ = d.store.AppendLog(ctx, &LogEntry{
				DeploymentID: id,
				Stream:       StreamSystem,
				Message:      fmt.Sprintf("Warning: could not register dynamic Caddy route for %s: %v", dep.Slug, err),
				Timestamp:    time.Now().UTC(),
			})
		} else {
			_ = d.store.AppendLog(ctx, &LogEntry{
				DeploymentID: id,
				Stream:       StreamSystem,
				Message:      fmt.Sprintf("Dynamic Caddy route registered: %s -> 127.0.0.1:%d", domain, port),
				Timestamp:    time.Now().UTC(),
			})
		}
	}

	return nil
}

// Stop cleanly terminates the active deployment process (graceful SIGTERM -> SIGKILL),
// releases the allocated port back to the pool, and updates DB status to 'stopped'.
func (d *LocalDriver) Stop(ctx context.Context, id string) error {
	dep, err := d.store.GetDeployment(ctx, id)
	if err != nil {
		return err
	}

	d.mu.Lock()
	proc, exists := d.activeProcesses[id]
	if exists {
		delete(d.activeProcesses, id)
	}
	d.mu.Unlock()

	if proc != nil {
		proc.mu.Lock()
		proc.Stopped = true
		proc.mu.Unlock()

		if proc.Cmd != nil && proc.Cmd.Process != nil {
			pid := proc.Cmd.Process.Pid

			// Graceful termination: SIGTERM to process group
			pgid, pgidErr := syscall.Getpgid(pid)
			if pgidErr == nil {
				_ = syscall.Kill(-pgid, syscall.SIGTERM)
			} else {
				_ = proc.Cmd.Process.Signal(syscall.SIGTERM)
			}

			// Wait up to 2 seconds for graceful exit, then force SIGKILL
			select {
			case <-proc.Done:
				// Process terminated cleanly
			case <-time.After(2 * time.Second):
				if pgidErr == nil {
					_ = syscall.Kill(-pgid, syscall.SIGKILL)
				} else {
					_ = proc.Cmd.Process.Kill()
				}
				select {
				case <-proc.Done:
				case <-time.After(500 * time.Millisecond):
				}
			}
		}

		if proc.Streamer != nil {
			proc.Streamer.Close()
		}
	} else if dep.ProcessPID > 0 {
		// Process is not in local memory (e.g. following daemon restart) but recorded in DB
		if err := syscall.Kill(dep.ProcessPID, 0); err == nil {
			_ = syscall.Kill(dep.ProcessPID, syscall.SIGTERM)
			time.Sleep(200 * time.Millisecond)
			if err := syscall.Kill(dep.ProcessPID, 0); err == nil {
				_ = syscall.Kill(dep.ProcessPID, syscall.SIGKILL)
			}
		}
	}

	// Free port lease
	d.portPool.Release(id)

	// Update DB state
	dep.Status = StatusStopped
	dep.ProcessPID = 0
	dep.UpdatedAt = time.Now().UTC()
	if err := d.store.UpdateDeployment(ctx, dep); err != nil {
		return err
	}

	// Revoke dynamic virtual host route in Caddy
	if d.router != nil {
		if err := d.router.UnregisterRoute(ctx, dep.Slug); err != nil {
			_ = d.store.AppendLog(ctx, &LogEntry{
				DeploymentID: id,
				Stream:       StreamSystem,
				Message:      fmt.Sprintf("Warning: could not unregister dynamic Caddy route for %s: %v", dep.Slug, err),
				Timestamp:    time.Now().UTC(),
			})
		} else {
			_ = d.store.AppendLog(ctx, &LogEntry{
				DeploymentID: id,
				Stream:       StreamSystem,
				Message:      fmt.Sprintf("Dynamic Caddy route revoked for %s", dep.Slug),
				Timestamp:    time.Now().UTC(),
			})
		}
	}

	_ = d.store.AppendLog(ctx, &LogEntry{
		DeploymentID: id,
		Stream:       StreamSystem,
		Message:      fmt.Sprintf("Deployment %s stopped cleanly", id),
		Timestamp:    time.Now().UTC(),
	})

	return nil
}

// Restart stops and restarts the deployment process.
func (d *LocalDriver) Restart(ctx context.Context, id string) error {
	if err := d.Stop(ctx, id); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	return d.Start(ctx, id)
}

// Delete cleanly terminates any running process, revokes virtual host routes, releases port, and removes stored resources.
func (d *LocalDriver) Delete(ctx context.Context, id string) error {
	dep, err := d.store.GetDeployment(ctx, id)
	if err != nil {
		return err
	}

	// Stop any active process and release port
	_ = d.Stop(ctx, id)

	// Ensure dynamic route is revoked
	if d.router != nil {
		_ = d.router.UnregisterRoute(ctx, dep.Slug)
	}

	// Purge local storage directory
	_ = d.storage.DeleteDeploymentDir(id)

	// Delete record from store
	return d.store.DeleteDeployment(ctx, id)
}

// Health checks if the deployment process is currently alive via PID signal probing
// and verifies that the leased port responds to TCP connections.
func (d *LocalDriver) Health(ctx context.Context, id string) (*HealthStatus, error) {
	dep, err := d.store.GetDeployment(ctx, id)
	if err != nil {
		return nil, err
	}

	if dep.Status != StatusRunning || dep.ProcessPID <= 0 {
		return &HealthStatus{
			Alive:          false,
			PID:            dep.ProcessPID,
			Port:           dep.InternalPort,
			PortResponding: false,
			Error:          "deployment is not running",
		}, nil
	}

	// 1. Probe PID liveness using kill -0
	if err := syscall.Kill(dep.ProcessPID, 0); err != nil {
		return &HealthStatus{
			Alive:          false,
			PID:            dep.ProcessPID,
			Port:           dep.InternalPort,
			PortResponding: false,
			Error:          fmt.Sprintf("process PID %d is dead: %v", dep.ProcessPID, err),
		}, nil
	}

	// 2. Ping internal TCP port
	portResponding := false
	if dep.InternalPort > 0 {
		addr := fmt.Sprintf("127.0.0.1:%d", dep.InternalPort)
		conn, dialErr := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if dialErr == nil {
			portResponding = true
			_ = conn.Close()
		}
	}

	return &HealthStatus{
		Alive:          true,
		PID:            dep.ProcessPID,
		Port:           dep.InternalPort,
		PortResponding: portResponding,
	}, nil
}

// HealthCheck is an alias for Health to support alternate naming conventions.
func (d *LocalDriver) HealthCheck(ctx context.Context, id string) (*HealthStatus, error) {
	return d.Health(ctx, id)
}

// GetLogs fetches recent logs from the database store.
func (d *LocalDriver) GetLogs(ctx context.Context, id string, limit int) ([]LogEntry, error) {
	return d.store.GetLogs(ctx, id, limit)
}

// buildProcessCommand prepares the exec.Cmd instance for the given runtime profile.
func (d *LocalDriver) buildProcessCommand(runtime, appDir string, port int, envVars map[string]interface{}) (*exec.Cmd, error) {
	var cmd *exec.Cmd

	switch runtime {
	case RuntimePHP:
		phpBin, err := exec.LookPath("php")
		if err != nil {
			return nil, fmt.Errorf("php runtime binary not found in PATH: %w", err)
		}
		// php -S 127.0.0.1:{port} -t {appDir}
		cmd = exec.Command(phpBin, "-S", fmt.Sprintf("127.0.0.1:%d", port), "-t", appDir)

	case RuntimeNodeJS:
		nodeBin, err := exec.LookPath("node")
		if err != nil {
			return nil, fmt.Errorf("node runtime binary not found in PATH: %w", err)
		}

		entryFile := resolveNodeEntry(appDir)
		if entryFile == "npm:start" {
			npmBin, npmErr := exec.LookPath("npm")
			if npmErr != nil {
				return nil, fmt.Errorf("npm binary not found in PATH: %w", npmErr)
			}
			cmd = exec.Command(npmBin, "start")
		} else {
			cmd = exec.Command(nodeBin, entryFile)
		}

	case RuntimeStatic:
		var err error
		cmd, err = d.buildStaticServerCommand(appDir, port)
		if err != nil {
			return nil, fmt.Errorf("static server runner: %w", err)
		}

	case RuntimeCustom:
		// Check for custom start script
		startScript := filepath.Join(appDir, "start.sh")
		if FileExists(startScript) {
			cmd = exec.Command("bash", "start.sh")
		} else {
			// Auto-fallback based on detected file types
			if FileExists(filepath.Join(appDir, "index.php")) {
				return d.buildProcessCommand(RuntimePHP, appDir, port, envVars)
			}
			if FileExists(filepath.Join(appDir, "package.json")) {
				return d.buildProcessCommand(RuntimeNodeJS, appDir, port, envVars)
			}
			return d.buildProcessCommand(RuntimeStatic, appDir, port, envVars)
		}

	default:
		return d.buildProcessCommand(RuntimeStatic, appDir, port, envVars)
	}

	cmd.Dir = appDir

	// Prepare environment variables
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("PORT=%d", port))
	cmd.Env = append(cmd.Env, "HOST=127.0.0.1")
	cmd.Env = append(cmd.Env, fmt.Sprintf("CUAN_PORT=%d", port))

	for k, v := range envVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%v", k, v))
	}

	return cmd, nil
}

// buildStaticServerCommand returns an exec.Cmd that serves static files on the allocated port.
// It prefers the Go static-server binary, falling back to PHP built-in or Python 3.
func (d *LocalDriver) buildStaticServerCommand(appDir string, port int) (*exec.Cmd, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 1. Check if cached binary is valid
	if d.staticBinPath != "" && FileExists(d.staticBinPath) {
		return exec.Command(d.staticBinPath, "-port", fmt.Sprintf("%d", port), "-dir", appDir), nil
	}

	// 2. Check /tmp/cuan-static-server
	tmpBin := filepath.Join(os.TempDir(), "cuan-static-server")
	if FileExists(tmpBin) {
		d.staticBinPath = tmpBin
		return exec.Command(d.staticBinPath, "-port", fmt.Sprintf("%d", port), "-dir", appDir), nil
	}

	// 3. Attempt to compile cmd/static-server if Go is available
	goBin, err := exec.LookPath("go")
	if err == nil {
		buildCmd := exec.Command(goBin, "build", "-o", tmpBin, "./cmd/static-server")
		if err := buildCmd.Run(); err == nil && FileExists(tmpBin) {
			d.staticBinPath = tmpBin
			return exec.Command(d.staticBinPath, "-port", fmt.Sprintf("%d", port), "-dir", appDir), nil
		}
	}

	// 4. Fallback: PHP built-in web server
	if phpBin, err := exec.LookPath("php"); err == nil {
		return exec.Command(phpBin, "-S", fmt.Sprintf("127.0.0.1:%d", port), "-t", appDir), nil
	}

	// 5. Fallback: Python 3 http.server
	if pyBin, err := exec.LookPath("python3"); err == nil {
		return exec.Command(pyBin, "-m", "http.server", fmt.Sprintf("%d", port), "--bind", "127.0.0.1", "--directory", appDir), nil
	}

	return nil, fmt.Errorf("no suitable static server runtime found (go, php, or python3 required)")
}

// resolveNodeEntry determines the primary script file for a Node.js project.
func resolveNodeEntry(appDir string) string {
	candidates := []string{"server.js", "index.js", "app.js", "main.js"}
	for _, c := range candidates {
		if FileExists(filepath.Join(appDir, c)) {
			return c
		}
	}

	// Inspect package.json
	pkgPath := filepath.Join(appDir, "package.json")
	if FileExists(pkgPath) {
		data, err := os.ReadFile(pkgPath)
		if err == nil {
			var pkg struct {
				Main    string            `json:"main"`
				Scripts map[string]string `json:"scripts"`
			}
			if err := json.Unmarshal(data, &pkg); err == nil {
				if pkg.Main != "" && FileExists(filepath.Join(appDir, pkg.Main)) {
					return pkg.Main
				}
				if pkg.Scripts != nil && pkg.Scripts["start"] != "" {
					return "npm:start"
				}
			}
		}
	}

	return "index.js"
}

func waitForSocket(port int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(30*time.Millisecond)
	}
}

func generateDeploymentID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("dep_%s", hex.EncodeToString(b))
}

var nonAlphaNumRegex = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	slug := nonAlphaNumRegex.ReplaceAllString(lower, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "app-" + generateDeploymentID()[:8]
	}
	return slug
}

// Helper functions for path verification
func PathExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func DirExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func FileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// List retrieves all deployments from the underlying store.
func (d *LocalDriver) List(ctx context.Context) ([]Deployment, error) {
	return d.store.ListDeployments(ctx)
}

// Get retrieves an individual deployment record by its ID.
func (d *LocalDriver) Get(ctx context.Context, id string) (*Deployment, error) {
	return d.store.GetDeployment(ctx, id)
}

// cloneGitRepository performs a shallow git clone into targetDir.
func cloneGitRepository(ctx context.Context, repoURL, branch, targetDir string) error {
	_ = os.RemoveAll(targetDir)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	gitBin, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git binary not found in PATH: %w", err)
	}

	actualURL := repoURL
	actualBranch := branch
	if actualBranch == "" && strings.Contains(actualURL, "#") {
		parts := strings.SplitN(actualURL, "#", 2)
		actualURL = parts[0]
		actualBranch = parts[1]
	}

	args := []string{"clone", "--depth", "1"}
	if actualBranch != "" {
		args = append(args, "-b", actualBranch)
	}
	args = append(args, actualURL, targetDir)

	cmd := exec.CommandContext(ctx, gitBin, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w (output: %s)", err, out.String())
	}
	return nil
}

// Ensure interface compliance at compile-time
var _ DeployDriver = (*LocalDriver)(nil)

