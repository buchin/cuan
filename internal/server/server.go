package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	extensionSDK "github.com/buchin/cuan/pkg/extensions"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/campaign"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/creator"
	"github.com/buchin/cuan/internal/crypto"
	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
	"github.com/buchin/cuan/internal/domain"
	"github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/inspector"
	"github.com/buchin/cuan/internal/observability"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/publisher"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/sponsorship"
	"github.com/buchin/cuan/internal/studio"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
	"github.com/buchin/cuan/internal/wordpress"
)

type Server struct {
	extensions             *extensionSDK.Registry
	crm                    *crmStore
	canned                 *cannedStore
	cfg                    *config.Config
	db                     *sql.DB
	authSvc                *auth.Service
	projectSvc             *project.Service
	taskSvc                *task.Service
	runnerSvc              *runner.RunnerService
	sponsorshipSvc         *sponsorship.Service
	deployerDriver         deployer.DeployDriver
	serverRepo             cuanDB.ServerRepository
	domainRepo             cuanDB.DomainRepository
	dnsVerifier            *domain.DNSVerifier
	dnsPoller              *domain.DNSHealthPoller
	keyVault               *crypto.KeyVault
	sshManager             deployer.SSHManager
	healthPoller           *deployer.ProcessHealthPoller
	caddyRouter            *deployer.CaddyRouter
	obsRepo                cuanDB.ObservabilityRepository
	obsHealthPoller        *observability.HealthPoller
	obsMetricsCollector    *observability.MetricsCollector
	obsAlertDispatcher     *observability.AlertDispatcher
	projectStudioRepo      cuanDB.ProjectStudioRepository
	batchCampaignRepo      cuanDB.BatchCampaignRepository
	agentRepo              cuanDB.AgentRepository
	blueprintEngine        *creator.BlueprintEngine
	taskExecutionEngine    *creator.TaskExecutionEngine
	inspectorEngine        *inspector.Engine
	providerRegistry       *agent.ProviderRegistry
	distillerEngine        *studio.DistillerEngine
	agentExecutionHook     studio.AgentExecutionHook
	campaignDecompiler     *campaign.DecompilerEngine
	campaignRunner         *campaign.MultiProjectRunner
	campaignAggregator     *campaign.RollupAggregator
	inboxSlots             chan struct{}
	inboxStopping          atomic.Bool
	inboxWorkers           sync.Map
	taskLaunches           sync.Map
	taskAgentGrants        sync.Map
	taskAgentURL           string
	inboxCancels           sync.Map
	githubDeviceMu         sync.Mutex
	githubDeviceJobs       map[string]*githubDeviceLogin
	roomEventBroker        *RoomEventBroker
	terminalManager        *terminal.Manager
	terminalHandler        *terminal.Handler
	projectIntegrationRepo cuanDB.ProjectIntegrationRepository
	publisherEngine        *publisher.Engine
	scriptDeployRepo       cuanDB.ScriptDeploymentRepository
	provisionedDBRepo      cuanDB.ProvisionedDatabaseRepository
	provisionerEngine      *deployer.DatabaseProvisioner
	scriptRunner           *deployer.ScriptRunner
	processSupervisor      *deployer.ProcessSupervisor
	wordpressRepo          cuanDB.WordPressRepository
	wordpressOrchestrator  *wordpress.Orchestrator
	wordpressWebmaster     *wordpress.WebmasterEngine
	projectRepo            cuanDB.ProjectRepository
	taskProjectRepo        cuanDB.TaskProjectRepository
	agentSessionStore      agent.SessionStore
	router                 *chi.Mux
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
	if _, err := pSvc.EnsureDefault(context.Background()); err != nil {
		log.Print("Default project initialization unavailable")
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

	obsAlertDispatcher := observability.NewAlertDispatcher(
		obsRepo,
		observability.WithAlertKeyVault(vault),
		observability.WithAlertBaseURL(fmt.Sprintf("https://%s", baseHost)),
		observability.WithProjectResolver(func(ctx context.Context, targetType, targetID string) string {
			if targetType == "deployment" && depStore != nil {
				dep, err := depStore.GetDeployment(ctx, targetID)
				if err == nil && dep != nil && pSvc != nil {
					projects, err := pSvc.List(ctx)
					if err == nil {
						for _, p := range projects {
							if p.Name == dep.ProjectName || p.RepoName == dep.Slug {
								return p.ID
							}
						}
						if len(projects) == 1 {
							return projects[0].ID
						}
					}
				}
			}
			return "default"
		}),
	)

	obsHealthPoller.SetProbeCallback(func(hc *cuanDB.HealthCheck, res *observability.ProbeResult) {
		_ = obsAlertDispatcher.HandleHealthProbe(context.Background(), hc, res)
	})
	obsMetricsCollector.SetMetricCallback(func(metric *cuanDB.ServerMetric) {
		_ = obsAlertDispatcher.HandleServerMetric(context.Background(), metric)
	})

	var studioRepo cuanDB.ProjectStudioRepository
	if db != nil {
		studioRepo = cuanDB.NewSQLProjectStudioRepository(db)
	} else {
		studioRepo = cuanDB.NewMemoryProjectStudioRepository()
	}

	var batchRepo cuanDB.BatchCampaignRepository
	if db != nil {
		batchRepo = cuanDB.NewSQLBatchCampaignRepository(db)
	} else {
		batchRepo = cuanDB.NewMemoryBatchCampaignRepository()
	}

	var agentRepo cuanDB.AgentRepository
	if db != nil {
		agentRepo = cuanDB.NewSQLAgentRepository(db)
	} else {
		agentRepo = cuanDB.NewMemoryAgentRepository()
	}

	var hookDispatcher studio.ExecutionDispatcher
	if tStore != nil {
		hookDispatcher = studio.NewRunnerTaskBridgeDispatcher(tStore)
	}
	agentHook := studio.NewAgentExecutionHook(hookDispatcher)
	distiller := studio.NewDistillerEngine(studio.WithExecutionHook(agentHook))

	campaignResolver := campaign.NewDefaultProjectResolver(pSvc, studioRepo)
	campaignDecompiler := campaign.NewDecompilerEngine(
		campaign.WithProjectResolver(campaignResolver),
		campaign.WithProjectStudioRepo(studioRepo),
		campaign.WithMinDoDItems(3),
	)
	campaignAggregator := campaign.NewRollupAggregator(
		campaign.WithAggregatorStudioRepo(studioRepo),
		campaign.WithAggregatorBatchRepo(batchRepo),
	)
	campaignRunner := campaign.NewMultiProjectRunner(
		campaign.WithMaxWorkers(campaign.DefaultMaxWorkers),
		campaign.WithRunnerBatchRepo(batchRepo),
		campaign.WithRunnerStudioRepo(studioRepo),
		campaign.WithDecompiler(campaignDecompiler),
		campaign.WithRunnerAggregator(campaignAggregator),
	)

	workspacesDir := cfg.WorkspacesDir
	if workspacesDir == "" {
		workspacesDir = filepath.Join("data", "workspaces")
	}
	termMgr := terminal.NewManager(terminal.ManagerConfig{
		WorkspacesDir:         workspacesDir,
		DefaultShell:          "",
		DefaultCols:           80,
		DefaultRows:           24,
		IdleTimeout:           15 * time.Minute,
		DisconnectGracePeriod: 30 * time.Second,
	})
	termHandler := terminal.NewHandler(terminal.HandlerConfig{
		Manager:     termMgr,
		AttachOnly:  true,
		AuthService: svc,
	})

	var bpEngine *creator.BlueprintEngine
	var inspEngine *inspector.Engine
	if pSvc != nil {
		bpEngine = creator.NewBlueprintEngine(
			creator.WithProjectStore(pSvc.Store()),
			creator.WithStudioRepo(studioRepo),
			creator.WithAgentRepo(agentRepo),
		)
		inspEngine = inspector.NewEngine(
			inspector.WithProjectStore(pSvc.Store()),
			inspector.WithStudioRepo(studioRepo),
			inspector.WithAgentRepo(agentRepo),
			inspector.WithBaseDir(projectBaseDir),
		)
	}

	roomBroker := NewRoomEventBroker()
	var pStore project.Store
	if pSvc != nil {
		pStore = pSvc.Store()
	}
	taskExecEngine := creator.NewTaskExecutionEngine(
		creator.WithExecutionStudioRepo(studioRepo),
		creator.WithExecutionAgentRepo(agentRepo),
		creator.WithExecutionProjectStore(pStore),
		creator.WithEventBroadcaster(roomBroker),
	)

	var intRepo cuanDB.ProjectIntegrationRepository
	if db != nil {
		intRepo = cuanDB.NewSQLProjectIntegrationRepository(db)
	} else {
		intRepo = cuanDB.NewMemoryProjectIntegrationRepository()
	}

	pubSecret := cfg.AuthSecret
	if pubSecret == "" {
		pubSecret = "cuan-default-publisher-secret-key-32b"
	}
	pubEngine := publisher.NewEngine(
		publisher.WithStudioRepo(studioRepo),
		publisher.WithIntegrationRepo(intRepo),
		publisher.WithSecretKey(pubSecret),
	)

	var scriptDeployRepo cuanDB.ScriptDeploymentRepository
	var provisionedDBRepo cuanDB.ProvisionedDatabaseRepository
	if db != nil {
		scriptDeployRepo = cuanDB.NewSQLScriptDeploymentRepository(db)
		provisionedDBRepo = cuanDB.NewSQLProvisionedDatabaseRepository(db)
	} else {
		scriptDeployRepo = cuanDB.NewMemoryScriptDeploymentRepository()
		provisionedDBRepo = cuanDB.NewMemoryProvisionedDatabaseRepository()
	}

	provSecret := cfg.AuthSecret
	if provSecret == "" {
		provSecret = "cuan-default-database-provisioner-vault-key"
	}
	provEngine := deployer.NewDatabaseProvisioner(deployer.ProvisionerConfig{
		DBRepo:           provisionedDBRepo,
		VaultSecret:      provSecret,
		ProjectBaseDir:   projectBaseDir,
		PostgresAdminURL: cfg.DatabaseURL,
	})

	scriptRunner := deployer.NewScriptRunner()
	processSupervisor := deployer.NewProcessSupervisor(deployer.ProcessSupervisorConfig{
		PortPool:          portPool,
		Router:            caddyRouter,
		Repo:              scriptDeployRepo,
		ProvisionedDBRepo: provisionedDBRepo,
		ServerRepo:        srvRepo,
		SSHManager:        sshMgr,
		KeyVault:          vault,
		ProjectBaseDir:    projectBaseDir,
		DeploymentsDir:    deploymentsDir,
		VaultSecret:       provSecret,
	})

	var wpRepo cuanDB.WordPressRepository
	if db != nil {
		wpRepo = cuanDB.NewSQLWordPressRepository(db)
	} else {
		wpRepo = cuanDB.NewMemoryWordPressRepository()
	}

	wpDeployDir := filepath.Join(deploymentsDir, "wordpress")
	wpCacheDir := filepath.Join("data", "wordpress", "cache")
	wpOrchestrator := wordpress.NewOrchestrator(wordpress.Config{
		DBRepo:        wpRepo,
		DBProvisioner: provEngine,
		PortPool:      portPool,
		Router:        caddyRouter,
		VaultSecret:   provSecret,
		BaseDeployDir: wpDeployDir,
		CoreCacheDir:  wpCacheDir,
	})

	wpWebmaster := wordpress.NewWebmasterEngine(wordpress.WebmasterConfig{
		DBRepo:        wpRepo,
		BaseDeployDir: wpDeployDir,
	})

	var projRepo cuanDB.ProjectRepository
	var tpRepo cuanDB.TaskProjectRepository
	if db != nil {
		projRepo = cuanDB.NewSQLProjectRepository(db)
		tpRepo = cuanDB.NewSQLTaskProjectRepository(db)
	} else {
		projRepo = cuanDB.NewMemoryProjectRepository()
		tpRepo = cuanDB.NewMemoryTaskProjectRepository()
	}

	s := &Server{
		extensions:             extensionSDK.New(),
		crm:                    &crmStore{contacts: make(map[string]CRMContact)},
		canned:                 newCannedStore(),
		cfg:                    cfg,
		db:                     db,
		authSvc:                svc,
		projectSvc:             pSvc,
		taskSvc:                tSvc,
		runnerSvc:              rSvc,
		sponsorshipSvc:         spSvc,
		deployerDriver:         depDriver,
		serverRepo:             srvRepo,
		domainRepo:             domRepo,
		dnsVerifier:            dnsVerifier,
		dnsPoller:              dnsPoller,
		keyVault:               vault,
		sshManager:             sshMgr,
		healthPoller:           healthPoller,
		caddyRouter:            caddyRouter,
		obsRepo:                obsRepo,
		obsHealthPoller:        obsHealthPoller,
		obsMetricsCollector:    obsMetricsCollector,
		obsAlertDispatcher:     obsAlertDispatcher,
		projectStudioRepo:      studioRepo,
		batchCampaignRepo:      batchRepo,
		agentRepo:              agentRepo,
		blueprintEngine:        bpEngine,
		taskExecutionEngine:    taskExecEngine,
		inspectorEngine:        inspEngine,
		providerRegistry:       agent.DefaultRegistry(),
		distillerEngine:        distiller,
		agentExecutionHook:     agentHook,
		campaignDecompiler:     campaignDecompiler,
		campaignRunner:         campaignRunner,
		campaignAggregator:     campaignAggregator,
		roomEventBroker:        roomBroker,
		inboxSlots:             make(chan struct{}, 4),
		terminalManager:        termMgr,
		terminalHandler:        termHandler,
		projectIntegrationRepo: intRepo,
		publisherEngine:        pubEngine,
		scriptDeployRepo:       scriptDeployRepo,
		provisionedDBRepo:      provisionedDBRepo,
		provisionerEngine:      provEngine,
		scriptRunner:           scriptRunner,
		processSupervisor:      processSupervisor,
		wordpressRepo:          wpRepo,
		wordpressOrchestrator:  wpOrchestrator,
		wordpressWebmaster:     wpWebmaster,
		projectRepo:            projRepo,
		taskProjectRepo:        tpRepo,
		agentSessionStore:      agent.NewMemorySessionStore(),
		router:                 r,
	}

	tSvc.SetProjectFinder(liveProjectFinder{server: s})
	s.configureSharedGitHubAccess()
	s.routes()
	s.setupStaticRoutes()
	s.SetExtensionRegistry(s.extensions)
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
	s.configureSharedGitHubAccess()
}

func (s *Server) ProjectRepository() cuanDB.ProjectRepository {
	return s.projectRepo
}

func (s *Server) SetProjectRepository(r cuanDB.ProjectRepository) {
	s.projectRepo = r
}

func (s *Server) TaskProjectRepository() cuanDB.TaskProjectRepository {
	return s.taskProjectRepo
}

func (s *Server) SetTaskProjectRepository(r cuanDB.TaskProjectRepository) {
	s.taskProjectRepo = r
}

func (s *Server) TaskService() *task.Service {
	return s.taskSvc
}

func (s *Server) SetTaskService(svc *task.Service) {
	s.taskSvc = svc
	if svc != nil {
		svc.SetExtensionRegistry(s.extensions)
	}
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

func (s *Server) ObservabilityAlertDispatcher() *observability.AlertDispatcher {
	return s.obsAlertDispatcher
}

func (s *Server) SetObservabilityAlertDispatcher(d *observability.AlertDispatcher) {
	s.obsAlertDispatcher = d
}

func (s *Server) ProjectStudioRepository() cuanDB.ProjectStudioRepository {
	return s.projectStudioRepo
}

func (s *Server) SetProjectStudioRepository(repo cuanDB.ProjectStudioRepository) {
	s.projectStudioRepo = repo
}

func (s *Server) BatchCampaignRepository() cuanDB.BatchCampaignRepository {
	return s.batchCampaignRepo
}

func (s *Server) SetBatchCampaignRepository(repo cuanDB.BatchCampaignRepository) {
	s.batchCampaignRepo = repo
}

func (s *Server) AgentRepository() cuanDB.AgentRepository {
	return s.agentRepo
}

func (s *Server) SetAgentRepository(repo cuanDB.AgentRepository) {
	s.agentRepo = repo
}

func (s *Server) BlueprintEngine() *creator.BlueprintEngine {
	return s.blueprintEngine
}

func (s *Server) SetBlueprintEngine(e *creator.BlueprintEngine) {
	s.blueprintEngine = e
}

func (s *Server) TaskExecutionEngine() *creator.TaskExecutionEngine {
	return s.taskExecutionEngine
}

func (s *Server) SetTaskExecutionEngine(e *creator.TaskExecutionEngine) {
	s.taskExecutionEngine = e
}

func (s *Server) InspectorEngine() *inspector.Engine {
	return s.inspectorEngine
}

func (s *Server) SetInspectorEngine(e *inspector.Engine) {
	s.inspectorEngine = e
}

func (s *Server) ScriptDeploymentRepository() cuanDB.ScriptDeploymentRepository {
	return s.scriptDeployRepo
}

func (s *Server) SetScriptDeploymentRepository(repo cuanDB.ScriptDeploymentRepository) {
	s.scriptDeployRepo = repo
}

func (s *Server) ProvisionedDatabaseRepository() cuanDB.ProvisionedDatabaseRepository {
	return s.provisionedDBRepo
}

func (s *Server) SetProvisionedDatabaseRepository(repo cuanDB.ProvisionedDatabaseRepository) {
	s.provisionedDBRepo = repo
}

func (s *Server) ProvisionerEngine() *deployer.DatabaseProvisioner {
	return s.provisionerEngine
}

func (s *Server) SetProvisionerEngine(e *deployer.DatabaseProvisioner) {
	s.provisionerEngine = e
}

func (s *Server) ScriptRunner() *deployer.ScriptRunner {
	return s.scriptRunner
}

func (s *Server) SetScriptRunner(r *deployer.ScriptRunner) {
	s.scriptRunner = r
}

func (s *Server) ProcessSupervisor() *deployer.ProcessSupervisor {
	return s.processSupervisor
}

func (s *Server) SetProcessSupervisor(ps *deployer.ProcessSupervisor) {
	s.processSupervisor = ps
}

func (s *Server) ProviderRegistry() *agent.ProviderRegistry {
	if s.providerRegistry == nil {
		s.providerRegistry = agent.DefaultRegistry()
	}
	return s.providerRegistry
}

func (s *Server) SetProviderRegistry(r *agent.ProviderRegistry) {
	s.providerRegistry = r
}

func (s *Server) CampaignDecompiler() *campaign.DecompilerEngine {
	return s.campaignDecompiler
}

func (s *Server) SetCampaignDecompiler(d *campaign.DecompilerEngine) {
	s.campaignDecompiler = d
}

func (s *Server) CampaignRunner() *campaign.MultiProjectRunner {
	return s.campaignRunner
}

func (s *Server) SetCampaignRunner(r *campaign.MultiProjectRunner) {
	s.campaignRunner = r
}

func (s *Server) CampaignAggregator() *campaign.RollupAggregator {
	return s.campaignAggregator
}

func (s *Server) SetCampaignAggregator(a *campaign.RollupAggregator) {
	s.campaignAggregator = a
	if s.campaignRunner != nil && a != nil {
		s.campaignRunner.AddListener(a)
	}
}

func (s *Server) RoomEventBroker() *RoomEventBroker {
	return s.roomEventBroker
}

func (s *Server) SetRoomEventBroker(b *RoomEventBroker) {
	s.roomEventBroker = b
}

func (s *Server) DistillerEngine() *studio.DistillerEngine {
	return s.distillerEngine
}

func (s *Server) SetDistillerEngine(e *studio.DistillerEngine) {
	s.distillerEngine = e
}

func (s *Server) AgentExecutionHook() studio.AgentExecutionHook {
	return s.agentExecutionHook
}

func (s *Server) SetAgentExecutionHook(h studio.AgentExecutionHook) {
	s.agentExecutionHook = h
}

func (s *Server) TerminalManager() *terminal.Manager {
	return s.terminalManager
}

func (s *Server) SetTerminalManager(m *terminal.Manager) {
	s.terminalManager = m
	if m != nil {
		s.terminalHandler = terminal.NewHandler(terminal.HandlerConfig{
			Manager:     m,
			AttachOnly:  true,
			AuthService: s.authSvc,
		})
	}
}

func (s *Server) TerminalHandler() *terminal.Handler {
	return s.terminalHandler
}

func (s *Server) SetTerminalHandler(h *terminal.Handler) {
	s.terminalHandler = h
}

func (s *Server) AgentSessionStore() agent.SessionStore {
	return s.agentSessionStore
}

func (s *Server) SetAgentSessionStore(store agent.SessionStore) {
	s.agentSessionStore = store
}

func (s *Server) PublisherEngine() *publisher.Engine {
	return s.publisherEngine
}

func (s *Server) SetPublisherEngine(e *publisher.Engine) {
	s.publisherEngine = e
}

func (s *Server) ProjectIntegrationRepository() cuanDB.ProjectIntegrationRepository {
	return s.projectIntegrationRepo
}

func (s *Server) SetProjectIntegrationRepository(r cuanDB.ProjectIntegrationRepository) {
	s.projectIntegrationRepo = r
	if s.publisherEngine != nil {
		s.publisherEngine = publisher.NewEngine(
			publisher.WithStudioRepo(s.projectStudioRepo),
			publisher.WithIntegrationRepo(r),
			publisher.WithSecretKey(s.cfg.AuthSecret),
		)
	}
}

func (s *Server) WordPressRepository() cuanDB.WordPressRepository {
	return s.wordpressRepo
}

func (s *Server) SetWordPressRepository(r cuanDB.WordPressRepository) {
	s.wordpressRepo = r
}

func (s *Server) WordPressOrchestrator() *wordpress.Orchestrator {
	return s.wordpressOrchestrator
}

func (s *Server) SetWordPressOrchestrator(o *wordpress.Orchestrator) {
	s.wordpressOrchestrator = o
}

func (s *Server) WordPressWebmaster() *wordpress.WebmasterEngine {
	return s.wordpressWebmaster
}

func (s *Server) SetWordPressWebmaster(w *wordpress.WebmasterEngine) {
	s.wordpressWebmaster = w
}

func (s *Server) routes() {
	s.router.Get("/api/v1/task-agent/{id}", s.handleTaskAgent)
	s.router.Post("/api/v1/task-agent/{id}", s.handleTaskAgent)
 s.router.Post("/api/v1/task-agent/{id}/push", s.handleTaskAgent)
	// Public Health, Readiness & System Mode (Sprint 3 S03.5)
	s.router.Get("/healthz", s.handleHealthz)
	s.router.Get("/readyz", s.handleReadyz)
	s.router.Get("/api/info", s.handleInfo)
	s.router.Get("/api/mode", s.handleGetSystemMode)
	s.router.Get("/api/system/mode", s.handleGetSystemMode)
	s.router.Get("/install.sh", s.handleInstallScript)

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

	// Incident Alert Dispatcher & Multi-Channel Notification Engine Endpoints (Sprint 6 S06.3)
	s.router.Post("/api/projects/{id}/alert-channels", s.handleCreateAlertChannel)
	s.router.Get("/api/projects/{id}/alert-channels", s.handleListAlertChannels)
	s.router.Post("/api/alert-channels/{id}/test", s.handleTestAlertChannel)
	s.router.Delete("/api/alert-channels/{id}", s.handleDeleteAlertChannel)
	s.router.Get("/api/incidents", s.handleListIncidents)

	// Project Studio Discussions, Messages, Tasks & Real-Time SSE (Sprint 7 S07.2)
	// Project Rooms
	s.router.Post("/api/projects/{id}/rooms", s.handleCreateProjectRoom)
	s.router.Get("/api/projects/{id}/rooms", s.handleListProjectRooms)
	s.router.Get("/api/rooms/{id}", s.handleGetProjectRoom)
	s.router.Put("/api/rooms/{id}", s.handleUpdateProjectRoom)
	s.router.Delete("/api/rooms/{id}", s.handleDeleteProjectRoom)

	// Project Rooms alias routes (/api/projects/rooms/{id})
	s.router.Get("/api/projects/rooms/{id}", s.handleGetProjectRoom)
	s.router.Put("/api/projects/rooms/{id}", s.handleUpdateProjectRoom)
	s.router.Delete("/api/projects/rooms/{id}", s.handleDeleteProjectRoom)

	// Messages
	s.router.Post("/api/rooms/{id}/messages", s.handleCreateProjectMessage)
	s.router.Get("/api/rooms/{id}/messages", s.handleListProjectMessages)
	s.router.Post("/api/projects/rooms/{id}/messages", s.handleCreateProjectMessage)
	s.router.Get("/api/projects/rooms/{id}/messages", s.handleListProjectMessages)
	s.router.Delete("/api/messages/{id}", s.handleDeleteProjectMessage)

	// Real-Time Event Streaming Engine (SSE)
	s.router.Get("/api/rooms/{id}/events", s.handleStreamRoomEvents)
	s.router.Get("/api/projects/rooms/{id}/events", s.handleStreamRoomEvents)

	// Discussion-to-Task Distiller Engine
	s.router.Post("/api/rooms/{id}/distill", s.handleDistillTaskFromRoom)
	s.router.Post("/api/projects/rooms/{id}/distill", s.handleDistillTaskFromRoom)
	// Conversational Inbox Engine & Thread Management (Sprint 17 S17.2 & S17.3)
	s.router.Get("/api/inbox", s.handleListInboxThreads)
	s.router.Post("/api/inbox", s.handleCreateInboxThread)
	s.router.Get("/api/inbox/{id}", s.handleGetInboxThread)
	s.router.Post("/api/inbox/{id}/messages", s.handleCreateInboxMessage)
	s.router.Post("/api/inbox/{id}/approve", s.handleApproveInboxThread)

	// Direct Messenger Hub & Dojo Live Chat Replacement (Sprint 18)
	// Dojo Drop-In Widget JS
	s.router.With(chatCORSMiddleware).Get("/chat/widget.js", s.handleChatWidgetJS)
	s.router.With(chatCORSMiddleware).Options("/chat/widget.js", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Visitor Live Chat API (Dojo contract compatible, open CORS)
	s.router.Route("/api/chat", func(cr chi.Router) {
		cr.Use(chatCORSMiddleware)
		cr.Post("/session", s.handleVisitorChatSession)
		cr.Get("/messages", s.handleVisitorGetMessages)
		cr.Post("/messages", s.handleVisitorSendMessage)
		cr.Post("/attachments", s.handleVisitorUploadAttachment)
		cr.Get("/attachments/{id}", s.handleVisitorDownloadAttachment)
		cr.Post("/sessions/{uuid}/page", s.handleVisitorPageTelemetry)
		cr.Post("/receipts", s.handleVisitorReceipts)
		cr.Get("/contacts", s.handleInternalListContacts)
		cr.Get("/canned-responses", s.handleListCannedResponses)
		cr.Get("/conversations/{id}/messages", s.handleInternalGetConversationMessages)
		cr.Post("/conversations/{id}/messages", s.handleInternalSendConversationMessage)
		cr.Get("/conversations/{id}/events", s.handleInboxStream)
		cr.Post("/conversations/{id}/messages/{messageID}/task", s.handleInboxMessageTask)
		cr.Post("/conversations/{id}/requests/{requestID}/cancel", s.handleCancelInboxRequest)
		cr.Post("/conversations/{id}/requests/{requestID}/retry", s.handleRetryInboxRequest)
	})

	// Project Tasks
	s.router.Post("/api/projects/{id}/tasks", s.handleCreateProjectTask)
	s.router.Get("/api/projects/{id}/tasks", s.handleListProjectTasks)
	s.router.Get("/api/tasks/{id}", s.handleGetProjectTask)
	s.router.Put("/api/tasks/{id}", s.handleUpdateProjectTask)
	s.router.Patch("/api/tasks/{id}/status", s.handlePatchProjectTaskStatus)
	s.router.Delete("/api/tasks/{id}", s.handleDeleteProjectTask)

	// Project Tasks alias routes (/api/projects/tasks/{id})
	s.router.Get("/api/projects/tasks/{id}", s.handleGetProjectTask)
	s.router.Put("/api/projects/tasks/{id}", s.handleUpdateProjectTask)
	s.router.Patch("/api/projects/tasks/{id}/status", s.handlePatchProjectTaskStatus)
	s.router.Delete("/api/projects/tasks/{id}", s.handleDeleteProjectTask)

	// In-Task Agentic Execution Engine (Sprint 10 S10.3)
	s.router.Post("/api/tasks/{id}/execute-agent", s.handleExecuteTaskAgent)
	s.router.Post("/api/projects/tasks/{id}/execute-agent", s.handleExecuteTaskAgent)
	s.router.Post("/api/v1/tasks/{id}/execute-agent", s.handleExecuteTaskAgent)
	s.router.Post("/api/v1/projects/tasks/{id}/execute-agent", s.handleExecuteTaskAgent)
	s.router.Post("/api/tasks/{id}/apply", s.handleApplyTask)
	s.router.Post("/api/projects/tasks/{id}/apply", s.handleApplyTask)
	s.router.Post("/api/v1/tasks/{id}/apply", s.handleApplyTask)
	s.router.Post("/api/v1/projects/tasks/{id}/apply", s.handleApplyTask)

	// WebSocket Pseudo-Terminal (PTY) Engine (Sprint 9 S09.3)
	s.router.Get("/api/tasks/{id}/terminal", s.handleTaskTerminalWebSocket)
	s.router.Delete("/api/tasks/{id}/terminal", s.handleCloseTaskTerminalSession)
	s.router.Get("/api/projects/tasks/{id}/terminal", s.handleTaskTerminalWebSocket)
	s.router.Delete("/api/projects/tasks/{id}/terminal", s.handleCloseTaskTerminalSession)
	s.router.Get("/api/v1/tasks/{id}/terminal", s.handleTaskTerminalWebSocket)
	s.router.Delete("/api/v1/tasks/{id}/terminal", s.handleCloseTaskTerminalSession)
	s.router.Get("/api/v1/projects/tasks/{id}/terminal", s.handleTaskTerminalWebSocket)
	s.router.Delete("/api/v1/projects/tasks/{id}/terminal", s.handleCloseTaskTerminalSession)
	s.router.Get("/api/terminal/sessions", s.handleGetTerminalSessionsStatus)
	s.router.Get("/api/v1/terminal/sessions", s.handleGetTerminalSessionsStatus)

	// Autonomous PTY Agent Task Runner (Sprint 22 S22.2)
	s.router.Post("/api/tasks/{id}/run-agent", s.handleRunTaskAgent)
	s.router.Post("/api/projects/tasks/{id}/run-agent", s.handleRunTaskAgent)
	s.router.Post("/api/v1/tasks/{id}/run-agent", s.handleRunTaskAgent)
	s.router.Post("/api/v1/projects/tasks/{id}/run-agent", s.handleRunTaskAgent)
	s.router.Get("/api/tasks/{id}/run-agent/history", s.handleGetTaskRunHistory)
	s.router.Get("/api/v1/tasks/{id}/run-agent/history", s.handleGetTaskRunHistory)

	// Agent CLI Account Onboarding & Host Detection (Sprint 22 S22.1)
	s.router.Get("/api/agents/binaries", s.handleGetAgentBinaries)
	s.router.Get("/api/v1/agents/binaries", s.handleGetAgentBinaries)
	s.router.Post("/api/agents/pair", s.handlePairAgentDevice)
	s.router.Post("/api/v1/agents/pair", s.handlePairAgentDevice)
	s.router.Get("/api/agents/sessions", s.handleListAgentSessions)
	s.router.Get("/api/v1/agents/sessions", s.handleListAgentSessions)
	s.router.Delete("/api/agents/sessions/{id}", s.handleRevokeAgentSession)
	s.router.Delete("/api/v1/agents/sessions/{id}", s.handleRevokeAgentSession)

	// Batch Campaigns Engine Endpoints (Sprint 8 S08.2 & S08.3)
	s.router.Post("/api/campaigns", s.handleCreateBatchCampaign)
	s.router.Get("/api/campaigns", s.handleListBatchCampaigns)
	s.router.Get("/api/campaigns/{id}", s.handleGetBatchCampaign)
	s.router.Get("/api/campaigns/{id}/rollup", s.handleGetCampaignRollup)
	s.router.Get("/api/campaigns/{id}/events", s.handleCampaignEventsSSE)
	s.router.Post("/api/campaigns/{id}/execute", s.handleExecuteBatchCampaign)

	// Agent Personas & Connection Engine Endpoints (Sprint 9 S09.2)
	s.router.Get("/api/agents", s.handleListAgents)
	s.router.Post("/api/agents", s.handleCreateAgent)
	s.router.Get("/api/agents/auth-status", s.handleGetHarnessAuthStatus)
	s.router.Post("/api/agents/auth-init", s.handleInitHarnessAuth)
	s.router.Post("/api/agents/auth-code", s.handleSubmitHarnessAuthCode)
	s.router.Get("/api/auth-status", s.handleGetHarnessAuthStatus)
	s.router.Post("/api/auth-init", s.handleInitHarnessAuth)
	s.router.Post("/api/auth-code", s.handleSubmitHarnessAuthCode)
	s.router.Post("/auth-init", s.handleInitHarnessAuth)
	s.router.Post("/auth-code", s.handleSubmitHarnessAuthCode)
	s.router.Post("/api/agents/connect-cli", s.handleConnectCLIAgent)
	s.router.Get("/api/agents/{id}", s.handleGetAgent)
	s.router.Put("/api/agents/{id}", s.handleUpdateAgent)
	s.router.Patch("/api/agents/{id}", s.handlePatchAgent)
	s.router.Delete("/api/agents/{id}", s.handleDeleteAgent)

	s.router.Get("/api/agent-connections", s.handleListAgentConnections)
	s.router.Post("/api/agent-connections", s.handleCreateAgentConnection)
	s.router.Get("/api/agent-connections/{id}", s.handleGetAgentConnection)
	s.router.Delete("/api/agent-connections/{id}", s.handleDeleteAgentConnection)
	s.router.Post("/api/agent-connections/{id}/verify", s.handleVerifyAgentConnection)

	// Backward-compatibility aliases for /api/agents/connections
	s.router.Get("/api/agents/connections", s.handleListAgentConnections)
	s.router.Post("/api/agents/connections", s.handleCreateAgentConnection)
	s.router.Get("/api/agents/connections/{id}", s.handleGetAgentConnection)
	s.router.Delete("/api/agents/connections/{id}", s.handleDeleteAgentConnection)
	s.router.Post("/api/agents/connections/{id}/verify", s.handleVerifyAgentConnection)

	// Creator Blueprint Engine Endpoints (Sprint 10 S10.2)
	s.router.Get("/api/creator/templates", s.handleListCreatorTemplates)
	s.router.Get("/api/creator/templates/{id}", s.handleGetCreatorTemplate)
	s.router.Post("/api/creator/projects/instantiate", s.handleInstantiateCreatorProject)
	s.router.Get("/api/v1/creator/templates", s.handleListCreatorTemplates)
	s.router.Get("/api/v1/creator/templates/{id}", s.handleGetCreatorTemplate)
	s.router.Post("/api/v1/creator/projects/instantiate", s.handleInstantiateCreatorProject)

	// AI Script Inspector & ZIP Uploader Endpoints (Sprint 11 S11.2)
	s.router.Post("/api/projects/from-zip", s.handleCreateProjectFromZip)
	s.router.Post("/api/v1/projects/from-zip", s.handleCreateProjectFromZip)
	s.router.Post("/api/projects/inspect-zip", s.handleInspectZip)
	s.router.Post("/api/v1/projects/inspect-zip", s.handleInspectZip)
	s.router.Post("/api/inspector/inspect-zip", s.handleInspectZip)
	s.router.Post("/api/v1/inspector/inspect-zip", s.handleInspectZip)

	// WordPress CMS Publishing Engine & Multi-Format Exporter Endpoints (Sprint 11 S11.3)
	s.router.Post("/api/projects/{id}/integrations/wordpress", s.handleSetupWordPressIntegration)
	s.router.Get("/api/projects/{id}/integrations/wordpress", s.handleGetWordPressIntegration)
	s.router.Post("/api/v1/projects/{id}/integrations/wordpress", s.handleSetupWordPressIntegration)
	s.router.Get("/api/v1/projects/{id}/integrations/wordpress", s.handleGetWordPressIntegration)
	s.router.Post("/api/tasks/{id}/publish", s.handlePublishTask)
	s.router.Post("/api/projects/tasks/{id}/publish", s.handlePublishTask)
	s.router.Post("/api/v1/tasks/{id}/publish", s.handlePublishTask)
	s.router.Post("/api/v1/projects/tasks/{id}/publish", s.handlePublishTask)
	s.router.Get("/api/tasks/{id}/export", s.handleExportTask)
	s.router.Get("/api/projects/tasks/{id}/export", s.handleExportTask)
	s.router.Get("/api/v1/tasks/{id}/export", s.handleExportTask)
	s.router.Get("/api/v1/projects/tasks/{id}/export", s.handleExportTask)

	// Dynamic Project Modules Endpoints (Sprint 11 S11.4)
	s.router.Get("/api/projects", s.handleListProjects)
	s.router.Get("/api/projects/{id}", s.handleGetProject)
	s.router.Patch("/api/projects/{id}/modules", s.handleUpdateProjectModules)
	s.router.Put("/api/projects/{id}/modules", s.handleUpdateProjectModules)
	s.router.Patch("/api/v1/projects/{id}/modules", s.handleUpdateProjectModules)
	s.router.Put("/api/v1/projects/{id}/modules", s.handleUpdateProjectModules)

	// Project Archiving Endpoints (Sprint 14 S14.4)
	s.router.Patch("/api/projects/{id}/archive", s.handleArchiveProject)
	s.router.Post("/api/projects/{id}/archive", s.handleArchiveProject)
	s.router.Patch("/api/v1/projects/{id}/archive", s.handleArchiveProject)
	s.router.Post("/api/v1/projects/{id}/archive", s.handleArchiveProject)
	s.router.Patch("/api/projects/{id}/unarchive", s.handleUnarchiveProject)
	s.router.Post("/api/projects/{id}/unarchive", s.handleUnarchiveProject)
	s.router.Patch("/api/v1/projects/{id}/unarchive", s.handleUnarchiveProject)
	s.router.Post("/api/v1/projects/{id}/unarchive", s.handleUnarchiveProject)
	s.router.Patch("/api/projects/{id}/restore", s.handleUnarchiveProject)
	s.router.Post("/api/projects/{id}/restore", s.handleUnarchiveProject)
	s.router.Patch("/api/v1/projects/{id}/restore", s.handleUnarchiveProject)
	s.router.Post("/api/v1/projects/{id}/restore", s.handleUnarchiveProject)

	// Automated Database Provisioner & Environment Secrets Endpoints (Sprint 12 S12.2)
	s.router.Post("/api/projects/{id}/provision-db", s.handleProvisionProjectDatabase)
	s.router.Post("/api/v1/projects/{id}/provision-db", s.handleProvisionProjectDatabase)
	s.router.Post("/api/projects/{id}/generate-env", s.handleGenerateProjectEnv)
	s.router.Post("/api/v1/projects/{id}/generate-env", s.handleGenerateProjectEnv)
	s.router.Get("/api/projects/{id}/env", s.handleGetProjectEnv)
	s.router.Get("/api/v1/projects/{id}/env", s.handleGetProjectEnv)

	// Script Build Runner & Process Supervisor Endpoints (Sprint 12 S12.3)
	s.router.Post("/api/projects/{id}/script-deploy", s.handleScriptDeploy)
	s.router.Post("/api/v1/projects/{id}/script-deploy", s.handleScriptDeploy)
	s.router.Get("/api/projects/{id}/script-deploy/logs", s.handleScriptDeployLogs)
	s.router.Get("/api/v1/projects/{id}/script-deploy/logs", s.handleScriptDeployLogs)
	s.router.Post("/api/projects/{id}/script-deploy/restart", s.handleScriptDeployRestart)
	s.router.Post("/api/v1/projects/{id}/script-deploy/restart", s.handleScriptDeployRestart)
	s.router.Get("/api/projects/{id}/script-deploy/status", s.handleScriptDeployStatus)
	s.router.Get("/api/v1/projects/{id}/script-deploy/status", s.handleScriptDeployStatus)

	// Turnkey WordPress Cloud & Orchestrator Endpoints (Sprint 13 S13.2)
	s.router.Post("/api/projects/{id}/wordpress/deploy", s.handleWordPressDeploy)
	s.router.Post("/api/v1/projects/{id}/wordpress/deploy", s.handleWordPressDeploy)
	s.router.Get("/api/projects/{id}/wordpress", s.handleGetWordPressDeployment)
	s.router.Get("/api/v1/projects/{id}/wordpress", s.handleGetWordPressDeployment)
	s.router.Get("/api/projects/{id}/wordpress/credentials", s.handleGetWordPressCredentials)
	s.router.Get("/api/v1/projects/{id}/wordpress/credentials", s.handleGetWordPressCredentials)
	s.router.Post("/api/projects/{id}/wordpress/restart", s.handleRestartWordPressDeployment)
	s.router.Post("/api/v1/projects/{id}/wordpress/restart", s.handleRestartWordPressDeployment)
	s.router.Get("/api/projects/{id}/wordpress/health", s.handleHealthCheckWordPressDeployment)
	s.router.Get("/api/v1/projects/{id}/wordpress/health", s.handleHealthCheckWordPressDeployment)
	s.router.Post("/api/projects/{id}/wordpress/stop", s.handleStopWordPressDeployment)
	s.router.Post("/api/v1/projects/{id}/wordpress/stop", s.handleStopWordPressDeployment)
	s.router.Delete("/api/projects/{id}/wordpress", s.handleDeleteWordPressDeployment)
	s.router.Delete("/api/v1/projects/{id}/wordpress", s.handleDeleteWordPressDeployment)

	// AI Webmaster Engine Endpoints (Sprint 13 S13.3)
	// Assets (Themes & Plugins)
	s.router.Post("/api/projects/{id}/wordpress/assets/upload", s.handleUploadWordPressAsset)
	s.router.Post("/api/v1/projects/{id}/wordpress/assets/upload", s.handleUploadWordPressAsset)
	s.router.Get("/api/projects/{id}/wordpress/assets", s.handleListWordPressAssets)
	s.router.Get("/api/v1/projects/{id}/wordpress/assets", s.handleListWordPressAssets)
	s.router.Post("/api/projects/{id}/wordpress/assets/{asset_id}/status", s.handleToggleWordPressAssetStatus)
	s.router.Post("/api/v1/projects/{id}/wordpress/assets/{asset_id}/status", s.handleToggleWordPressAssetStatus)
	s.router.Delete("/api/projects/{id}/wordpress/assets/{asset_id}", s.handleDeleteWordPressAsset)
	s.router.Delete("/api/v1/projects/{id}/wordpress/assets/{asset_id}", s.handleDeleteWordPressAsset)

	// Safe Script Injector (mu-plugins)
	s.router.Post("/api/projects/{id}/wordpress/scripts", s.handleInjectWordPressScript)
	s.router.Post("/api/v1/projects/{id}/wordpress/scripts", s.handleInjectWordPressScript)
	s.router.Get("/api/projects/{id}/wordpress/scripts", s.handleListWordPressScripts)
	s.router.Get("/api/v1/projects/{id}/wordpress/scripts", s.handleListWordPressScripts)
	s.router.Post("/api/projects/{id}/wordpress/scripts/{script_id}/toggle", s.handleToggleWordPressScript)
	s.router.Post("/api/v1/projects/{id}/wordpress/scripts/{script_id}/toggle", s.handleToggleWordPressScript)
	s.router.Delete("/api/projects/{id}/wordpress/scripts/{script_id}", s.handleDeleteWordPressScript)
	s.router.Delete("/api/v1/projects/{id}/wordpress/scripts/{script_id}", s.handleDeleteWordPressScript)

	// Config Tweaker (wp-config.php)
	s.router.Post("/api/projects/{id}/wordpress/config", s.handleTweakWordPressConfig)
	s.router.Post("/api/v1/projects/{id}/wordpress/config", s.handleTweakWordPressConfig)
	s.router.Get("/api/projects/{id}/wordpress/config", s.handleGetWordPressConfig)
	s.router.Get("/api/v1/projects/{id}/wordpress/config", s.handleGetWordPressConfig)

	// AI Webmaster Audit Action History
	s.router.Get("/api/projects/{id}/wordpress/actions", s.handleListWordPressWebmasterActions)
	s.router.Get("/api/v1/projects/{id}/wordpress/actions", s.handleListWordPressWebmasterActions)

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

	// Batch Campaigns route group in /api/v1 (Sprint 8 S08.2 & S08.3)
	s.router.Route("/api/v1/campaigns", func(cr chi.Router) {
		cr.Get("/", s.handleListBatchCampaigns)
		cr.Post("/", s.handleCreateBatchCampaign)
		cr.Get("/{id}", s.handleGetBatchCampaign)
		cr.Get("/{id}/rollup", s.handleGetCampaignRollup)
		cr.Get("/{id}/events", s.handleCampaignEventsSSE)
		cr.Post("/{id}/execute", s.handleExecuteBatchCampaign)
	})

	s.router.Get("/api/v1/item-preferences", s.handleItemPreferences)
	s.router.Patch("/api/v1/item-preferences", s.handleItemPreferences)

	s.router.Route("/api/v1/provider-accounts", func(ar chi.Router) {
		ar.Get("/", s.handleListProviderAccounts)
		ar.Post("/", s.handleCreateProviderAccount)
		ar.Post("/{accountID}/auth-init", s.handleProviderAccountAuthInit)
		ar.Post("/{accountID}/auth-code", s.handleProviderAccountAuthCode)
		ar.Post("/{accountID}/verify", s.handleVerifyProviderAccount)
		ar.Patch("/{accountID}/failover", s.handleProviderAccountFailover)
		ar.Patch("/{accountID}/model-fallback", s.handleProviderAccountFailover)
		ar.Get("/{accountID}/usage", s.handleProviderAccountUsage)
	})

	// Agent Personas route group in /api/v1 (Sprint 9 S09.2)
	s.router.Route("/api/v1/agents", func(ar chi.Router) {
		ar.Get("/", s.handleListAgents)
		ar.Post("/", s.handleCreateAgent)
		ar.Get("/auth-status", s.handleGetHarnessAuthStatus)
		ar.Post("/auth-init", s.handleInitHarnessAuth)
		ar.Post("/auth-code", s.handleSubmitHarnessAuthCode)
		ar.Post("/connect-cli", s.handleConnectCLIAgent)
		ar.Post("/{id}/connection/verify", s.handleVerifyAgentProfile)
		ar.Get("/{id}", s.handleGetAgent)
		ar.Put("/{id}", s.handleUpdateAgent)
		ar.Patch("/{id}", s.handlePatchAgent)
		ar.Delete("/{id}", s.handleDeleteAgent)

		// Nested /api/v1/agents/connections
		ar.Get("/connections", s.handleListAgentConnections)
		ar.Post("/connections", s.handleCreateAgentConnection)
		ar.Get("/connections/{id}", s.handleGetAgentConnection)
		ar.Delete("/connections/{id}", s.handleDeleteAgentConnection)
		ar.Post("/connections/{id}/verify", s.handleVerifyAgentConnection)
	})

	// Agent Connections route group in /api/v1 (Sprint 9 S09.2)
	s.router.Route("/api/v1/agent-connections", func(cr chi.Router) {
		cr.Get("/", s.handleListAgentConnections)
		cr.Post("/", s.handleCreateAgentConnection)
		cr.Get("/{id}", s.handleGetAgentConnection)
		cr.Delete("/{id}", s.handleDeleteAgentConnection)
		cr.Post("/{id}/verify", s.handleVerifyAgentConnection)
	})

	s.router.Route("/api/v1", func(r chi.Router) {
		// Public endpoints
		r.Get("/integrations/github/oauth/callback", s.handleGitHubOAuthCallback)
		r.Get("/info", s.handleInfo)
		r.Get("/system/mode", s.handleGetSystemMode)
		r.Get("/auth/status", s.handleAuthStatus)
		r.Get("/auth-status", s.handleGetHarnessAuthStatus)
		r.Post("/auth-init", s.handleInitHarnessAuth)
		r.Post("/auth-code", s.handleSubmitHarnessAuthCode)
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

		// Observability Alert Channels & Incidents in v1
		r.Post("/projects/{id}/alert-channels", s.handleCreateAlertChannel)
		r.Get("/projects/{id}/alert-channels", s.handleListAlertChannels)
		r.Post("/alert-channels/{id}/test", s.handleTestAlertChannel)
		r.Delete("/alert-channels/{id}", s.handleDeleteAlertChannel)
		r.Get("/incidents", s.handleListIncidents)

		// Project Archiving in v1 (Sprint 14 S14.4)
		r.Patch("/projects/{id}/archive", s.handleArchiveProject)
		r.Post("/projects/{id}/archive", s.handleArchiveProject)
		r.Patch("/projects/{id}/unarchive", s.handleUnarchiveProject)
		r.Post("/projects/{id}/unarchive", s.handleUnarchiveProject)
		r.Patch("/projects/{id}/restore", s.handleUnarchiveProject)
		r.Post("/projects/{id}/restore", s.handleUnarchiveProject)

		// Project Studio Discussions, Messages, Tasks & Real-Time SSE in v1 (Sprint 7 S07.2)
		r.Post("/projects/{id}/rooms", s.handleCreateProjectRoom)
		r.Get("/projects/{id}/rooms", s.handleListProjectRooms)
		r.Get("/rooms/{id}", s.handleGetProjectRoom)
		r.Put("/rooms/{id}", s.handleUpdateProjectRoom)
		r.Delete("/rooms/{id}", s.handleDeleteProjectRoom)
		r.Get("/projects/rooms/{id}", s.handleGetProjectRoom)
		r.Put("/projects/rooms/{id}", s.handleUpdateProjectRoom)
		r.Delete("/projects/rooms/{id}", s.handleDeleteProjectRoom)

		r.Post("/rooms/{id}/messages", s.handleCreateProjectMessage)
		r.Get("/rooms/{id}/messages", s.handleListProjectMessages)
		r.Post("/projects/rooms/{id}/messages", s.handleCreateProjectMessage)
		r.Get("/projects/rooms/{id}/messages", s.handleListProjectMessages)
		r.Delete("/messages/{id}", s.handleDeleteProjectMessage)

		r.Get("/rooms/{id}/events", s.handleStreamRoomEvents)
		r.Get("/projects/rooms/{id}/events", s.handleStreamRoomEvents)

		r.Post("/rooms/{id}/distill", s.handleDistillTaskFromRoom)
		r.Post("/projects/rooms/{id}/distill", s.handleDistillTaskFromRoom)

		r.Post("/projects/{id}/tasks", s.handleCreateProjectTask)
		r.Get("/projects/{id}/tasks", s.handleListProjectTasks)
		r.Get("/projects/tasks/{id}", s.handleGetProjectTask)
		r.Put("/projects/tasks/{id}", s.handleUpdateProjectTask)
		r.Patch("/projects/tasks/{id}/status", s.handlePatchProjectTaskStatus)
		r.Delete("/projects/tasks/{id}", s.handleDeleteProjectTask)
		r.Patch("/tasks/{id}/status", s.handlePatchProjectTaskStatus)

		// In-Task Agentic Execution Engine in v1 (Sprint 10 S10.3)
		r.Post("/tasks/{id}/execute-agent", s.handleExecuteTaskAgent)
		r.Post("/projects/tasks/{id}/execute-agent", s.handleExecuteTaskAgent)
		r.Post("/tasks/{id}/apply", s.handleApplyTask)
		r.Post("/projects/tasks/{id}/apply", s.handleApplyTask)

		// WordPress CMS Integration & Task Publishing / Export in v1 (Sprint 11 S11.3)
		r.Post("/projects/{id}/integrations/wordpress", s.handleSetupWordPressIntegration)
		r.Get("/projects/{id}/integrations/wordpress", s.handleGetWordPressIntegration)

		// Automated Database Provisioner & Environment Secrets in v1 (Sprint 12 S12.2)
		r.Post("/projects/{id}/provision-db", s.handleProvisionProjectDatabase)
		r.Post("/projects/{id}/generate-env", s.handleGenerateProjectEnv)
		r.Get("/projects/{id}/env", s.handleGetProjectEnv)

		// Script Build Runner & Process Supervisor in v1 (Sprint 12 S12.3)
		r.Post("/projects/{id}/script-deploy", s.handleScriptDeploy)
		r.Get("/projects/{id}/script-deploy/logs", s.handleScriptDeployLogs)
		r.Post("/projects/{id}/script-deploy/restart", s.handleScriptDeployRestart)
		r.Get("/projects/{id}/script-deploy/status", s.handleScriptDeployStatus)

		// Turnkey WordPress Cloud & Orchestrator Endpoints in v1 (Sprint 13 S13.2)
		r.Post("/projects/{id}/wordpress/deploy", s.handleWordPressDeploy)
		r.Get("/projects/{id}/wordpress", s.handleGetWordPressDeployment)
		r.Get("/projects/{id}/wordpress/credentials", s.handleGetWordPressCredentials)
		r.Post("/projects/{id}/wordpress/restart", s.handleRestartWordPressDeployment)
		r.Get("/projects/{id}/wordpress/health", s.handleHealthCheckWordPressDeployment)
		r.Post("/projects/{id}/wordpress/stop", s.handleStopWordPressDeployment)
		r.Delete("/projects/{id}/wordpress", s.handleDeleteWordPressDeployment)

		// AI Webmaster Engine in v1 (Sprint 13 S13.3)
		r.Post("/projects/{id}/wordpress/assets/upload", s.handleUploadWordPressAsset)
		r.Get("/projects/{id}/wordpress/assets", s.handleListWordPressAssets)
		r.Post("/projects/{id}/wordpress/assets/{asset_id}/status", s.handleToggleWordPressAssetStatus)
		r.Delete("/projects/{id}/wordpress/assets/{asset_id}", s.handleDeleteWordPressAsset)

		r.Post("/projects/{id}/wordpress/scripts", s.handleInjectWordPressScript)
		r.Get("/projects/{id}/wordpress/scripts", s.handleListWordPressScripts)
		r.Post("/projects/{id}/wordpress/scripts/{script_id}/toggle", s.handleToggleWordPressScript)
		r.Delete("/projects/{id}/wordpress/scripts/{script_id}", s.handleDeleteWordPressScript)

		r.Post("/projects/{id}/wordpress/config", s.handleTweakWordPressConfig)
		r.Get("/projects/{id}/wordpress/config", s.handleGetWordPressConfig)

		r.Get("/projects/{id}/wordpress/actions", s.handleListWordPressWebmasterActions)
		r.Post("/tasks/{id}/publish", s.handlePublishTask)
		r.Post("/projects/tasks/{id}/publish", s.handlePublishTask)
		r.Get("/tasks/{id}/export", s.handleExportTask)
		r.Get("/projects/tasks/{id}/export", s.handleExportTask)

		// WebSocket Pseudo-Terminal (PTY) Engine in v1 (Sprint 9 S09.3)
		r.Get("/tasks/{id}/terminal", s.handleTaskTerminalWebSocket)
		r.Delete("/tasks/{id}/terminal", s.handleCloseTaskTerminalSession)
		r.Get("/projects/tasks/{id}/terminal", s.handleTaskTerminalWebSocket)
		r.Delete("/projects/tasks/{id}/terminal", s.handleCloseTaskTerminalSession)
		r.Get("/terminal/sessions", s.handleGetTerminalSessionsStatus)

		// Autonomous PTY Agent Task Runner (Sprint 22 S22.2)
		r.Post("/tasks/{id}/run-agent", s.handleRunTaskAgent)
		r.Post("/projects/tasks/{id}/run-agent", s.handleRunTaskAgent)
		r.Get("/tasks/{id}/run-agent/history", s.handleGetTaskRunHistory)

		// Agent CLI Account Onboarding & Host Detection (Sprint 22 S22.1)
		r.Get("/agents/binaries", s.handleGetAgentBinaries)
		r.Post("/agents/pair", s.handlePairAgentDevice)
		r.Get("/agents/sessions", s.handleListAgentSessions)
		r.Delete("/agents/sessions/{id}", s.handleRevokeAgentSession)

		// Batch Campaigns in v1 (Sprint 8 S08.2 & S08.3)
		r.Post("/campaigns", s.handleCreateBatchCampaign)
		r.Get("/campaigns", s.handleListBatchCampaigns)
		r.Get("/campaigns/{id}", s.handleGetBatchCampaign)
		r.Get("/campaigns/{id}/rollup", s.handleGetCampaignRollup)
		r.Get("/campaigns/{id}/events", s.handleCampaignEventsSSE)
		r.Post("/campaigns/{id}/execute", s.handleExecuteBatchCampaign)

		// Agent Personas in v1 (Sprint 9 S09.2)
		r.Get("/agents", s.handleListAgents)
		r.Post("/agents", s.handleCreateAgent)
		r.Post("/agents/connect-cli", s.handleConnectCLIAgent)
		r.Get("/agents/{id}", s.handleGetAgent)
		r.Put("/agents/{id}", s.handleUpdateAgent)
		r.Patch("/agents/{id}", s.handlePatchAgent)
		r.Delete("/agents/{id}", s.handleDeleteAgent)

		// Agent Connections in v1 (Sprint 9 S09.2)
		r.Get("/agent-connections", s.handleListAgentConnections)
		r.Post("/agent-connections", s.handleCreateAgentConnection)
		r.Get("/agent-connections/{id}", s.handleGetAgentConnection)
		r.Delete("/agent-connections/{id}", s.handleDeleteAgentConnection)
		r.Post("/agent-connections/{id}/verify", s.handleVerifyAgentConnection)

		// Backward-compatibility aliases for /agents/connections
		r.Get("/agents/connections", s.handleListAgentConnections)
		r.Post("/agents/connections", s.handleCreateAgentConnection)
		r.Get("/agents/connections/{id}", s.handleGetAgentConnection)
		r.Delete("/agents/connections/{id}", s.handleDeleteAgentConnection)
		r.Post("/agents/connections/{id}/verify", s.handleVerifyAgentConnection)

		// Creator Blueprint Engine in v1 (Sprint 10 S10.2)
		r.Get("/creator/templates", s.handleListCreatorTemplates)
		r.Get("/creator/templates/{id}", s.handleGetCreatorTemplate)
		r.Post("/creator/projects/instantiate", s.handleInstantiateCreatorProject)

		// AI Script Inspector & ZIP Uploader in v1 (Sprint 11 S11.2)
		r.Post("/projects/from-zip", s.handleCreateProjectFromZip)
		r.Post("/projects/inspect-zip", s.handleInspectZip)
		r.Post("/inspector/inspect-zip", s.handleInspectZip)

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
			pr.Patch("/auth/username", s.handleUsername)
			pr.Get("/integrations/github/oauth", s.handleGitHubOAuth)
			pr.Post("/integrations/github/oauth", s.handleGitHubOAuth)
			pr.Get("/integrations/github/device/{loginID}", s.handleGitHubDeviceLogin)
			pr.Post("/integrations/github/device/{loginID}/cancel", s.handleGitHubDeviceLogin)
			pr.Get("/integrations/github", s.handleGitHubConnection)
			pr.Put("/integrations/github", s.handleGitHubConnection)
			pr.Delete("/integrations/github", s.handleGitHubConnection)
			pr.Get("/integrations/github/repositories", s.handleGitHubRepositories)
			pr.Get("/profile", s.handleProfile)
			pr.Patch("/profile", s.handleProfile)
			pr.Get("/notifications", s.handleNotifications)
			pr.Patch("/notifications", s.handleNotifications)
			pr.Get("/notifications/preferences", s.handleNotificationPreferences)
			pr.Patch("/notifications/preferences", s.handleNotificationPreferences)
			pr.Get("/notifications/push", s.handlePushSubscription)
			pr.Post("/notifications/push", s.handlePushSubscription)
			pr.Delete("/notifications/push", s.handlePushSubscription)
			pr.Post("/auth/password", s.handleAuthPassword)

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
			pr.Post("/tasks/{id}/apply", s.handleApplyTask)
			pr.Get("/tasks/{id}/attempts/{attemptId}/logs", s.handleGetAttemptLogs)

			// Runners & Infrastructure (T5)
			pr.Get("/runners", s.handleListRunners)
			pr.Get("/runners/{id}", s.handleGetRunner)
			pr.Post("/runner/reconcile", s.handleReconcileLeases)

			// Projects endpoints (T3)
			pr.Get("/projects", s.handleListProjects)
			pr.Post("/projects", s.handleConnectProject)
			pr.Get("/projects/{id}", s.handleGetProject)
			pr.Patch("/projects/{id}/settings", s.handleProjectSettings)
			pr.Post("/projects/{id}/sync", s.handleSyncProject)
			pr.Delete("/projects/{id}", s.handleDeleteProject)
			pr.Get("/projects/{id}/branches", s.handleGetProjectBranches)
			pr.Patch("/projects/{id}/modules", s.handleUpdateProjectModules)
			pr.Put("/projects/{id}/modules", s.handleUpdateProjectModules)
			pr.Patch("/projects/{id}/archive", s.handleArchiveProject)
			pr.Post("/projects/{id}/archive", s.handleArchiveProject)
			pr.Patch("/projects/{id}/unarchive", s.handleUnarchiveProject)
			pr.Post("/projects/{id}/unarchive", s.handleUnarchiveProject)
			pr.Patch("/projects/{id}/restore", s.handleUnarchiveProject)
			pr.Post("/projects/{id}/restore", s.handleUnarchiveProject)

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

			// Conversational Inbox Engine & Thread Management (Sprint 17 S17.2 & S17.3)
			pr.Get("/inbox", s.handleListInboxThreads)
			pr.Post("/inbox", s.handleCreateInboxThread)
			pr.Get("/inbox/{id}", s.handleGetInboxThread)
			pr.Post("/inbox/{id}/messages", s.handleCreateInboxMessage)
			pr.Post("/inbox/{id}/approve", s.handleApproveInboxThread)

			// Direct Messenger Hub (Sprint 18)
			pr.Get("/extensions", s.handleExtensions)
			pr.Post("/extensions/{extensionID}/actions/{action}", s.handleExtensionAction)
			pr.Get("/search", s.handleUniversalSearch)
			pr.Get("/contacts/search", s.handleCRMSearch)
			pr.Get("/contacts/{id}/memory", s.handleContactMemory)
			pr.Post("/contacts/{id}/memory", s.handleContactMemory)
			pr.Patch("/contacts/{id}/memory/{memoryID}", s.handleContactMemoryChange)
			pr.Delete("/contacts/{id}/memory/{memoryID}", s.handleContactMemoryChange)
			pr.Get("/contacts", s.handleCRMList)
			pr.Post("/contacts", s.handleCRMCreate)
			pr.Patch("/contacts/{id}", s.handleCRMUpdate)
			pr.Get("/chat/contacts", s.handleInternalListContacts)
			pr.Get("/chat/canned-responses", s.handleListCannedResponses)
			pr.Post("/chat/canned-responses", s.handleCreateCannedResponse)
			pr.Get("/chat/canned-responses/{id}", s.handleGetCannedResponse)
			pr.Put("/chat/canned-responses/{id}", s.handleUpdateCannedResponse)
			pr.Patch("/chat/canned-responses/{id}", s.handleUpdateCannedResponse)
			pr.Delete("/chat/canned-responses/{id}", s.handleDeleteCannedResponse)
			pr.Post("/chat/conversations", s.handleCreateAgentTopic)
			pr.Post("/studio/goals", s.handleStudioGoal)
			pr.Post("/chat/attachments", s.handleUploadConversationAttachment)
			pr.Get("/chat/attachments/{id}", s.handleDownloadConversationAttachment)
			pr.Get("/chat/conversations/{id}/messages", s.handleInternalGetConversationMessages)
			pr.Post("/chat/conversations/{id}/messages", s.handleInternalSendConversationMessage)
			pr.Get("/chat/conversations/{id}/events", s.handleInboxStream)
			pr.Post("/chat/conversations/{id}/messages/{messageID}/task", s.handleInboxMessageTask)
			pr.Post("/chat/conversations/{id}/requests/{requestID}/cancel", s.handleCancelInboxRequest)
			pr.Post("/chat/conversations/{id}/requests/{requestID}/retry", s.handleRetryInboxRequest)
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
			if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/healthz") || strings.HasPrefix(path, "/readyz") || strings.HasPrefix(path, "/uploads") || strings.HasPrefix(path, "/chat") || path == "/install.sh" {
				http.NotFound(w, r)
				return
			}

			filePath := filepath.Join(staticDir, path)
			if f, err := os.Stat(filePath); err == nil && !f.IsDir() {
				if path == "/sw.js" {
					w.Header().Set("Cache-Control", "no-store")
					w.Header().Set("Service-Worker-Allowed", "/")
				}
				if path == "/manifest.webmanifest" {
					w.Header().Set("Content-Type", "application/manifest+json")
				}
				fileServer.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Cache-Control", "no-store")
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
		})
	} else {
		s.router.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			path := filepath.Clean(r.URL.Path)
			if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/healthz") || strings.HasPrefix(path, "/readyz") || strings.HasPrefix(path, "/uploads") || path == "/install.sh" {
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

func (s *Server) handleInstallScript(w http.ResponseWriter, r *http.Request) {
	candidates := []string{
		filepath.Join(".", "scripts", "install.sh"),
		filepath.Join("..", "scripts", "install.sh"),
		filepath.Join("..", "..", "scripts", "install.sh"),
		"/opt/cuan/scripts/install.sh",
	}
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil && len(data) > 0 {
			w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
			return
		}
	}
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("#!/usr/bin/env bash\necho 'Cuan installer script temporarily unavailable. Please visit https://github.com/buchin/cuan'\nexit 1\n"))
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

	identifier := strings.TrimSpace(req.Username)
	if identifier == "" {
		identifier = strings.TrimSpace(req.Email)
	}

	resp, err := s.authSvc.Login(r.Context(), identifier, req.Password)
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

	ownerPreferencesMu.Lock()
	preferences, _ := readOwnerPreferences(owner.ID)
	ownerPreferencesMu.Unlock()
	userData := map[string]interface{}{
		"display_name": preferences.DisplayName,
		"id":           owner.ID,
		"username":     owner.Username,
		"email":        owner.Email,
		"created_at":   owner.CreatedAt,
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"owner": userData,
		"user":  userData,
	})
}

func (s *Server) handleAuthPassword(w http.ResponseWriter, r *http.Request) {
	owner := auth.GetOwnerFromContext(r.Context())
	if owner == nil {
		respondJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req auth.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := s.authSvc.ChangePassword(r.Context(), owner.ID, req.CurrentPassword, req.NewPassword); err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			respondJSONError(w, http.StatusUnauthorized, "invalid current password")
			return
		}
		if errors.Is(err, auth.ErrInvalidInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update password: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "password updated successfully",
	})
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	var projectIDs []string
	if pids := r.URL.Query().Get("project_ids"); pids != "" {
		for _, id := range strings.Split(pids, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				projectIDs = append(projectIDs, id)
			}
		}
	}
	if qPids := r.URL.Query()["project_id"]; len(qPids) > 1 {
		for _, id := range qPids {
			id = strings.TrimSpace(id)
			if id != "" {
				projectIDs = append(projectIDs, id)
			}
		}
	}

	filter := task.TaskFilter{
		ProjectID:  r.URL.Query().Get("project_id"),
		ProjectIDs: projectIDs,
		Status:     r.URL.Query().Get("status"),
		Search:     r.URL.Query().Get("search"),
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

	if strings.TrimSpace(req.ProjectID) == "" && len(req.ProjectIDs) == 0 {
		if s.projectSvc == nil {
			respondJSONError(w, 503, "Projects are unavailable")
			return
		}
		p, err := s.projectSvc.EnsureDefault(r.Context())
		if err != nil {
			respondJSONError(w, 503, "Default project is unavailable")
			return
		}
		if p.IsArchived {
			respondJSONError(w, 409, "Default project is archived. Choose an active project.")
			return
		}
		req.ProjectID = "default"
		req.ProjectIDs = []string{"default"}
	}

	author := "owner"
	if owner := auth.GetOwnerFromContext(r.Context()); owner != nil && owner.Email != "" {
		author = owner.Email
	}

	created, err := s.taskSvc.CreateTask(r.Context(), author, req)
	if err != nil {
		if errors.Is(err, task.ErrRequestConflict) {
			respondJSONError(w, 409, err.Error())
			return
		}
		if errors.Is(err, task.ErrInvalidTaskInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create task: %v", err))
		return
	}

	if s.taskProjectRepo != nil && len(req.ProjectIDs) > 0 {
		_ = s.taskProjectRepo.SetTaskProjects(r.Context(), created.ID, req.ProjectIDs)
	}

	respondJSON(w, http.StatusCreated, created)
}

func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	detail, err := s.taskSvc.GetTaskDetail(r.Context(), id)
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			if s.projectStudioRepo != nil {
				if pt, ptErr := s.projectStudioRepo.GetTaskByID(r.Context(), id); ptErr == nil && pt != nil {
					respondJSON(w, http.StatusOK, map[string]interface{}{
						"task": pt.ToDTO(),
					})
					return
				}
			}
			respondJSONError(w, http.StatusNotFound, "Task not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get task: %v", err))
		return
	}

	if s.projectStudioRepo != nil && detail.SourceRoomID != "" && detail.LatestAttempt != nil {
		if m, err := s.projectStudioRepo.GetMessageByID(r.Context(), delegationMessageID("task-delivery:"+detail.LatestAttempt.ID)); err == nil && m != nil && m.RoomID == detail.SourceRoomID && m.AuthorType == "agent" && m.ReplyToID != nil && *m.ReplyToID == detail.SourceMessageID {
			_, ids := decodeAttachmentMessage(m.Content)
			if owner, err := s.taskArtifactOwner(r.Context(), detail); err == nil {
				for _, aid := range ids {
					if a, _, err := loadAttachment(owner, aid); err == nil {
						detail.Attachments = append(detail.Attachments, a)
					}
				}
			}
		}
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
		if errors.Is(err, task.ErrTaskActive) || errors.Is(err, task.ErrRequestConflict) {
			respondJSONError(w, 409, err.Error())
			return
		}
		if errors.Is(err, task.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "Task not found")
			return
		}
		if errors.Is(err, task.ErrInvalidTaskInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update task: %v", err))
		return
	}

	if s.taskProjectRepo != nil && req.ProjectIDs != nil {
		_ = s.taskProjectRepo.SetTaskProjects(r.Context(), id, *req.ProjectIDs)
	}

	respondJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	err := s.taskSvc.DeleteTask(r.Context(), id)
	if err != nil {
		if errors.Is(err, task.ErrTaskActive) {
			respondJSONError(w, 409, "Cancel the active session before deleting this task")
			return
		}
		if errors.Is(err, task.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "Task not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete task: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Task deleted",
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
			respondJSONError(w, http.StatusNotFound, "Task not found")
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
			respondJSONError(w, http.StatusNotFound, "Task not found")
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

	if _, err := s.taskSvc.AttemptForTask(r.Context(), chi.URLParam(r, "id"), attemptId); err != nil {
		respondJSONError(w, http.StatusNotFound, "Attempt not found for this task")
		return
	}
	author := "owner"
	if owner := auth.GetOwnerFromContext(r.Context()); owner != nil && owner.Email != "" {
		author = owner.Email
	}

	att, err := s.taskSvc.UpdateAttemptStatus(r.Context(), attemptId, author, req)
	if err != nil {
		if errors.Is(err, task.ErrAttemptNotFound) {
			respondJSONError(w, http.StatusNotFound, "Attempt not found")
			return
		}
		if errors.Is(err, task.ErrInvalidTaskInput) {
			respondJSONError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update attempt: %v", err))
		return
	}

	if att.Status != task.AttemptStatusRunning && att.Status != task.AttemptStatusQueued {
		if err := s.taskSvc.ReconcileLocalTask(r.Context(), att.TaskID, att.ID); err != nil {
			respondJSONError(w, http.StatusInternalServerError, "Execution status saved; task status could not be refreshed. Retry the same update.")
			return
		}
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

	archivedParam := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("archived")))
	includeArchivedParam := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("include_archived")))
	statusParam := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))

	var filtered []*project.ProjectResponse
	for _, p := range projects {
		if archivedParam == "true" || statusParam == "archived" {
			if p.IsArchived {
				filtered = append(filtered, p)
			}
		} else if archivedParam == "all" || includeArchivedParam == "true" || statusParam == "all" {
			filtered = append(filtered, p)
		} else {
			// default: only active (is_archived == false)
			if !p.IsArchived {
				filtered = append(filtered, p)
			}
		}
	}
	if filtered == nil {
		filtered = []*project.ProjectResponse{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"projects": filtered,
		"total":    len(filtered),
	})
}

