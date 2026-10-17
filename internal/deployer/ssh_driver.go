package deployer

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/db"
)

var (
	// ErrRemotePathTraversal is returned when a path traversal attempt is detected.
	ErrRemotePathTraversal = errors.New("remote path traversal detected")

	// ErrInvalidServerID is returned when server_id is empty or missing for remote deployments.
	ErrInvalidServerID = errors.New("server id is required for remote deployment")
)

// RemoteSSHDriver implements DeployDriver for managing remote deployments via SSH / BYOS (Sprint 4).
type RemoteSSHDriver struct {
	serverRepo    db.ServerRepository
	keyVault      *crypto.KeyVault
	sshManager    SSHManager
	store         Store
	portPool      *PortPool
	router        Router
	storage       *StorageEngine
	remoteBaseDir string
	mu            sync.RWMutex
}

// NewRemoteSSHDriver creates a new instance of RemoteSSHDriver.
func NewRemoteSSHDriver(
	serverRepo db.ServerRepository,
	keyVault *crypto.KeyVault,
	sshManager SSHManager,
	store Store,
	portPool *PortPool,
	router Router,
) *RemoteSSHDriver {
	return &RemoteSSHDriver{
		serverRepo:    serverRepo,
		keyVault:      keyVault,
		sshManager:    sshManager,
		store:         store,
		portPool:      portPool,
		router:        router,
		remoteBaseDir: "cuan-apps",
	}
}

// SetStorage configures an optional storage engine for local cache/directory cleanup.
func (d *RemoteSSHDriver) SetStorage(s *StorageEngine) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.storage = s
}

// ValidateDeploymentID validates that the deployment ID is safe and contains no traversal sequences.
func ValidateDeploymentID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("deployment id cannot be empty")
	}

	// Reject path traversal tokens, slashes, backslashes, and control characters
	if strings.Contains(id, "..") || strings.Contains(id, "/") || strings.Contains(id, "\\") || strings.Contains(id, "\x00") {
		return fmt.Errorf("%w: deployment id contains illegal characters or path traversal: %q", ErrRemotePathTraversal, id)
	}

	// Strict character whitelist: alphanumeric, dash, underscore
	for _, ch := range id {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_') {
			return fmt.Errorf("%w: deployment id contains disallowed character: %c", ErrRemotePathTraversal, ch)
		}
	}

	return nil
}

// ValidateRemotePath ensures a remote relative path remains strictly confined inside ~/cuan-apps/<deployment_id>/
func ValidateRemotePath(deploymentID string, subpath ...string) (string, error) {
	if err := ValidateDeploymentID(deploymentID); err != nil {
		return "", err
	}

	base := "cuan-apps/" + deploymentID
	if len(subpath) == 0 || subpath[0] == "" {
		return base, nil
	}

	sub := subpath[0]
	if strings.Contains(sub, "..") || strings.Contains(sub, "\x00") || strings.Contains(sub, "\\") {
		return "", fmt.Errorf("%w: subpath contains traversal characters: %q", ErrRemotePathTraversal, sub)
	}

	cleanSub := path.Clean("/" + sub)
	cleanSub = strings.TrimPrefix(cleanSub, "/")
	if cleanSub == "" || cleanSub == "." {
		return base, nil
	}
	if strings.HasPrefix(cleanSub, "..") {
		return "", fmt.Errorf("%w: subpath escapes remote app directory: %q", ErrRemotePathTraversal, sub)
	}

	return path.Join(base, cleanSub), nil
}

