package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/crypto"
	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
	"github.com/buchin/cuan/internal/domain"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/observability"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/sponsorship"
	"github.com/buchin/cuan/internal/task"
)

type Server struct {
	cfg                 *config.Config
	db                  *sql.DB
	authSvc             *auth.Service
	projectSvc          *project.Service
	taskSvc             *task.Service
	runnerSvc           *runner.RunnerService
	sponsorshipSvc      *sponsorship.Service
	deployerDriver      deployer.DeployDriver
	serverRepo          cuanDB.ServerRepository
	domainRepo          cuanDB.DomainRepository
	dnsVerifier         *domain.DNSVerifier
	dnsPoller           *domain.DNSHealthPoller
	keyVault            *crypto.KeyVault
	sshManager          deployer.SSHManager
	healthPoller        *deployer.ProcessHealthPoller
	caddyRouter         *deployer.CaddyRouter
	obsRepo             cuanDB.ObservabilityRepository
	obsHealthPoller     *observability.HealthPoller
	obsMetricsCollector *observability.MetricsCollector
	router              *chi.Mux
}


func NewServer(cfg *config.Config, db *sql.DB, authSvc ...*auth.Service) *Server {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	var svc *auth.Service
	if len(authSvc) > 0 && authSvc[0] != nil {
		svc = authSvc[0]
	} else if db != nil {
		svc = auth.NewService(auth.NewDBStore(db))
	} else {
		svc = auth.NewService(auth.NewMemoryStore())
	}

	var pSvc *project.Service
	projectBaseDir := filepath.Join("data", "projects")
	if db != nil {
		pSvc = project.NewService(project.NewDBStore(db), git.NewClient(), cfg.AuthSecret, projectBaseDir)
	} else {
		pSvc = project.NewService(project.NewMemoryStore(), git.NewClient(), cfg.AuthSecret, projectBaseDir)
	}

	var tStore task.Store
	if db != nil {
		tStore = task.NewDBStore(db)
	} else {
		tStore = task.NewMemoryStore()
	}
	tSvc := task.NewService(tStore, pSvc)

	var rStore runner.Store
	if db != nil {
		rStore = runner.NewDBStore(db)
	} else {
		rStore = runner.NewMemoryStore(tStore)
	}
	rSvc := runner.NewService(rStore, tStore, pSvc)

	var spStore sponsorship.Store
	if db != nil {
		spStore = sponsorship.NewDBStore(db)
	} else {
		spStore = sponsorship.NewMemoryStore()
	}
	logosDir := cfg.SponsorLogosDir
	if logosDir == "" {
		logosDir = filepath.Join("data", "sponsors", "logos")
	}
	logoStorage, _ := sponsorship.NewLogoStorage(logosDir)
	spSvc := sponsorship.NewService(spStore, logoStorage, cfg.DojoWebhookSecret)
	if cfg.DojoWaitingListWebhookURL != "" {
		spSvc.SetDojoWebhookURL(cfg.DojoWaitingListWebhookURL)
	}
	if cfg.DojoCheckoutURL != "" {
		spSvc.SetCheckoutBaseURL(cfg.DojoCheckoutURL)
	}
	if cfg.AutoSyncReadme {
		syncer := sponsorship.NewSyncer(spStore, sponsorship.SyncerConfig{
			ReadmePath:         cfg.ReadmePath,
			SponsorsArchiveDir: cfg.SponsorsArchiveDir,
			AutoGitCommit:      cfg.SyncGitCommit,
			AuthorName:         cfg.GitAuthorName,
			AuthorEmail:        cfg.GitAuthorEmail,
		})
		spSvc.SetSyncer(syncer)
	}

	var depStore deployer.Store
	if db != nil {
		depStore = deployer.NewDBStore(db)
	} else {
		depStore = deployer.NewMemoryStore()
	}
	deploymentsDir := filepath.Join("data", "deployments")
	depStorage, _ := deployer.NewStorageEngine(deploymentsDir)
	caddyCfg := deployer.DefaultCaddyConfig()
	if cfg.BaseDomain != "" {
		caddyCfg.BaseDomain = cfg.BaseDomain
	}
	if cfg.CaddyAdminURL != "" {
		caddyCfg.AdminURL = cfg.CaddyAdminURL
	}
	caddyRouter := deployer.NewCaddyRouter(caddyCfg)
	portPool := deployer.NewPortPool()
	localDriver := deployer.NewLocalDriverWithRouter(depStorage, depStore, portPool, caddyRouter)

	var srvRepo cuanDB.ServerRepository
	if db != nil {
		srvRepo = cuanDB.NewSQLServerRepository(db)
	} else {
		srvRepo = cuanDB.NewMemoryServerRepository()
	}

	var domRepo cuanDB.DomainRepository
	if db != nil {
		domRepo = cuanDB.NewSQLDomainRepository(db)
	} else {
		domRepo = cuanDB.NewMemoryDomainRepository()
	}

	vault := crypto.DefaultKeyVault()
	sshMgr := deployer.NewClientPool(deployer.DefaultSSHTimeout)

	remoteDriver := deployer.NewRemoteSSHDriver(srvRepo, vault, sshMgr, depStore, portPool, caddyRouter)
	remoteDriver.SetStorage(depStorage)
	depDriver := deployer.NewDriverFactory(localDriver, remoteDriver, depStore)

	healthPoller := deployer.NewProcessHealthPoller(depStore, srvRepo, vault, sshMgr, portPool)
	remoteDriver.SetHealthPoller(healthPoller)

	baseHost := "cuan.dojo.cc"
	if cfg.BaseDomain != "" {
		baseHost = cfg.BaseDomain
	}
	dnsVerifier := domain.NewDNSVerifier(domain.DNSVerifierConfig{
		CanonicalHosts: []string{baseHost, "cuan.app", "cuan.dojo.cc"},
		TargetIPs:      []string{"127.0.0.1", "::1"},
	})
	dnsPoller := domain.NewDNSHealthPoller(domRepo, dnsVerifier, depDriver, srvRepo, caddyRouter)

	var obsRepo cuanDB.ObservabilityRepository
	if db != nil {
		obsRepo = cuanDB.NewSQLObservabilityRepository(db)
	} else {
		obsRepo = cuanDB.NewMemoryObservabilityRepository()
	}
	obsHealthPoller := observability.NewHealthPoller(obsRepo)
	defaultSSHRunner := observability.NewDefaultSSHRunner(vault, sshMgr)
	obsMetricsCollector := observability.NewMetricsCollector(
		obsRepo,
		observability.WithSSHRunner(defaultSSHRunner),
		observability.WithServerRepo(srvRepo),
	)

	s := &Server{
		cfg:                 cfg,
		db:                  db,
		authSvc:             svc,
		projectSvc:          pSvc,
		taskSvc:             tSvc,
		runnerSvc:           rSvc,
		sponsorshipSvc:      spSvc,
		deployerDriver:      depDriver,
		serverRepo:          srvRepo,
		domainRepo:          domRepo,
		dnsVerifier:         dnsVerifier,
		dnsPoller:           dnsPoller,
		keyVault:            vault,
		sshManager:          sshMgr,
		healthPoller:        healthPoller,
		caddyRouter:         caddyRouter,
		obsRepo:             obsRepo,
		obsHealthPoller:     obsHealthPoller,
		obsMetricsCollector: obsMetricsCollector,
		router:              r,
	}

	s.routes()
	s.setupStaticRoutes()
	return s
}

