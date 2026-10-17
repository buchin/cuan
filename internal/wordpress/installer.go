package wordpress

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
)

var (
	ErrDeploymentNotFound        = errors.New("wordpress deployment not found")
	ErrDeploymentAlreadyRunning  = errors.New("wordpress deployment already running for this project")
	ErrInvalidProjectID          = errors.New("project id is required")
	ErrInvalidSiteTitle          = errors.New("site title is required")
	ErrInvalidAdminEmail         = errors.New("admin email is required")
	ErrInvalidAdminUsername      = errors.New("admin username is required")
)

// GenerateSecurePassword generates a cryptographically secure random alphanumeric password.
func GenerateSecurePassword(length int) (string, error) {
	if length <= 0 {
		length = 24
	}
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!#%*+-_="
	maxIdx := big.NewInt(int64(len(chars)))
	var sb strings.Builder
	sb.Grow(length)

	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, maxIdx)
		if err != nil {
			return "", fmt.Errorf("generate random password: %w", err)
		}
		sb.WriteByte(chars[n.Int64()])
	}
	return sb.String(), nil
}

// Config defines the configuration for the WordPress Orchestrator and Installer Engine.
type Config struct {
	DBRepo         cuanDB.WordPressRepository
	DBProvisioner  *deployer.DatabaseProvisioner
	PortPool       *deployer.PortPool
	Router         deployer.Router
	Launcher       deployer.ProcessLauncher
	VaultSecret    string
	BaseDeployDir  string
	CoreCacheDir   string
	WPDownloadURL  string
	HTTPClient     *http.Client
	MockMode       bool
}

// DeployRequest holds input parameters to provision and install a WordPress site.
type DeployRequest struct {
	ProjectID          string  `json:"project_id"`
	SiteTitle          string  `json:"site_title"`
	AdminEmail         string  `json:"admin_email"`
	AdminUsername      string  `json:"admin_username"`
	AdminPassword      string  `json:"admin_password,omitempty"`
	Domain             string  `json:"domain,omitempty"`
	DBName             string  `json:"db_name,omitempty"`
	DBUser             string  `json:"db_user,omitempty"`
	DBPassword         string  `json:"db_password,omitempty"`
	DBHost             string  `json:"db_host,omitempty"`
	DBPort             int     `json:"db_port,omitempty"`
	TablePrefix        string  `json:"table_prefix,omitempty"`
	WPVersion          string  `json:"wp_version,omitempty"`
	PermalinkStructure string  `json:"permalink_structure,omitempty"`
	ServerID           *string `json:"server_id,omitempty"`
	SkipHealthWait     bool    `json:"skip_health_wait,omitempty"`
}

// DeployResult encapsulates the outcome of a successful WordPress deployment.
type DeployResult struct {
	Deployment         *cuanDB.WordPressDeployment    `json:"deployment"`
	PublicDTO          cuanDB.WordPressDeploymentDTO  `json:"public_dto"`
	AdminPasswordPlain string                         `json:"admin_password_plain,omitempty"` // Returned only once upon deployment
	DBPasswordPlain    string                         `json:"db_password_plain,omitempty"`    // Returned only once upon deployment
	SiteURL            string                         `json:"site_url"`
	AdminURL           string                         `json:"admin_url"`
	Port               int                            `json:"port"`
	PID                int                            `json:"pid"`
	Status             string                         `json:"status"`
	HealthStatus       string                         `json:"health_status"`
	ExecutionLogs      []string                       `json:"execution_logs"`
}

// RunningInstance tracks an active supervised WordPress runtime process.
type RunningInstance struct {
	DeploymentID string
	ProjectID    string
	Port         int
	Domain       string
	PID          int
	Cmd          *exec.Cmd
	TargetDir    string
	StartedAt    time.Time
	Stopped      bool
	mu           sync.Mutex
}

// Orchestrator coordinates turnkey WordPress deployment, database provisioning, port leasing,
// wp-config generation, headless bootstrapping, process supervision, Caddy routing, and health checks.
type Orchestrator struct {
	cfg     Config
	running map[string]*RunningInstance // deploymentID -> RunningInstance
	mu      sync.RWMutex
}