// StreamZipAsTarToSSH converts an uploaded ZIP stream to a TAR archive on-the-fly and extracts it
// via SSH stream into ~/cuan-apps/<id>/ on the remote host, performing strict anti-ZipSlip verification.
func StreamZipAsTarToSSH(ctx context.Context, client *SSHClient, id string, zipReader io.Reader, zipSize int64) (string, error) {
	if err := ValidateDeploymentID(id); err != nil {
		return "", err
	}

	// Read zip entries
	var zr *zip.Reader
	if ra, ok := zipReader.(io.ReaderAt); ok && zipSize > 0 {
		var err error
		zr, err = zip.NewReader(ra, zipSize)
		if err != nil {
			return "", fmt.Errorf("read zip stream: %w", err)
		}
	} else {
		buf, err := io.ReadAll(zipReader)
		if err != nil {
			return "", fmt.Errorf("buffer zip stream: %w", err)
		}
		zr, err = zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
		if err != nil {
			return "", fmt.Errorf("parse zip archive: %w", err)
		}
	}

	// First pass: validate all entries against ZipSlip path traversal
	var detectedRuntime string
	for _, f := range zr.File {
		cleanName := filepath.Clean(f.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) || strings.Contains(f.Name, ":") || strings.Contains(f.Name, "\x00") {
			return "", fmt.Errorf("%w: %s", ErrZipSlip, f.Name)
		}
		base := filepath.Base(cleanName)
		if base == "index.php" && detectedRuntime == "" {
			detectedRuntime = RuntimePHP
		} else if base == "package.json" && (detectedRuntime == "" || detectedRuntime == RuntimePHP) {
			detectedRuntime = RuntimeNodeJS
		} else if base == "start.sh" {
			detectedRuntime = RuntimeCustom
		}
	}

	// Stream TAR archive directly to SSH command
	pr, pw := io.Pipe()
	go func() {
		tw := tar.NewWriter(pw)
		defer func() {
			_ = tw.Close()
			_ = pw.Close()
		}()

		for _, f := range zr.File {
			cleanName := filepath.ToSlash(filepath.Clean(f.Name))
			cleanName = strings.TrimPrefix(cleanName, "/")
			if cleanName == "" || cleanName == "." {
				continue
			}

			hdr := &tar.Header{
				Name:    cleanName,
				Mode:    int64(f.Mode().Perm()),
				Size:    int64(f.UncompressedSize64),
				ModTime: f.Modified,
			}
			if f.FileInfo().IsDir() {
				hdr.Typeflag = tar.TypeDir
				hdr.Mode = 0755
			} else {
				hdr.Typeflag = tar.TypeReg
				if hdr.Mode == 0 {
					hdr.Mode = 0644
				}
			}

			if err := tw.WriteHeader(hdr); err != nil {
				_ = pw.CloseWithError(err)
				return
			}

			if !f.FileInfo().IsDir() {
				rc, err := f.Open()
				if err != nil {
					_ = pw.CloseWithError(err)
					return
				}
				_, err = io.Copy(tw, rc)
				_ = rc.Close()
				if err != nil {
					_ = pw.CloseWithError(err)
					return
				}
			}
		}
	}()

	remoteDir := fmt.Sprintf("cuan-apps/%s", id)
	extractCmd := fmt.Sprintf("mkdir -p ~/%s && tar -xf - -C ~/%s", remoteDir, remoteDir)
	_, stderr, err := client.RunCommandWithInput(ctx, extractCmd, pr)
	if err != nil {
		return "", fmt.Errorf("remote tar extraction failed: %w (stderr: %s)", err, stderr)
	}

	return detectedRuntime, nil
}