func (s *Server) handleArchiveProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	res, err := s.projectSvc.Archive(r.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			respondJSONError(w, http.StatusNotFound, "Project not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to archive project: %v", err))
		return
	}

	if s.projectRepo != nil {
		_ = s.projectRepo.ArchiveProject(r.Context(), id)
	}

	respondJSON(w, http.StatusOK, res)
}

func (s *Server) handleUnarchiveProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	res, err := s.projectSvc.Unarchive(r.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			respondJSONError(w, http.StatusNotFound, "Project not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to unarchive project: %v", err))
		return
	}

	if s.projectRepo != nil {
		_ = s.projectRepo.UnarchiveProject(r.Context(), id)
	}

	respondJSON(w, http.StatusOK, res)
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
			respondJSONError(w, http.StatusBadRequest, "Invalid GitHub repository URL. Use 'https://github.com/owner/repo' or 'owner/repo'.")
			return
		}
		if errors.Is(err, git.ErrRepoNotFound) {
			respondJSONError(w, http.StatusNotFound, "Repository not found. Check its name and make sure the GitHub account connected in Settings > Integrations has access.")
			return
		}
		if errors.Is(err, git.ErrAccessDenied) {
			respondJSONError(w, http.StatusForbidden, "GitHub could not grant access to this repository. Reconnect GitHub in Settings > Integrations with an account that has access, then try again.")
			return
		}
		if errors.Is(err, project.ErrProjectAlreadyExists) {
			respondJSONError(w, http.StatusConflict, "This GitHub repository is already connected to a Cuan project.")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Could not connect repository: %v", err))
		return
	}

	respondJSON(w, http.StatusCreated, res)
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := s.projectSvc.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			respondJSONError(w, http.StatusNotFound, "Project not found")
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
			respondJSONError(w, http.StatusNotFound, "Project not found")
			return
		}
		respondJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   fmt.Sprintf("Sync failed: %v", err),
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
			respondJSONError(w, http.StatusNotFound, "Project not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to delete project: %v", err))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Project removed from Cuan. The remote GitHub repository is preserved.",
		"id":      id,
	})
}