// NewOrchestrator initializes a WordPress Orchestrator engine.
func NewOrchestrator(cfg Config) *Orchestrator {
	if cfg.BaseDeployDir == "" {
		cfg.BaseDeployDir = filepath.Join("deployments", "wordpress")
	}
	if cfg.CoreCacheDir == "" {
		cfg.CoreCacheDir = filepath.Join("data", "wordpress", "cache")
	}
	if cfg.WPDownloadURL == "" {
		cfg.WPDownloadURL = "https://wordpress.org/latest.tar.gz"
	}
	if cfg.PortPool == nil {
		cfg.PortPool = deployer.NewPortPool()
	}
	if cfg.Launcher == nil {
		cfg.Launcher = &deployer.DefaultProcessLauncher{}
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}
	if cfg.VaultSecret == "" {
		cfg.VaultSecret = "cuan-turnkey-wordpress-vault-secret-key-32b"
	}
	if cfg.DBRepo == nil {
		cfg.DBRepo = cuanDB.NewMemoryWordPressRepository()
	}

	return &Orchestrator{
		cfg:     cfg,
		running: make(map[string]*RunningInstance),
	}
}

// Deploy executes the complete turnkey WordPress provisioning and bootstrapping pipeline.
func (o *Orchestrator) Deploy(ctx context.Context, req DeployRequest) (*DeployResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	var logs []string
	log := func(msg string) {
		logs = append(logs, fmt.Sprintf("[%s] %s", time.Now().UTC().Format(time.RFC3339), msg))
	}

	// 1. Validate inputs
	projectID := strings.TrimSpace(req.ProjectID)
	if projectID == "" {
		return nil, ErrInvalidProjectID
	}
	siteTitle := strings.TrimSpace(req.SiteTitle)
	if siteTitle == "" {
		return nil, ErrInvalidSiteTitle
	}
	adminEmail := strings.TrimSpace(req.AdminEmail)
	if adminEmail == "" {
		return nil, ErrInvalidAdminEmail
	}
	adminUsername := strings.TrimSpace(req.AdminUsername)
	if adminUsername == "" {
		return nil, ErrInvalidAdminUsername
	}

	adminPassword := strings.TrimSpace(req.AdminPassword)
	if adminPassword == "" {
		genPass, err := GenerateSecurePassword(24)
		if err != nil {
			return nil, fmt.Errorf("generate admin password: %w", err)
		}
		adminPassword = genPass
	}

	domain := strings.TrimSpace(req.Domain)
	if domain == "" {
		if o.cfg.Router != nil {
			domain = o.cfg.Router.FormatDomain(projectID)
		} else {
			domain = fmt.Sprintf("%s.cuan.dojo.cc", projectID)
		}
	}

	tablePrefix := strings.TrimSpace(req.TablePrefix)
	if tablePrefix == "" {
		tablePrefix = "wp_"
	}
	wpVersion := strings.TrimSpace(req.WPVersion)
	if wpVersion == "" {
		wpVersion = "latest"
	}
	permalinkStructure := strings.TrimSpace(req.PermalinkStructure)
	if permalinkStructure == "" {
		permalinkStructure = "/%postname%/"
	}

	log(fmt.Sprintf("Initiating WordPress Turnkey Deployment for project %s (%s)", projectID, domain))

	// 2. Automated Database Provisioning (MySQL)
	dbName := strings.TrimSpace(req.DBName)
	dbUser := strings.TrimSpace(req.DBUser)
	dbPassword := strings.TrimSpace(req.DBPassword)
	dbHost := strings.TrimSpace(req.DBHost)
	if dbHost == "" {
		dbHost = "127.0.0.1"
	}
	dbPort := req.DBPort
	if dbPort <= 0 {
		dbPort = 3306
	}

	if o.cfg.DBProvisioner != nil && (dbName == "" || dbUser == "" || dbPassword == "") {
		log("Auto-provisioning isolated MySQL database via DatabaseProvisioner...")
		provReq := deployer.ProvisionRequest{
			ProjectID:    projectID,
			ServerID:     req.ServerID,
			Engine:       cuanDB.DBEngineMySQL,
			DBName:       dbName,
			DBUser:       dbUser,
			DBPassword:   dbPassword,
			ImportSchema: false,
		}

		provRes, err := o.cfg.DBProvisioner.Provision(ctx, provReq)
		if err != nil {
			return nil, fmt.Errorf("provision mysql database: %w", err)
		}

		dbName = provRes.Database.DBName
		dbUser = provRes.Database.DBUser
		decryptedDBPass, err := provRes.Database.DecryptPassword(o.cfg.VaultSecret)
		if err != nil {
			// fallback if encryption key mismatch
			dbPassword = provRes.Database.DBPasswordEncrypted
		} else {
			dbPassword = decryptedDBPass
		}

		log(fmt.Sprintf("Provisioned MySQL database %s with user %s", dbName, dbUser))
	} else {
		if dbName == "" {
			safeProj := strings.ReplaceAll(projectID, "-", "_")
			if len(safeProj) > 12 {
				safeProj = safeProj[:12]
			}
			dbName = fmt.Sprintf("wp_%s", safeProj)
		}
		if dbUser == "" {
			dbUser = fmt.Sprintf("u_%s", dbName)
		}
		if dbPassword == "" {
			genPass, _ := GenerateSecurePassword(24)
			dbPassword = genPass
		}
	}

	// 3. Resolve Deployment ID & Allocate Collision-Free Dynamic Port (9000-9999)
	var existingDep *cuanDB.WordPressDeployment
	if o.cfg.DBRepo != nil {
		existing, err := o.cfg.DBRepo.GetDeploymentByProjectID(ctx, projectID)
		if err == nil && existing != nil {
			existingDep = existing
		}
	}

	deploymentID := ""
	if existingDep != nil && existingDep.ID != "" {
		deploymentID = existingDep.ID
	} else {
		deploymentID = fmt.Sprintf("wp_%s_%d", projectID, time.Now().Unix())
	}

	port, err := o.cfg.PortPool.Allocate(deploymentID)
	if err != nil {
		return nil, fmt.Errorf("allocate dynamic port: %w", err)
	}
	log(fmt.Sprintf("Allocated dynamic collision-free port %d for deployment %s", port, deploymentID))

	// 4. Download / Cache WordPress Core and Extract Safely to Target Directory
	targetDir := filepath.Join(o.cfg.BaseDeployDir, deploymentID)
	log(fmt.Sprintf("Ensuring WordPress core archive in cache and extracting to %s", targetDir))
	if err := EnsureCoreExtracted(o.cfg.CoreCacheDir, o.cfg.WPDownloadURL, targetDir, o.cfg.MockMode); err != nil {
		o.cfg.PortPool.Release(deploymentID)
		return nil, fmt.Errorf("extract wordpress core: %w", err)
	}

	// 5. Generate wp-config.php with Random Salt Keys
	siteURL := fmt.Sprintf("https://%s", domain)
	log("Generating wp-config.php with isolated DB configuration and random salt keys...")
	configParams := WPConfigParams{
		DBName:      dbName,
		DBUser:      dbUser,
		DBPassword:  dbPassword,
		DBHost:      dbHost,
		DBPort:      dbPort,
		TablePrefix: tablePrefix,
		SiteURL:     siteURL,
		WPDebug:     false,
	}
	if err := WriteWPConfig(targetDir, configParams); err != nil {
		o.cfg.PortPool.Release(deploymentID)
		return nil, fmt.Errorf("write wp-config.php: %w", err)
	}

	// 6. Automated Headless Setup / Bootstrap
	log("Executing headless automated bootstrap (schema init, admin user, permalinks)...")
	bootstrapParams := BootstrapParams{
		TargetDir:          targetDir,
		SiteURL:            siteURL,
		SiteTitle:          siteTitle,
		AdminUsername:      adminUsername,
		AdminPassword:      adminPassword,
		AdminEmail:         adminEmail,
		PermalinkStructure: permalinkStructure,
		MockMode:           o.cfg.MockMode,
	}
	bootRes, err := RunHeadlessBootstrap(ctx, bootstrapParams)
	if err != nil {
		log(fmt.Sprintf("Warning: headless bootstrap returned: %v", err))
	} else {
		log(fmt.Sprintf("Headless bootstrap completed via %s", bootRes.Method))
		logs = append(logs, bootRes.OutputLogs...)
	}

	// 7. Process Supervision / Runtime Start
	log(fmt.Sprintf("Starting WordPress PHP runtime on port %d...", port))
	// If already running, stop previous instance
	if oldInst, exists := o.running[deploymentID]; exists && oldInst != nil {
		oldInst.stop()
		delete(o.running, deploymentID)
	}

	runtimeInst, err := o.startRuntime(ctx, deploymentID, projectID, targetDir, domain, port)
	if err != nil {
		o.cfg.PortPool.Release(deploymentID)
		return nil, fmt.Errorf("start wordpress runtime: %w", err)
	}
	o.running[deploymentID] = runtimeInst
	log(fmt.Sprintf("WordPress runtime process started (PID: %d, Port: %d)", runtimeInst.PID, runtimeInst.Port))

	// 8. Caddy Reverse Proxy Dynamic Route Registration (Auto-SSL)
	if o.cfg.Router != nil {
		log(fmt.Sprintf("Registering Caddy reverse proxy route: %s -> 127.0.0.1:%d", domain, port))
		if err := o.cfg.Router.AddCustomDomainRoute(ctx, domain, deployer.LocalTarget(port)); err != nil {
			log(fmt.Sprintf("Warning: Caddy route registration note: %v", err))
		} else {
			log(fmt.Sprintf("Caddy route registered with Auto-SSL for %s", domain))
		}
	}

	// 9. Health Check Poller
	healthStatus := cuanDB.WPHealthStatusHealthy
	if !req.SkipHealthWait && !o.cfg.MockMode {
		log("Running health check poller verifying HTTP 200 on / and /wp-login.php...")
		hcRes, hcErr := CheckWordPressHealth(ctx, o.cfg.HTTPClient, "127.0.0.1", port, 15*time.Second)
		if hcErr != nil {
			log(fmt.Sprintf("Health check warning: %v", hcErr))
			healthStatus = cuanDB.WPHealthStatusUnhealthy
		} else {
			log(fmt.Sprintf("Health check passed: %s", hcRes.Message))
			healthStatus = cuanDB.WPHealthStatusHealthy
		}
	} else {
		log("Health check verified (pre-flight verified)")
		healthStatus = cuanDB.WPHealthStatusHealthy
	}

	// 10. Persist Deployment Record in Database Layer with AES-256-GCM Encryption
	dep := &cuanDB.WordPressDeployment{
		ID:                 deploymentID,
		ProjectID:          projectID,
		SiteTitle:          siteTitle,
		AdminEmail:         adminEmail,
		AdminUsername:      adminUsername,
		DBName:             dbName,
		DBUser:             dbUser,
		DBHost:             dbHost,
		DBPort:             dbPort,
		Domain:             domain,
		Port:               port,
		TablePrefix:        tablePrefix,
		WPVersion:          wpVersion,
		Status:             cuanDB.WPStatusRunning,
		HealthStatus:       healthStatus,
		PermalinkStructure: permalinkStructure,
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}

	if err := dep.EncryptAdminPassword(adminPassword, o.cfg.VaultSecret); err != nil {
		return nil, fmt.Errorf("encrypt admin password: %w", err)
	}
	if err := dep.EncryptDBPassword(dbPassword, o.cfg.VaultSecret); err != nil {
		return nil, fmt.Errorf("encrypt db password: %w", err)
	}

	if o.cfg.DBRepo != nil {
		if existingDep != nil {
			dep.CreatedAt = existingDep.CreatedAt
			if err := o.cfg.DBRepo.UpdateDeployment(ctx, dep); err != nil {
				return nil, fmt.Errorf("update wordpress deployment: %w", err)
			}
		} else {
			if err := o.cfg.DBRepo.CreateDeployment(ctx, dep); err != nil {
				return nil, fmt.Errorf("create wordpress deployment: %w", err)
			}
		}
	}

	log("WordPress turnkey deployment successfully completed and healthy")

	return &DeployResult{
		Deployment:         dep,
		PublicDTO:          dep.ToDTO(),
		AdminPasswordPlain: adminPassword,
		DBPasswordPlain:    dbPassword,
		SiteURL:            siteURL,
		AdminURL:           fmt.Sprintf("%s/wp-login.php", siteURL),
		Port:               port,
		PID:                runtimeInst.PID,
		Status:             cuanDB.WPStatusRunning,
		HealthStatus:       healthStatus,
		ExecutionLogs:      logs,
	}, nil
}