// Deploy transfers project files via SFTP/SSH stream to ~/cuan-apps/<id>/, detects runtime,
// and persists deployment metadata in Store.
func (d *RemoteSSHDriver) Deploy(ctx context.Context, req DeploymentRequest) (*DeploymentResult, error) {
	if req.ProjectName == "" {
		return nil, fmt.Errorf("project_name is required")
	}

	if req.ServerID == "" {
		return nil, ErrInvalidServerID
	}

	if req.ID == "" {
		req.ID = generateDeploymentID()
	}

	if err := ValidateDeploymentID(req.ID); err != nil {
		return nil, err
	}

	if req.Slug == "" {
		req.Slug = slugify(req.ProjectName)
	}

	if req.SourceType == "" {
		req.SourceType = SourceTypeZip
	}

	// 1. Fetch remote server record
	srv, err := d.serverRepo.GetByID(ctx, req.ServerID)
	if err != nil {
		return nil, fmt.Errorf("lookup remote server %s: %w", req.ServerID, err)
	}

	// 2. Decrypt SSH credentials
	decryptedKey, err := d.keyVault.Decrypt(srv.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt server ssh key: %w", err)
	}

	// 3. Connect to remote host
	client, err := d.sshManager.GetClient(ctx, srv, decryptedKey)
	if err != nil {
		return nil, fmt.Errorf("connect to remote server: %w", err)
	}

	remoteDir := fmt.Sprintf("cuan-apps/%s", req.ID)
	detectedRuntime := req.Runtime

	// 4. Transfer bundle and extract remotely
	if req.ZipReader != nil {
		rt, err := StreamZipAsTarToSSH(ctx, client, req.ID, req.ZipReader, req.ZipSize)
		if err != nil {
			return nil, err
		}
		if rt != "" {
			detectedRuntime = rt
		}
	} else if req.SourceType == SourceTypeGit && req.SourceRef != "" {
		cloneCmd := fmt.Sprintf("rm -rf ~/%s && git clone --depth 1", remoteDir)
		if req.Branch != "" {
			cloneCmd += fmt.Sprintf(" -b %s", req.Branch)
		}
		cloneCmd += fmt.Sprintf(" %s ~/%s", req.SourceRef, remoteDir)

		_, stderr, err := client.RunCommand(ctx, cloneCmd)
		if err != nil {
			return nil, fmt.Errorf("remote git clone failed: %w (stderr: %s)", err, stderr)
		}

		detectCmd := fmt.Sprintf(`if [ -f ~/%s/start.sh ]; then echo "custom"; elif [ -f ~/%s/package.json ]; then echo "nodejs"; elif [ -f ~/%s/index.php ]; then echo "php"; else echo "static"; fi`, remoteDir, remoteDir, remoteDir)
		rtOut, _, _ := client.RunCommand(ctx, detectCmd)
		rt := strings.TrimSpace(rtOut)
		if rt != "" {
			detectedRuntime = rt
		}
	} else {
		// Just ensure remote application directory exists
		mkdirCmd := fmt.Sprintf("mkdir -p ~/%s", remoteDir)
		if _, stderr, err := client.RunCommand(ctx, mkdirCmd); err != nil {
			return nil, fmt.Errorf("create remote app directory: %w (stderr: %s)", err, stderr)
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
		ServerID:     req.ServerID,
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
			return nil, fmt.Errorf("update remote deployment record: %w", err)
		}
	} else {
		if err := d.store.CreateDeployment(ctx, dep); err != nil {
			return nil, fmt.Errorf("create remote deployment record: %w", err)
		}
	}

	// Record initial system log
	_ = d.store.AppendLog(ctx, &LogEntry{
		DeploymentID: dep.ID,
		Stream:       StreamSystem,
		Message:      fmt.Sprintf("Remote deployment %s initialized on server %s (%s) with runtime %s", dep.ProjectName, srv.Name, srv.Host, dep.Runtime),
		Timestamp:    time.Now().UTC(),
	})

	return &DeploymentResult{
		Deployment:      dep,
		DetectedRuntime: detectedRuntime,
		ExtractedPath:   fmt.Sprintf("~/cuan-apps/%s", req.ID),
		Status:          dep.Status,
	}, nil
}