func (s *Server) Router() http.Handler {
	return s.router
}

func (s *Server) AuthService() *auth.Service {
	return s.authSvc
}

func (s *Server) ProjectService() *project.Service {
	return s.projectSvc
}

func (s *Server) SetProjectService(svc *project.Service) {
	s.projectSvc = svc
}

func (s *Server) TaskService() *task.Service {
	return s.taskSvc
}

func (s *Server) SetTaskService(svc *task.Service) {
	s.taskSvc = svc
}

func (s *Server) RunnerService() *runner.RunnerService {
	return s.runnerSvc
}

func (s *Server) SetRunnerService(svc *runner.RunnerService) {
	s.runnerSvc = svc
}

func (s *Server) SponsorshipService() *sponsorship.Service {
	return s.sponsorshipSvc
}

func (s *Server) SetSponsorshipService(svc *sponsorship.Service) {
	s.sponsorshipSvc = svc
}

func (s *Server) DeployerDriver() deployer.DeployDriver {
	return s.deployerDriver
}

func (s *Server) SetDeployerDriver(d deployer.DeployDriver) {
	s.deployerDriver = d
}

func (s *Server) ServerRepository() cuanDB.ServerRepository {
	return s.serverRepo
}

func (s *Server) SetServerRepository(r cuanDB.ServerRepository) {
	s.serverRepo = r
}

func (s *Server) KeyVault() *crypto.KeyVault {
	return s.keyVault
}

func (s *Server) SetKeyVault(v *crypto.KeyVault) {
	s.keyVault = v
}

func (s *Server) SSHManager() deployer.SSHManager {
	return s.sshManager
}

func (s *Server) SetSSHManager(m deployer.SSHManager) {
	s.sshManager = m
}

func (s *Server) HealthPoller() *deployer.ProcessHealthPoller {
	return s.healthPoller
}

func (s *Server) SetHealthPoller(p *deployer.ProcessHealthPoller) {
	s.healthPoller = p
}

func (s *Server) DomainRepository() cuanDB.DomainRepository {
	return s.domainRepo
}

func (s *Server) SetDomainRepository(r cuanDB.DomainRepository) {
	s.domainRepo = r
}