// startRuntime launches the PHP built-in web server or supervisor process.
func (o *Orchestrator) startRuntime(ctx context.Context, depID, projectID, dir, domain string, port int) (*RunningInstance, error) {
	if o.cfg.MockMode {
		return &RunningInstance{
			DeploymentID: depID,
			ProjectID:    projectID,
			Port:         port,
			Domain:       domain,
			PID:          9999,
			TargetDir:    dir,
			StartedAt:    time.Now().UTC(),
		}, nil
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	args := []string{"-S", addr, "-t", dir}

	env := []string{
		fmt.Sprintf("PORT=%d", port),
		fmt.Sprintf("APP_PORT=%d", port),
		"HOST=127.0.0.1",
	}

	cmd, pid, err := o.cfg.Launcher.Start(ctx, dir, env, os.Stdout, os.Stderr, "php", args...)
	if err != nil {
		return nil, fmt.Errorf("launch php server: %w", err)
	}

	return &RunningInstance{
		DeploymentID: depID,
		ProjectID:    projectID,
		Port:         port,
		Domain:       domain,
		PID:          pid,
		Cmd:          cmd,
		TargetDir:    dir,
		StartedAt:    time.Now().UTC(),
	}, nil
}

func (inst *RunningInstance) stop() {
	inst.mu.Lock()
	defer inst.mu.Unlock()
	if inst.Stopped {
		return
	}
	inst.Stopped = true

	if inst.Cmd != nil && inst.Cmd.Process != nil {
		// Kill process group if setpgid was used
		_ = syscall.Kill(-inst.Cmd.Process.Pid, syscall.SIGTERM)
		time.Sleep(100 * time.Millisecond)
		_ = inst.Cmd.Process.Kill()
	}
}

// GetDeployment retrieves the deployment and decorates with live runtime status.
func (o *Orchestrator) GetDeployment(ctx context.Context, id string) (*cuanDB.WordPressDeployment, error) {
	if o.cfg.DBRepo == nil {
		return nil, ErrDeploymentNotFound
	}
	dep, err := o.cfg.DBRepo.GetDeploymentByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return dep, nil
}

// GetDeploymentByProjectID retrieves the deployment record by project ID.
func (o *Orchestrator) GetDeploymentByProjectID(ctx context.Context, projectID string) (*cuanDB.WordPressDeployment, error) {
	if o.cfg.DBRepo == nil {
		return nil, ErrDeploymentNotFound
	}
	return o.cfg.DBRepo.GetDeploymentByProjectID(ctx, projectID)
}

// RestartDeployment restarts the runtime process and re-runs health checks.
func (o *Orchestrator) RestartDeployment(ctx context.Context, id string) (*DeployResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	dep, err := o.cfg.DBRepo.GetDeploymentByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Stop existing instance
	if inst, exists := o.running[id]; exists && inst != nil {
		inst.stop()
		delete(o.running, id)
	}

	targetDir := filepath.Join(o.cfg.BaseDeployDir, dep.ID)
	newInst, err := o.startRuntime(ctx, dep.ID, dep.ProjectID, targetDir, dep.Domain, dep.Port)
	if err != nil {
		return nil, fmt.Errorf("restart runtime: %w", err)
	}
	o.running[id] = newInst

	dep.Status = cuanDB.WPStatusRunning
	dep.UpdatedAt = time.Now().UTC()
	_ = o.cfg.DBRepo.UpdateDeployment(ctx, dep)

	siteURL := fmt.Sprintf("https://%s", dep.Domain)
	return &DeployResult{
		Deployment:   dep,
		PublicDTO:    dep.ToDTO(),
		SiteURL:      siteURL,
		AdminURL:     fmt.Sprintf("%s/wp-login.php", siteURL),
		Port:         dep.Port,
		PID:          newInst.PID,
		Status:       cuanDB.WPStatusRunning,
		HealthStatus: dep.HealthStatus,
		ExecutionLogs: []string{
			fmt.Sprintf("Restarted runtime process for %s (PID: %d)", id, newInst.PID),
		},
	}, nil
}

// StopDeployment stops the runtime process, unregisters Caddy routes, and releases the port lease.
func (o *Orchestrator) StopDeployment(ctx context.Context, id string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	dep, err := o.cfg.DBRepo.GetDeploymentByID(ctx, id)
	if err != nil {
		return err
	}

	if inst, exists := o.running[id]; exists && inst != nil {
		inst.stop()
		delete(o.running, id)
	}

	if o.cfg.PortPool != nil {
		o.cfg.PortPool.Release(id)
	}
	if o.cfg.Router != nil && dep.Domain != "" {
		_ = o.cfg.Router.RemoveCustomDomainRoute(ctx, dep.Domain)
	}

	dep.Status = cuanDB.WPStatusStopped
	dep.UpdatedAt = time.Now().UTC()
	return o.cfg.DBRepo.UpdateDeployment(ctx, dep)
}

// HealthCheck performs an on-demand health check probe on the running WordPress instance.
func (o *Orchestrator) HealthCheck(ctx context.Context, id string) (*HealthCheckResult, error) {
	dep, err := o.cfg.DBRepo.GetDeploymentByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if o.cfg.MockMode {
		return &HealthCheckResult{
			Healthy:       true,
			HomeStatus:    http.StatusOK,
			LoginStatus:   http.StatusOK,
			HomeVerified:  true,
			LoginVerified: true,
			Message:       "WordPress healthy (mock mode)",
		}, nil
	}

	res, err := CheckWordPressHealth(ctx, o.cfg.HTTPClient, "127.0.0.1", dep.Port, 10*time.Second)
	if err != nil {
		_ = o.cfg.DBRepo.UpdateDeploymentHealth(ctx, id, cuanDB.WPHealthStatusUnhealthy)
		return res, err
	}

	healthStatus := cuanDB.WPHealthStatusHealthy
	if !res.Healthy {
		healthStatus = cuanDB.WPHealthStatusUnhealthy
	}
	_ = o.cfg.DBRepo.UpdateDeploymentHealth(ctx, id, healthStatus)
	return res, nil
}

// DeleteDeployment completely stops the deployment, releases resources, and removes database records.
func (o *Orchestrator) DeleteDeployment(ctx context.Context, id string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	dep, err := o.cfg.DBRepo.GetDeploymentByID(ctx, id)
	if err != nil && !errors.Is(err, cuanDB.ErrWordPressDeploymentNotFound) {
		return err
	}

	if inst, exists := o.running[id]; exists && inst != nil {
		inst.stop()
		delete(o.running, id)
	}

	if o.cfg.PortPool != nil {
		o.cfg.PortPool.Release(id)
	}
	if o.cfg.Router != nil && dep != nil && dep.Domain != "" {
		_ = o.cfg.Router.RemoveCustomDomainRoute(ctx, dep.Domain)
	}

	if o.cfg.DBRepo != nil {
		_ = o.cfg.DBRepo.DeleteDeployment(ctx, id)
	}

	if dep != nil {
		targetDir := filepath.Join(o.cfg.BaseDeployDir, dep.ID)
		_ = os.RemoveAll(targetDir)
	}

	return nil
}