// Start launches the background process on the remote host with dynamic port binding,
// tracking the remote PID in app.pid and persisting status to Store.
func (d *RemoteSSHDriver) Start(ctx context.Context, id string) error {
	if err := ValidateDeploymentID(id); err != nil {
		return err
	}

	dep, err := d.store.GetDeployment(ctx, id)
	if err != nil {
		return err
	}

	// 1. Fetch server & decrypt key
	srv, err := d.serverRepo.GetByID(ctx, dep.ServerID)
	if err != nil {
		return fmt.Errorf("lookup server %s: %w", dep.ServerID, err)
	}

	decryptedKey, err := d.keyVault.Decrypt(srv.EncryptedKey)
	if err != nil {
		return fmt.Errorf("decrypt server ssh key: %w", err)
	}

	client, err := d.sshManager.GetClient(ctx, srv, decryptedKey)
	if err != nil {
		return fmt.Errorf("connect to remote host: %w", err)
	}

	// Check if already running
	if dep.Status == StatusRunning && dep.ProcessPID > 0 {
		hs, _ := d.Health(ctx, id)
		if hs != nil && hs.Alive {
			return nil // already running and responsive
		}
		_ = d.Stop(ctx, id)
		dep, err = d.store.GetDeployment(ctx, id)
		if err != nil {
			return err
		}
	}

	// 2. Allocate dynamic port from pool if not set
	if dep.InternalPort <= 0 {
		port, err := d.portPool.Allocate(id)
		if err != nil {
			return fmt.Errorf("allocate dynamic port: %w", err)
		}
		dep.InternalPort = port
	}

	remoteDir := fmt.Sprintf("cuan-apps/%s", id)

	// 3. Build runner command
	var runCmd string
	switch dep.Runtime {
	case RuntimePHP:
		runCmd = fmt.Sprintf("php -S 0.0.0.0:%d -t ~/%s", dep.InternalPort, remoteDir)
	case RuntimeNodeJS:
		runCmd = fmt.Sprintf("if grep -q '\"start\":' ~/%s/package.json 2>/dev/null; then npm start; elif [ -f ~/%s/server.js ]; then node server.js; elif [ -f ~/%s/index.js ]; then node index.js; elif [ -f ~/%s/app.js ]; then node app.js; else node main.js; fi", remoteDir, remoteDir, remoteDir, remoteDir)
	case RuntimeStatic:
		runCmd = fmt.Sprintf("python3 -m http.server %d --bind 0.0.0.0 --directory ~/%s 2>/dev/null || php -S 0.0.0.0:%d -t ~/%s", dep.InternalPort, remoteDir, dep.InternalPort, remoteDir)
	case RuntimeCustom:
		runCmd = fmt.Sprintf("if [ -f ~/%s/start.sh ]; then bash start.sh; elif [ -f ~/%s/index.php ]; then php -S 0.0.0.0:%d -t ~/%s; elif [ -f ~/%s/package.json ]; then npm start; else python3 -m http.server %d --bind 0.0.0.0 --directory ~/%s; fi", remoteDir, remoteDir, dep.InternalPort, remoteDir, remoteDir, dep.InternalPort, remoteDir)
	default:
		runCmd = fmt.Sprintf("python3 -m http.server %d --bind 0.0.0.0 --directory ~/%s 2>/dev/null || php -S 0.0.0.0:%d -t ~/%s", dep.InternalPort, remoteDir, dep.InternalPort, remoteDir)
	}

	// 4. Build environment variables string
	envPrefix := fmt.Sprintf("PORT=%d CUAN_PORT=%d HOST=0.0.0.0", dep.InternalPort, dep.InternalPort)
	for k, v := range dep.EnvVars {
		escapedVal := strings.ReplaceAll(fmt.Sprintf("%v", v), "'", "'\\''")
		envPrefix += fmt.Sprintf(" %s='%s'", k, escapedVal)
	}

	// 5. Spawn background process on remote host and record PID
	spawnCmd := fmt.Sprintf("cd ~/%s && nohup env %s %s > app.log 2>&1 & echo $! > app.pid", remoteDir, envPrefix, runCmd)
	_, stderr, err := client.RunCommand(ctx, spawnCmd)
	if err != nil {
		d.portPool.Release(id)
		dep.Status = StatusFailed
		_ = d.store.UpdateDeployment(ctx, dep)
		_ = d.store.AppendLog(ctx, &LogEntry{
			DeploymentID: id,
			Stream:       StreamSystem,
			Message:      fmt.Sprintf("Failed to spawn remote process (%s): %v (stderr: %s)", dep.Runtime, err, stderr),
			Timestamp:    time.Now().UTC(),
		})
		return fmt.Errorf("spawn remote runner: %w (stderr: %s)", err, stderr)
	}

	// 6. Read spawned PID from remote app.pid
	pidOut, _, err := client.RunCommand(ctx, fmt.Sprintf("cat ~/%s/app.pid", remoteDir))
	if err != nil || strings.TrimSpace(pidOut) == "" {
		d.portPool.Release(id)
		return fmt.Errorf("read remote pid: %w", err)
	}

	pid, err := strconv.Atoi(strings.TrimSpace(pidOut))
	if err != nil || pid <= 0 {
		d.portPool.Release(id)
		return fmt.Errorf("invalid remote pid output %q: %w", pidOut, err)
	}

	dep.ProcessPID = pid
	dep.Status = StatusRunning
	dep.UpdatedAt = time.Now().UTC()

	if err := d.store.UpdateDeployment(ctx, dep); err != nil {
		d.portPool.Release(id)
		return fmt.Errorf("update deployment state: %w", err)
	}

	// 7. Register router route if configured
	if d.router != nil {
		domain := d.router.FormatDomain(dep.Slug)
		if dep.Domain != domain {
			dep.Domain = domain
			_ = d.store.UpdateDeployment(ctx, dep)
		}
		_ = d.router.RegisterRoute(ctx, dep.Slug, dep.InternalPort)
	}

	_ = d.store.AppendLog(ctx, &LogEntry{
		DeploymentID: id,
		Stream:       StreamSystem,
		Message:      fmt.Sprintf("Remote deployment %s started on host %s port %d (PID %d)", id, srv.Host, dep.InternalPort, pid),
		Timestamp:    time.Now().UTC(),
	})

	return nil
}