func (s *Server) DNSVerifier() *domain.DNSVerifier {
	return s.dnsVerifier
}

func (s *Server) SetDNSVerifier(v *domain.DNSVerifier) {
	s.dnsVerifier = v
}

func (s *Server) DNSPoller() *domain.DNSHealthPoller {
	return s.dnsPoller
}

func (s *Server) SetDNSPoller(p *domain.DNSHealthPoller) {
	s.dnsPoller = p
}

func (s *Server) CaddyRouter() *deployer.CaddyRouter {
	return s.caddyRouter
}

func (s *Server) SetCaddyRouter(r *deployer.CaddyRouter) {
	s.caddyRouter = r
}

func (s *Server) AppMode() string {
	if s.cfg != nil && s.cfg.AppMode != "" {
		return s.cfg.AppMode
	}
	return "marketing"
}

func (s *Server) SetAppMode(mode string) {
	if s.cfg != nil {
		s.cfg.AppMode = mode
	}
}

func (s *Server) ObservabilityRepository() cuanDB.ObservabilityRepository {
	return s.obsRepo
}

func (s *Server) SetObservabilityRepository(repo cuanDB.ObservabilityRepository) {
	s.obsRepo = repo
}

func (s *Server) ObservabilityHealthPoller() *observability.HealthPoller {
	return s.obsHealthPoller
}

func (s *Server) SetObservabilityHealthPoller(p *observability.HealthPoller) {
	s.obsHealthPoller = p
}

func (s *Server) ObservabilityMetricsCollector() *observability.MetricsCollector {
	return s.obsMetricsCollector
}

func (s *Server) SetObservabilityMetricsCollector(c *observability.MetricsCollector) {
	s.obsMetricsCollector = c
}