func (s *Server) handleGetProjectBranches(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	branches, defaultBranch, err := s.projectSvc.GetBranches(r.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			respondJSONError(w, http.StatusNotFound, "Project not found")
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

func (s *Server) handleUpdateProjectModules(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		respondJSONError(w, http.StatusBadRequest, "project id is required")
		return
	}

	var payload struct {
		EnabledModules []string `json:"enabled_modules"`
		Modules        []string `json:"modules"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		respondJSONError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	mods := payload.EnabledModules
	if len(mods) == 0 && len(payload.Modules) > 0 {
		mods = payload.Modules
	}

	res, err := s.projectSvc.UpdateModules(r.Context(), id, mods)
	if err != nil {
		if errors.Is(err, project.ErrProjectNotFound) {
			respondJSONError(w, http.StatusNotFound, "Project not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to update project modules: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, res)
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
			respondJSONError(w, http.StatusNotFound, "Runner not found")
			return
		}
		respondJSONError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get runner: %v", err))
		return
	}

	respondJSON(w, http.StatusOK, rInfo)
}

func (s *Server) handleGetAttemptLogs(w http.ResponseWriter, r *http.Request) {
	attemptID := chi.URLParam(r, "attemptId")
	if _, err := s.taskSvc.AttemptForTask(r.Context(), chi.URLParam(r, "id"), attemptID); err != nil {
		respondJSONError(w, 404, "Attempt not found for this task")
		return
	}
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

	cancelledLaunches, launchErr := s.taskSvc.CancelLaunches(r.Context(), taskID, time.Now().UTC())
	if launchErr != nil {
		respondJSONError(w, 503, "Could not save cancellation. Check the launch status before retrying.")
		return
	}
	att, err := s.taskSvc.CancelTask(r.Context(), taskID, author)
	if cancelledLaunches > 0 && (errors.Is(err, task.ErrAttemptNotFound) || errors.Is(err, task.ErrInvalidTaskInput)) {
		blocked := task.TaskStatusBlocked
		_, _ = s.taskSvc.UpdateTask(r.Context(), taskID, author, task.UpdateTaskRequest{Status: &blocked})
		if detail, e := s.taskSvc.GetTaskDetail(r.Context(), taskID); e == nil {
			s.publishInbox(detail.SourceRoomID)
		}
		respondJSON(w, 200, map[string]string{"status": "cancelled"})
		return
	}
	if err != nil {
		if errors.Is(err, task.ErrTaskNotFound) {
			respondJSONError(w, http.StatusNotFound, "Task not found")
			return
		}
		if errors.Is(err, task.ErrAttemptNotFound) {
			respondJSONError(w, http.StatusNotFound, "No attempt is available to cancel")
			return
		}
		respondJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	if s.terminalManager != nil {
		_ = s.terminalManager.CloseSession(taskID)
	}
	if detail, e := s.taskSvc.GetTaskDetail(r.Context(), taskID); e == nil && detail.SourceRoomID != "" {
		s.publishInbox(detail.SourceRoomID)
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
			respondJSONError(w, http.StatusNotFound, "Task not found")
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