// Stop gracefully terminates the remote process (SIGTERM then SIGKILL), removes the remote PID file,
// releases the allocated port, and sets DB status to 'stopped'.
func (d *RemoteSSHDriver) Stop(ctx context.Context, id string) error {
	if err := ValidateDeploymentID(id); err != nil {
		return err
	}

	dep, err := d.store.GetDeployment(ctx, id)
	if err != nil {
		return err
	}

	srv, err := d.serverRepo.GetByID(ctx, dep.ServerID)
	if err == nil && srv != nil {
		decryptedKey, decErr := d.keyVault.Decrypt(srv.EncryptedKey)
		if decErr == nil {
			client, clientErr := d.sshManager.GetClient(ctx, srv, decryptedKey)
			if clientErr == nil {
				remoteDir := fmt.Sprintf("cuan-apps/%s", id)
				stopScript := fmt.Sprintf(`PID=$(cat ~/%s/app.pid 2>/dev/null || echo %d)
if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
    kill -15 "$PID" 2>/dev/null
    for i in $(seq 1 20); do
        if ! kill -0 "$PID" 2>/dev/null; then
            break
        fi
        sleep 0.1
    done
    if kill -0 "$PID" 2>/dev/null; then
        kill -9 "$PID" 2>/dev/null
    fi
fi
rm -f ~/%s/app.pid`, remoteDir, dep.ProcessPID, remoteDir)

				_, _, _ = client.RunCommand(ctx, stopScript)
			}
		}
	}

	// Release port lease
	d.portPool.Release(id)

	// Update DB record
	dep.Status = StatusStopped
	dep.ProcessPID = 0
	dep.UpdatedAt = time.Now().UTC()
	if err := d.store.UpdateDeployment(ctx, dep); err != nil {
		return err
	}

	if d.router != nil {
		_ = d.router.UnregisterRoute(ctx, dep.Slug)
	}

	_ = d.store.AppendLog(ctx, &LogEntry{
		DeploymentID: id,
		Stream:       StreamSystem,
		Message:      fmt.Sprintf("Remote deployment %s stopped cleanly", id),
		Timestamp:    time.Now().UTC(),
	})

	return nil
}

// Restart stops and restarts the remote deployment process.
func (d *RemoteSSHDriver) Restart(ctx context.Context, id string) error {
	if err := d.Stop(ctx, id); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	return d.Start(ctx, id)
}