func (s *Server) routes() {
	// Public Health, Readiness & System Mode (Sprint 3 S03.5)
	s.router.Get("/healthz", s.handleHealthz)
	s.router.Get("/readyz", s.handleReadyz)
	s.router.Get("/api/system/mode", s.handleGetSystemMode)

	// Caddy On-Demand TLS Security Gatekeeper (Sprint 5 S05.2)
	s.router.Get("/api/caddy/ask", s.handleCaddyAsk)
	s.router.Get("/api/domains/check-ask", s.handleCaddyAsk)
	s.router.Get("/api/v1/caddy/ask", s.handleCaddyAsk)

	// Dojo Webhook Receiver (SSOT S02.1 spec: /api/webhooks/dojo)
	s.router.Post("/api/webhooks/dojo", s.handleDojoWebhook)

	// Public Sponsorship, Waiting List, and Logo Endpoints
	s.router.Post("/api/sponsors/upload-logo", s.handleUploadSponsorLogo)
	s.router.Post("/api/sponsors/intake", s.handleSponsorIntake)
	s.router.Post("/api/sponsors/checkout", s.handleSponsorCheckout)
	s.router.Get("/api/sponsors/current", s.handleListCurrentSponsors)
	s.router.Get("/api/sponsors/status", s.handleGetSponsorshipStatus)
	s.router.Get("/api/sponsors/batches", s.handleListBatches)
	s.router.Post("/api/leads", s.handleCreateLead)
	s.router.Post("/api/waiting-list", s.handleCreateLead)
	s.router.Get("/uploads/sponsors/logos/*", s.handleServeSponsorLogo)

	// Studio Deployments Engine Endpoints (Sprint 3)
	s.router.Get("/api/deployments", s.handleListDeployments)
	s.router.Post("/api/deployments/upload", s.handleUploadDeploymentZip)
	s.router.Post("/api/deployments/git", s.handleCreateDeploymentGit)
	s.router.Post("/api/deployments", s.handleCreateDeployment)
	s.router.Get("/api/deployments/{id}", s.handleGetDeployment)
	s.router.Post("/api/deployments/{id}/start", s.handleStartDeployment)
	s.router.Post("/api/deployments/{id}/stop", s.handleStopDeployment)
	s.router.Post("/api/deployments/{id}/restart", s.handleRestartDeployment)
	s.router.Delete("/api/deployments/{id}", s.handleDeleteDeployment)
	s.router.Get("/api/deployments/{id}/logs", s.handleGetDeploymentLogs)
	s.router.Get("/api/deployments/{id}/health", s.handleGetDeploymentHealth)
	s.router.Post("/api/deployments/{id}/health/check-now", s.handleCheckDeploymentHealthNow)

	// BYOS Server Management Endpoints (Sprint 4 S04.2 & Sprint 6 S06.2)
	s.router.Get("/api/servers", s.handleListServers)
	s.router.Post("/api/servers", s.handleCreateServer)
	s.router.Get("/api/servers/{id}", s.handleGetServer)
	s.router.Get("/api/servers/{id}/metrics", s.handleGetServerMetrics)
	s.router.Post("/api/servers/{id}/test", s.handleTestServer)
	s.router.Delete("/api/servers/{id}", s.handleDeleteServer)

	// Custom Domain & DNS Verification Endpoints (Sprint 5 S05.3)
	s.router.Post("/api/deployments/{id}/domains", s.handleCreateDeploymentDomain)
	s.router.Get("/api/deployments/{id}/domains", s.handleListDeploymentDomains)
	s.router.Get("/api/domains", s.handleListDomains)
	s.router.Get("/api/domains/{id}", s.handleGetDomain)
	s.router.Post("/api/domains/{id}/verify", s.handleVerifyDomain)
	s.router.Delete("/api/domains/{id}", s.handleDeleteDomain)

	// Admin Sponsorship Moderation Endpoints (Single-Owner access only)
	s.router.Group(func(ar chi.Router) {
		ar.Use(auth.Middleware(s.authSvc))
		ar.Get("/api/admin/sponsors", s.handleAdminListSponsors)
		ar.Post("/api/admin/sponsors/{id}/approve", s.handleAdminApproveSponsor)
		ar.Post("/api/admin/sponsors/{id}/reject", s.handleAdminRejectSponsor)
		ar.Post("/api/admin/sponsors/sync-readme", s.handleAdminSyncReadme)
		ar.Get("/api/admin/sponsors/{id}/activities", s.handleAdminListSponsorActivities)
		ar.Get("/api/admin/leads", s.handleAdminListLeads)
	})

	// Studio Deployments route group in /api/v1
	s.router.Route("/api/v1/deployments", func(dr chi.Router) {
		dr.Get("/", s.handleListDeployments)
		dr.Post("/upload", s.handleUploadDeploymentZip)
		dr.Post("/git", s.handleCreateDeploymentGit)
		dr.Post("/", s.handleCreateDeployment)
		dr.Get("/{id}", s.handleGetDeployment)
		dr.Post("/{id}/start", s.handleStartDeployment)
		dr.Post("/{id}/stop", s.handleStopDeployment)
		dr.Post("/{id}/restart", s.handleRestartDeployment)
		dr.Delete("/{id}", s.handleDeleteDeployment)
		dr.Get("/{id}/logs", s.handleGetDeploymentLogs)
		dr.Get("/{id}/health", s.handleGetDeploymentHealth)
		dr.Post("/{id}/health/check-now", s.handleCheckDeploymentHealthNow)
		dr.Post("/{id}/domains", s.handleCreateDeploymentDomain)
		dr.Get("/{id}/domains", s.handleListDeploymentDomains)
	})

	// BYOS Server Management route group in /api/v1
	s.router.Route("/api/v1/servers", func(sr chi.Router) {
		sr.Get("/", s.handleListServers)
		sr.Post("/", s.handleCreateServer)
		sr.Get("/{id}", s.handleGetServer)
		sr.Get("/{id}/metrics", s.handleGetServerMetrics)
		sr.Post("/{id}/test", s.handleTestServer)
		sr.Delete("/{id}", s.handleDeleteServer)
	})

	// Custom Domain & DNS Verification route group in /api/v1
	s.router.Route("/api/v1/domains", func(dr chi.Router) {
		dr.Get("/", s.handleListDomains)
		dr.Get("/{id}", s.handleGetDomain)
		dr.Post("/{id}/verify", s.handleVerifyDomain)
		dr.Delete("/{id}", s.handleDeleteDomain)
	})

	s.router.Route("/api/v1", func(r chi.Router) {
		// Public endpoints
		r.Get("/info", s.handleInfo)
		r.Get("/system/mode", s.handleGetSystemMode)
		r.Get("/auth/status", s.handleAuthStatus)
		r.Post("/auth/setup", s.handleAuthSetup)
		r.Post("/auth/login", s.handleAuthLogin)
		r.Post("/auth/logout", s.handleAuthLogout)

		// Webhook Dojo alias in v1
		r.Post("/webhooks/dojo", s.handleDojoWebhook)

		// Public sponsorship endpoints in v1
		r.Post("/sponsors/upload-logo", s.handleUploadSponsorLogo)
		r.Post("/sponsors/intake", s.handleSponsorIntake)
		r.Post("/sponsors/checkout", s.handleSponsorCheckout)
		r.Get("/sponsors/current", s.handleListCurrentSponsors)
		r.Get("/sponsors/status", s.handleGetSponsorshipStatus)
		r.Get("/sponsors/batches", s.handleListBatches)
		r.Post("/leads", s.handleCreateLead)
		r.Post("/waiting-list", s.handleCreateLead)

		// Runner outbound endpoints (T5 - Authenticated via X-Runner-Token or Owner Token)
		r.Group(func(rr chi.Router) {
			rr.Use(s.runnerAuthMiddleware)
			rr.Post("/runner/heartbeat", s.handleRunnerHeartbeat)
			rr.Post("/runner/claim", s.handleRunnerClaim)
			rr.Post("/runner/attempts/{id}/logs", s.handleRunnerStreamLogs)
			rr.Post("/runner/attempts/{id}/complete", s.handleRunnerComplete)
		})

		// Protected endpoints (Single-Owner access only)
		r.Group(func(pr chi.Router) {
			pr.Use(auth.Middleware(s.authSvc))

			pr.Get("/auth/me", s.handleAuthMe)

			// Tasks endpoints (T4 & T5)
			pr.Get("/tasks", s.handleListTasks)
			pr.Post("/tasks", s.handleCreateTask)
			pr.Get("/tasks/{id}", s.handleGetTask)
			pr.Put("/tasks/{id}", s.handleUpdateTask)
			pr.Delete("/tasks/{id}", s.handleDeleteTask)
			pr.Post("/tasks/{id}/comments", s.handleAddComment)
			pr.Post("/tasks/{id}/attempts", s.handleCreateAttempt)
			pr.Patch("/tasks/{id}/attempts/{attemptId}", s.handleUpdateAttempt)
			pr.Post("/tasks/{id}/cancel", s.handleCancelTask)
			pr.Post("/tasks/{id}/retry", s.handleRetryTask)
			pr.Get("/tasks/{id}/attempts/{attemptId}/logs", s.handleGetAttemptLogs)

			// Runners & Infrastructure (T5)
			pr.Get("/runners", s.handleListRunners)
			pr.Get("/runners/{id}", s.handleGetRunner)
			pr.Post("/runner/reconcile", s.handleReconcileLeases)

			// Projects endpoints (T3)
			pr.Get("/projects", s.handleListProjects)
			pr.Post("/projects", s.handleConnectProject)
			pr.Get("/projects/{id}", s.handleGetProject)
			pr.Post("/projects/{id}/sync", s.handleSyncProject)
			pr.Delete("/projects/{id}", s.handleDeleteProject)
			pr.Get("/projects/{id}/branches", s.handleGetProjectBranches)

			// Sponsorship Moderation endpoints (S02.4)
			pr.Get("/admin/sponsors", s.handleAdminListSponsors)
			pr.Post("/admin/sponsors/{id}/approve", s.handleAdminApproveSponsor)
			pr.Post("/admin/sponsors/{id}/reject", s.handleAdminRejectSponsor)
			pr.Post("/admin/sponsors/sync-readme", s.handleAdminSyncReadme)
			pr.Get("/admin/sponsors/{id}/activities", s.handleAdminListSponsorActivities)
			pr.Get("/sponsors", s.handleAdminListSponsors)
			pr.Post("/sponsors/{id}/approve", s.handleAdminApproveSponsor)
			pr.Post("/sponsors/{id}/reject", s.handleAdminRejectSponsor)
			pr.Post("/sponsors/sync-readme", s.handleAdminSyncReadme)
			pr.Get("/sponsors/{id}/activities", s.handleAdminListSponsorActivities)
			pr.Get("/leads", s.handleAdminListLeads)
		})
	})
}


func (s *Server) setupStaticRoutes() {
	staticDir := s.cfg.StaticDir
	if staticDir == "" {
		staticDir = "web/dist"
	}

	if fi, err := os.Stat(staticDir); err == nil && fi.IsDir() {
		fileServer := http.FileServer(http.Dir(staticDir))
		s.router.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Clean(r.URL.Path)
			if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/healthz") || strings.HasPrefix(path, "/readyz") || strings.HasPrefix(path, "/uploads") {
				http.NotFound(w, r)
				return
			}

			filePath := filepath.Join(staticDir, path)
			if f, err := os.Stat(filePath); err == nil && !f.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}

			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
		})
	} else {
		s.router.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Clean(r.URL.Path)
			if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/healthz") || strings.HasPrefix(path, "/readyz") || strings.HasPrefix(path, "/uploads") {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Cuan Server</title></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; padding: 2rem; max-width: 600px; margin: 0 auto; line-height: 1.5;">
  <h2>Cuan API Server</h2>
  <p>Status: Healthy. Backend services are operational.</p>
  <p>To view the React UI shell, build the frontend bundle:</p>
  <pre style="background: #f1f5f9; padding: 1rem; border-radius: 6px;">cd web && npm run build</pre>
  <ul>
    <li><a href="/healthz">/healthz</a> — Liveness check</li>
    <li><a href="/readyz">/readyz</a> — Readiness check</li>
    <li><a href="/api/v1/info">/api/v1/info</a> — Server metadata</li>
  </ul>
</body>
</html>`))
		})
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	version := "0.1.0"
	if s.cfg != nil && s.cfg.AppVersion != "" {
		version = s.cfg.AppVersion
	}
	resp := map[string]interface{}{
		"status":    "healthy",
		"app":       "cuan",
		"version":   version,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetSystemMode(w http.ResponseWriter, r *http.Request) {
	mode := "marketing"
	version := "0.1.0"
	baseDomain := "cuan.dojo.cc"

	if s.cfg != nil {
		if s.cfg.AppMode != "" {
			mode = s.cfg.AppMode
		}
		if s.cfg.AppVersion != "" {
			version = s.cfg.AppVersion
		}
		if s.cfg.BaseDomain != "" {
			baseDomain = s.cfg.BaseDomain
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"app_mode":    mode,
		"version":     version,
		"base_domain": baseDomain,
	})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		if s.cfg.DatabaseURL == "" {
			respondJSON(w, http.StatusOK, map[string]interface{}{
				"status":   "ready",
				"database": "in_memory_dev",
			})
			return
		}
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status":   "not_ready",
			"database": "disconnected",
			"error":    "database pool is not initialized",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if err := s.db.PingContext(ctx); err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status":   "not_ready",
			"database": "disconnected",
			"error":    err.Error(),
		})
		return
	}

	var migrationCount int
	err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM cuan_schema_migrations").Scan(&migrationCount)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status":     "not_ready",
			"database":   "connected",
			"migrations": "table_missing_or_error",
			"error":      err.Error(),
		})
		return
	}

	status, _ := s.authSvc.Status(ctx)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":           "ready",
		"database":         "connected",
		"migrations":       "applied",
		"migrations_count": migrationCount,
		"owner_configured": status.OwnerConfigured,
	})
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	version := "0.1.0"
	if s.cfg != nil && s.cfg.AppVersion != "" {
		version = s.cfg.AppVersion
	}
	resp := map[string]interface{}{
		"name":          "Cuan",
		"version":       version,
		"description":   "Agentic project management for self-hosted teams",
		"license":       "Apache-2.0",
		"repository":    "https://github.com/buchin/cuan",
		"documentation": "https://github.com/buchin/cuan#readme",
		"release_stage": "release-candidate",
		"author":        "Buchin <buchin@dropsugar.com>",
		"sprint":        "S1",
		"status":        "running",
		"adapter":       "agy",
		"architecture":  "Go backend + React Vite TS frontend",
	}
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.authSvc.Status(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, "failed to check auth status")
		return
	}
	respondJSON(w, http.StatusOK, status)
}

func (s *Server) handleAuthSetup(w http.ResponseWriter, r *http.Request) {
	var req auth.SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := s.authSvc.Setup(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrSetupAlreadyCompleted) {
			respondJSONError(w, http.StatusForbidden, "setup already completed: single owner already exists")
			return
		}
		if errors.Is(err, auth.ErrInvalidInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("setup failed: %v", err))
		return
	}

	setSessionCookie(w, r, resp.Token)
	respondJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	var req auth.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := s.authSvc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrSetupRequired) {
			respondJSONError(w, http.StatusBadRequest, "setup required: no owner account exists")
			return
		}
		if errors.Is(err, auth.ErrInvalidCredentials) {
			respondJSONError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}
		if errors.Is(err, auth.ErrInvalidInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("login failed: %v", err))
		return
	}

	setSessionCookie(w, r, resp.Token)
	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	token := auth.ExtractToken(r)
	if token != "" {
		_ = s.authSvc.Logout(r.Context(), token)
	}

	clearSessionCookie(w)
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "logged out successfully",
	})
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	owner := auth.GetOwnerFromContext(r.Context())
	if owner == nil {
		respondJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"owner": map[string]interface{}{
			"id":         owner.ID,
			"email":      owner.Email,
			"created_at": owner.CreatedAt,
		},
	})
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	filter := task.TaskFilter{
		ProjectID: r.URL.Query().Get("project_id"),
		Status:    r.URL.Query().Get("status"),
		Search:    r.URL.Query().Get("search"),
	}

	tasks, err := s.taskSvc.ListTasks(r.Context(), filter)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list tasks: %v", err))
		return
	}
	if tasks == nil {
		tasks = []*task.TaskItemResponse{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tasks": tasks,
		"total": len(tasks),
	})
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req task.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	author := "owner"
	if owner := auth.GetOwnerFromContext(r.Context()); owner != nil && owner.Email != "" {
		author = owner.Email
	}

	created, err := s.taskSvc.CreateTask(r.Context(), author, req)
	if err != nil {
		if errors.Is(err, task.ErrInvalidTaskInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create task: %v", err))
		return
	}

	respondJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	detail, err := s.taskSvc.GetTaskDetail(r.Context(), id)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "task tidak ditemukan")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get task: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, detail)
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req task.UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	author := "owner"
	if owner := auth.GetOwnerFromContext(r.Context()); owner != nil && owner.Email != "" {
		author = owner.Email
	}

	updated, err := s.taskSvc.UpdateTask(r.Context(), id, author, req)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "task tidak ditemukan")
			return
		}
		if errors.Is(err, task.ErrInvalidTaskInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update task: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := s.taskSvc.DeleteTask(r.Context(), id)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "task tidak ditemukan")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete task: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Task berhasil dihapus",
		"id":      id,
	})
}

func (s *Server) handleAddComment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req task.CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	author := "owner"
	if owner := auth.GetOwnerFromContext(r.Context()); owner != nil && owner.Email != "" {
		author = owner.Email
	}

	act, err := s.taskSvc.AddComment(r.Context(), id, author, req.Message)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "task tidak ditemukan")
			return
		}
		if errors.Is(err, task.ErrInvalidTaskInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to add comment: %v", err))
		return
	}

	respondJSON(w, http.StatusCreated, act)
}

func (s *Server) handleCreateAttempt(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req task.CreateAttemptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	author := "owner"
	if owner := auth.GetOwnerFromContext(r.Context()); owner != nil && owner.Email != "" {
		author = owner.Email
	}

	att, err := s.taskSvc.CreateAttempt(r.Context(), id, author, req)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "task tidak ditemukan")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create attempt: %v", err))
		return
	}

	respondJSON(w, http.StatusCreated, att)
}

func (s *Server) handleUpdateAttempt(w http.ResponseWriter, r *http.Request) {
	attemptId := chi.URLParam(r, "attemptId")
	var req task.UpdateAttemptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	author := "owner"
	if owner := auth.GetOwnerFromContext(r.Context()); owner != nil && owner.Email != "" {
		author = owner.Email
	}

	att, err := s.taskSvc.UpdateAttemptStatus(r.Context(), attemptId, author, req)
	if err != nil {
		if errors.Is(err, task.ErrAttemptNotFound) {
			respondJSONError(w, http.StatusNotFound, "attempt tidak ditemukan")
			return
		}
		if errors.Is(err, task.ErrInvalidTaskInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update attempt: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, att)
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.projectSvc.List(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list projects: %v", err))
		return
	}
	if projects == nil {
		projects = []*project.ProjectResponse{}
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"projects": projects,
		"total":    len(projects),
	})
}

func (s *Server) handleConnectProject(w http.ResponseWriter, r *http.Request) {
	var req project.ConnectProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.RepoURL) == "" {
		respondJSONError(w, http.StatusBadRequest, "repository URL is required")
		return
	}

	res, err := s.projectSvc.Connect(r.Context(), req)
	if err != nil {
		if errors.Is(err, git.ErrInvalidRepoURL) {
			respondJSONError(w, http.StatusBadRequest, "URL repository GitHub tidak valid. Gunakan format 'https://github.com/owner/repo' atau 'owner/repo'.")
			return
		}
		if errors.Is(err, git.ErrRepoNotFound) {
			respondJSONError(w, http.StatusNotFound, "Repository tidak ditemukan di GitHub. Pastikan nama owner/repo sudah benar dan jika repo bersifat private, sertakan Personal Access Token (PAT).")
			return
		}
		if errors.Is(err, git.ErrAccessDenied) {
			respondJSONError(w, http.StatusForbidden, "Akses ke repository GitHub ditolak (401/403). Periksa kevalidan Personal Access Token (PAT) dan izin scope 'repo'.")
			return
		}
		if errors.Is(err, project.ErrProjectAlreadyExists) {
			respondJSONError(w, http.StatusConflict, "Repository GitHub ini sudah pernah dihubungkan sebagai proyek di Cuan.")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("gagal menghubungkan repository: %v", err))
		return
	}

	respondJSON(w, http.StatusCreated, res)
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := s.projectSvc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			respondJSONError(w, http.StatusNotFound, "proyek tidak ditemukan")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get project: %v", err))
		return
	}
	respondJSON(w, http.StatusOK, p)
}

func (s *Server) handleSyncProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	res, err := s.projectSvc.Sync(r.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			respondJSONError(w, http.StatusNotFound, "proyek tidak ditemukan")
			return
		}
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   fmt.Sprintf("sinkronisasi gagal: %v", err),
			"project": res,
		})
		return
	}
	respondJSON(w, http.StatusOK, res)
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := s.projectSvc.Delete(r.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			respondJSONError(w, http.StatusNotFound, "proyek tidak ditemukan")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete project: %v", err))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Proyek berhasil dihapus dari Cuan. Repository GitHub remote tetap aman dan tidak disentuh.",
		"id":      id,
	})
}

func (s *Server) handleGetProjectBranches(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	branches, defaultBranch, err := s.projectSvc.GetBranches(r.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			respondJSONError(w, http.StatusNotFound, "proyek tidak ditemukan")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get branches: %v", err))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"branches":       branches,
		"default_branch": defaultBranch,
	})
}


func setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	isTLS := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	http.SetCookie(w, &http.Cookie{
		Name:     "cuan_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   isTLS,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(auth.SessionDuration.Seconds()),
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "cuan_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondJSONError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]interface{}{
		"error": message,
	})
}

func (s *Server) runnerAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Runner-Token")
		if token == "" {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				token = strings.TrimSpace(authHeader[7:])
			}
		}

		if token != "" {
			if token == s.cfg.RunnerToken || token == s.cfg.AuthSecret {
				next.ServeHTTP(w, r)
				return
			}
			if owner, err := s.authSvc.ValidateToken(r.Context(), token); err == nil && owner != nil {
				next.ServeHTTP(w, r)
				return
			}
		}

		respondJSONError(w, http.StatusUnauthorized, "unauthorized: runner token is invalid or missing")
	})
}

func (s *Server) handleRunnerHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req runner.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := s.runnerSvc.Heartbeat(r.Context(), req, s.cfg.RunnerLeaseDuration)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("heartbeat failed: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRunnerClaim(w http.ResponseWriter, r *http.Request) {
	var req runner.ClaimTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := s.runnerSvc.Claim(r.Context(), req, s.cfg.RunnerLeaseDuration)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("claim failed: %v", err))
		return
	}

	if resp.Claimed && resp.Task != nil && resp.Task.ProjectID != "" {
		if p, err := s.projectSvc.Get(r.Context(), resp.Task.ProjectID); err == nil && p != nil {
			resp.ProjectRepo = p.RepoURL
			resp.WorkingDir = p.LocalPath
		}
	}

	respondJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRunnerStreamLogs(w http.ResponseWriter, r *http.Request) {
	attemptID := chi.URLParam(r, "id")
	var req runner.StreamLogsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.taskSvc.AppendAttemptLogs(r.Context(), attemptID, req.Logs, s.cfg.RunnerMaxLogLines); err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to append logs: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "ok",
		"appended": len(req.Logs),
	})
}

func (s *Server) handleRunnerComplete(w http.ResponseWriter, r *http.Request) {
	attemptID := chi.URLParam(r, "id")
	var req runner.CompleteAttemptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	att, err := s.runnerSvc.Complete(r.Context(), attemptID, req)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to complete attempt: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, att)
}

func (s *Server) handleListRunners(w http.ResponseWriter, r *http.Request) {
	_, _ = s.runnerSvc.ReconcileExpiredLeases(r.Context())

	runners, err := s.runnerSvc.ListRunners(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list runners: %v", err))
		return
	}
	if runners == nil {
		runners = []*runner.RunnerInfo{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"runners": runners,
		"total":   len(runners),
	})
}

func (s *Server) handleGetRunner(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rInfo, err := s.runnerSvc.GetRunner(r.Context(), id)
	if err != nil {
		if errors.Is(err, runner.ErrRunnerNotFound) {
			respondJSONError(w, http.StatusNotFound, "runner tidak ditemukan")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get runner: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, rInfo)
}

func (s *Server) handleGetAttemptLogs(w http.ResponseWriter, r *http.Request) {
	attemptID := chi.URLParam(r, "attemptId")
	afterID, _ := strconv.ParseInt(r.URL.Query().Get("after_id"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 1000
	}

	logs, err := s.taskSvc.GetAttemptLogs(r.Context(), attemptID, afterID, limit)
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get logs: %v", err))
		return
	}
	if logs == nil {
		logs = []*task.LogRecord{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"logs":  logs,
		"total": len(logs),
	})
}

func (s *Server) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	author := "owner"
	if owner := auth.GetOwnerFromContext(r.Context()); owner != nil && owner.Email != "" {
		author = owner.Email
	}

	att, err := s.taskSvc.CancelTask(r.Context(), taskID, author)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "task tidak ditemukan")
			return
		}
		if errors.Is(err, task.ErrAttemptNotFound) {
			respondJSONError(w, http.StatusNotFound, "tidak ada attempt yang dapat dibatalkan")
			return
		}
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, att)
}

func (s *Server) handleRetryTask(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	author := "owner"
	if owner := auth.GetOwnerFromContext(r.Context()); owner != nil && owner.Email != "" {
		author = owner.Email
	}

	var req struct {
		RunnerID  string `json:"runner_id"`
		AgentName string `json:"agent_name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	att, err := s.taskSvc.RetryTask(r.Context(), taskID, author, req.RunnerID, req.AgentName)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "task tidak ditemukan")
			return
		}
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, att)
}

func (s *Server) handleReconcileLeases(w http.ResponseWriter, r *http.Request) {
	interrupted, err := s.runnerSvc.ReconcileExpiredLeases(r.Context())
	if err != nil {
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("reconcile leases failed: %v", err))
		return
	}
	if interrupted == nil {
		interrupted = []*task.TaskAttempt{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"interrupted_count": len(interrupted),
		"attempts":          interrupted,
	})
}