// Health probes process liveness on the remote server and verifies whether the leased port responds.
func (d *RemoteSSHDriver) Health(ctx context.Context, id string) (*HealthStatus, error) {
	if err := ValidateDeploymentID(id); err != nil {
		return nil, err
	}

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

	srv, err := d.serverRepo.GetByID(ctx, dep.ServerID)
	if err != nil {
		return nil, fmt.Errorf("lookup server %s: %w", dep.ServerID, err)
	}

	decryptedKey, err := d.keyVault.Decrypt(srv.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt server ssh key: %w", err)
	}

	client, err := d.sshManager.GetClient(ctx, srv, decryptedKey)
	if err != nil {
		return nil, fmt.Errorf("connect to remote host: %w", err)
	}

	// 1. Probe PID liveness via kill -0
	probeCmd := fmt.Sprintf("kill -0 %d 2>/dev/null && echo ALIVE || echo DEAD", dep.ProcessPID)
	stdout, _, err := client.RunCommand(ctx, probeCmd)
	if err != nil || strings.TrimSpace(stdout) != "ALIVE" {
		return &HealthStatus{
			Alive:          false,
			PID:            dep.ProcessPID,
			Port:           dep.InternalPort,
			PortResponding: false,
			Error:          fmt.Sprintf("remote process PID %d is not responding", dep.ProcessPID),
		}, nil
	}

	// 2. Check port responsiveness
	portResponding := false
	if dep.InternalPort > 0 {
		portProbeCmd := fmt.Sprintf("nc -z 127.0.0.1 %d 2>/dev/null || (echo > /dev/tcp/127.0.0.1/%d) 2>/dev/null && echo OPEN || echo CLOSED", dep.InternalPort, dep.InternalPort)
		portOut, _, _ := client.RunCommand(ctx, portProbeCmd)
		if strings.TrimSpace(portOut) == "OPEN" {
			portResponding = true
		} else {
			// Direct TCP fallback
			addr := fmt.Sprintf("%s:%d", srv.Host, dep.InternalPort)
			conn, dialErr := net.DialTimeout("tcp", addr, 300*time.Millisecond)
			if dialErr == nil {
				portResponding = true
				_ = conn.Close()
			}
		}
	}

	return &HealthStatus{
		Alive:          true,
		PID:            dep.ProcessPID,
		Port:           dep.InternalPort,
		PortResponding: portResponding,
	}, nil
}

// GetLogs retrieves recent log entries for the deployment from Store.
func (d *RemoteSSHDriver) GetLogs(ctx context.Context, id string, limit int) ([]LogEntry, error) {
	if err := ValidateDeploymentID(id); err != nil {
		return nil, err
	}
	return d.store.GetLogs(ctx, id, limit)
}

// List retrieves all deployment records from the Store.
func (d *RemoteSSHDriver) List(ctx context.Context) ([]Deployment, error) {
	return d.store.ListDeployments(ctx)
}

// Get retrieves an individual deployment record by unique ID.
func (d *RemoteSSHDriver) Get(ctx context.Context, id string) (*Deployment, error) {
	if err := ValidateDeploymentID(id); err != nil {
		return nil, err
	}
	return d.store.GetDeployment(ctx, id)
}

// Delete cleanly terminates any active process on the remote host, removes the remote app directory ~/cuan-apps/<id>/,
// revokes dynamic routes, releases port lease, and removes the deployment from Store.
func (d *RemoteSSHDriver) Delete(ctx context.Context, id string) error {
	if err := ValidateDeploymentID(id); err != nil {
		return err
	}

	dep, err := d.store.GetDeployment(ctx, id)
	if err != nil {
		return err
	}

	// 1. Stop active process and release port
	_ = d.Stop(ctx, id)

	// 2. Remove remote app directory
	srv, err := d.serverRepo.GetByID(ctx, dep.ServerID)
	if err == nil && srv != nil {
		decryptedKey, decErr := d.keyVault.Decrypt(srv.EncryptedKey)
		if decErr == nil {
			client, clientErr := d.sshManager.GetClient(ctx, srv, decryptedKey)
			if clientErr == nil {
				remoteDir := fmt.Sprintf("cuan-apps/%s", id)
				_, _, _ = client.RunCommand(ctx, fmt.Sprintf("rm -rf ~/%s", remoteDir))
			}
		}
	}

	// 3. Unregister dynamic virtual host route
	if d.router != nil {
		_ = d.router.UnregisterRoute(ctx, dep.Slug)
	}

	// 4. Clean local cache storage if configured
	if d.storage != nil {
		_ = d.storage.DeleteDeploymentDir(id)
	}

	// 5. Delete deployment record from Store
	return d.store.DeleteDeployment(ctx, id)
}

// Interface compliance check
var _ DeployDriver = (*RemoteSSHDriver)(nil)
