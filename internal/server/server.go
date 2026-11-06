package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	extensionSDK "github.com/buchin/cuan/pkg/extensions"
	"log"
	"net/http"
	"path/filepath"
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
	"github.com/buchin/cuan/internal/mcp"
	"github.com/buchin/cuan/internal/observability"
	"github.com/buchin/cuan/internal/project"
	"github.com/buchin/cuan/internal/publication"
	"github.com/buchin/cuan/internal/publisher"
	"github.com/buchin/cuan/internal/resend"
	"github.com/buchin/cuan/internal/runner"
	"github.com/buchin/cuan/internal/scheduler"
	"github.com/buchin/cuan/internal/sponsorship"
	"github.com/buchin/cuan/internal/studio"
	"github.com/buchin/cuan/internal/task"
	"github.com/buchin/cuan/internal/terminal"
	"github.com/buchin/cuan/internal/updater"
	"github.com/buchin/cuan/internal/wordpress"
)

type Server struct {
	publicationVerifier    func(context.Context, publication.Config, string) error
	publicationObserver    func(context.Context, publication.Config) (string, error)
	releaseObservations    sync.Map
	scopeAuditNext         atomic.Int64
	extensions             *extensionSDK.Registry
	crm                    *crmStore
	canned                 *cannedStore
	taste                  *tasteStore
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
	catalogCache           *agent.ModelCatalogCache
	aiProviderStore        *agent.AIProviderStore
	binaryManager          *agent.BinaryManager
	distillerEngine        *studio.DistillerEngine
	agentExecutionHook     studio.AgentExecutionHook
	campaignDecompiler     *campaign.DecompilerEngine
	campaignRunner         *campaign.MultiProjectRunner
	campaignAggregator     *campaign.RollupAggregator
	inboxSlots             chan struct{}
	inboxStopping          atomic.Bool
	inboxAdmission         sync.RWMutex
	inboxWorkers           sync.Map
	taskLaunches           sync.Map
	taskAgentGrants        sync.Map
	taskAgentURL           string
	inboxCancels           sync.Map
	messageGrounding       sync.Map
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
	mcpRegistry            *mcp.Registry
	scheduler              *scheduler.Scheduler
	taskManager            *scheduler.TaskManager
	updater                *updater.Service
	resendClient           resend.Client
	router                 *chi.Mux
}

func NewServer(cfg *config.Config, db *sql.DB, authSvc ...*auth.Service) *Server {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "SAMEORIGIN")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			next.ServeHTTP(w, r)
		})
	})

	var svc *auth.Service
	if len(authSvc) > 0 && authSvc[0] != nil {
		svc = authSvc[0]
	} else if db != nil {
		svc = auth.NewService(auth.NewDBStore(db))
	} else {
		svc = auth.NewService(auth.NewMemoryStore())
	}

	var pSvc *project.Service
	projectBaseDir := cfg.ProjectsDir
	if projectBaseDir == "" {
		projectBaseDir = filepath.Join("data", "projects")
	}
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
		CanonicalHosts: []string{baseHost, "cuan.work", "cuan.app", "cuan.dojo.cc"},
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
	if agentRepo != nil {
		_ = cuanDB.EnsureBuiltinAgents(context.Background(), agentRepo)
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
		IdleTimeout:           30 * time.Minute,
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

	sched := scheduler.New()
	updateSvc := updater.NewService(updater.Config{
		CurrentVersion: cfg.AppVersion,
		CurrentCommit:  cfg.CommitSHA,
		StoragePath:    filepath.Join("data", "system_update.json"),
	})

	_ = sched.Register(&scheduler.Job{
		ID:           "system_update_check",
		Name:         "Pemeriksaan Pembaruan Aplikasi",
		Description:  "Memeriksa rilis terbaru Cuan secara otomatis dari remote repository",
		ScheduleType: scheduler.ScheduleDaily,
		TimeOfDay:    updateSvc.GetInfo().AutoUpdateTime,
		Enabled:      updateSvc.GetInfo().AutoUpdateEnabled,
		Handler: func(ctx context.Context) error {
			_, err := updateSvc.CheckForUpdate(ctx)
			return err
		},
	})
	if cfg.InboxExecutionRole != "worker" {
		sched.Start(context.Background())
	}

	taskMgr := scheduler.NewTaskManager(filepath.Join("data", "scheduled_tasks.json"), func(ctx context.Context, task *scheduler.ScheduledTask) error {
		return nil
	})

	s := &Server{
		extensions:             extensionSDK.New(),
		crm:                    &crmStore{contacts: make(map[string]CRMContact)},
		canned:                 newCannedStore(),
		taste:                  newTasteStore(),
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
		mcpRegistry:            mcp.NewRegistry(),
		scheduler:              sched,
		taskManager:            taskMgr,
		updater:                updateSvc,
		router:                 r,
	}

	_ = sched.Register(&scheduler.Job{
		ID:           "cleanup_stale_anonymous_contacts",
		Name:         "Pembersihan Kontak Anonim Usang",
		Description:  "Menghapus sesi kontak pengunjung anonim tanpa pesan yang berusia lebih dari 24 jam",
		ScheduleType: scheduler.ScheduleDaily,
		TimeOfDay:    "04:00",
		Enabled:      true,
		Handler: func(ctx context.Context) error {
			_, err := s.PruneStaleAnonymousContacts(ctx, 24*time.Hour)
			return err
		},
	})

	_ = sched.Register(&scheduler.Job{
		ID:           "chat_idle_email_digest",
		Name:         "Notifikasi Email Chat Idle",
		Description:  "Memeriksa percakapan Live Chat yang belum terbaca dan mengirim transkrip via Resend",
		ScheduleType: scheduler.ScheduleInterval,
		Interval:     1 * time.Minute,
		Enabled:      true,
		Handler: func(ctx context.Context) error {
			_, err := s.CheckAndNotifyIdleChats(ctx)
			return err
		},
	})

	_ = sched.Register(&scheduler.Job{
		ID:           "gc_task_workspaces",
		Name:         "GC: Task Workspace Cleanup",
		Description:  "Menghapus git worktrees task yang sudah selesai dan tidak aktif (>48 jam, max 10 workspace)",
		ScheduleType: scheduler.ScheduleInterval,
		Interval:     6 * time.Hour,
		Enabled:      true,
		Handler: func(ctx context.Context) error {
			s.runTaskWorkspaceGC()
			return nil
		},
	})

	_ = sched.Register(&scheduler.Job{
		ID:           "gc_old_releases",
		Name:         "GC: Old Release Cleanup",
		Description:  "Menghapus release binary lama di /opt/cuan/releases, hanya mempertahankan 3 release terbaru",
		ScheduleType: scheduler.ScheduleInterval,
		Interval:     6 * time.Hour,
		Enabled:      true,
		Handler: func(ctx context.Context) error {
			s.runReleaseGC()
			return nil
		},
	})

	_ = sched.Register(&scheduler.Job{
		ID:           "gc_oversized_logs",
		Name:         "GC: Oversized Log Truncation",
		Description:  "Memotong file log nginx/postgresql yang melebihi 200 MB untuk mencegah disk penuh",
		ScheduleType: scheduler.ScheduleInterval,
		Interval:     6 * time.Hour,
		Enabled:      true,
		Handler: func(ctx context.Context) error {
			s.runLogGC()
			return nil
		},
	})

	termHandler.SetTaskResolver(s.resolveTaskID)
	tSvc.SetProjectFinder(liveProjectFinder{server: s})
	if s.runnerSvc != nil {
		s.runnerSvc.SetChatNotifier(runner.NewDefaultBusinessChatNotifier(func(ctx context.Context, roomID, messageContent string) error {
			if s.projectStudioRepo == nil || roomID == "" {
				return nil
			}
			// Single-bubble idempotency: avoid inserting duplicate message if already present in room
			if msgs, err := s.projectStudioRepo.ListMessagesByRoomID(ctx, roomID); err == nil && len(msgs) > 0 {
				for i := len(msgs) - 1; i >= 0 && i >= len(msgs)-5; i-- {
					if msgs[i].Content == messageContent {
						return nil
					}
				}
			}
			msg := &cuanDB.ProjectMessage{
				ID:         cuanDB.GenerateUUID(),
				RoomID:     roomID,
				AuthorType: "agent",
				AuthorID:   "cuan_assistant",
				AuthorName: "Cuan",
				Content:    messageContent,
				CreatedAt:  time.Now().UTC(),
				UpdatedAt:  time.Now().UTC(),
			}
			err := s.projectStudioRepo.CreateMessage(ctx, msg)
			if err == nil {
				s.publishInbox(roomID)
			}
			return err
		}))
	}
	s.InitScrumEngine(context.Background())
	s.configureSharedGitHubAccess()
	s.routes()
	s.setupStaticRoutes()
	s.SetExtensionRegistry(s.extensions)
	if cfg.InboxExecutionRole != "worker" {
		s.reconcileStartupState(context.Background())
	}
	return s
}

func (s *Server) reconcileStartupState(ctx context.Context) {
	now := time.Now().UTC()
	if s.taskSvc != nil {
		if err := s.taskSvc.ReconcileStartupLaunches(ctx, now); err != nil {
			log.Printf("[Startup] ReconcileStartupLaunches error: %v", err)
		}
		if err := s.taskSvc.RecoverStartupLocalAttempts(ctx); err != nil {
			log.Printf("[Startup] RecoverStartupLocalAttempts error: %v", err)
		}
		if err := s.taskSvc.ReconcileOrphanLaunches(ctx, now); err != nil {
			log.Printf("[Startup] ReconcileOrphanLaunches error: %v", err)
		}
	}
	if s.runnerSvc != nil {
		if _, err := s.runnerSvc.ReconcileExpiredLeases(ctx); err != nil {
			log.Printf("[Startup] ReconcileExpiredLeases error: %v", err)
		}
	}
	if err := s.ReconcileZombies(ctx); err != nil {
		log.Printf("[Startup] ReconcileZombies error: %v", err)
	}
	s.deliverInboxTaskUpdates(ctx)
	s.kickTaskLaunches()
}

// PrepareGracefulShutdown broadcasts restart notifications to active terminal sessions
// and flushes/closes active session runners cleanly before server exit.
func (s *Server) PrepareGracefulShutdown(_ time.Duration) {
	if s.terminalManager == nil {
		return
	}
	sessions := s.terminalManager.ActiveSessions()
	if len(sessions) == 0 {
		return
	}
	notice := []byte("\r\n\x1b[1;33m[⚡ Pembaruan sistem sedang diterapkan... Sesi akan otomatis tersambung kembali setelah restart]\x1b[0m\r\n")
	s.terminalManager.BroadcastNotice(notice)
	time.Sleep(200 * time.Millisecond)
	_ = s.terminalManager.CloseAll()
}

func (s *Server) Router() http.Handler {
	return s.router
}

func (s *Server) routes() {
	s.router.Get("/api/v1/task-agent/{id}", s.handleTaskAgent)
	s.router.Post("/api/v1/task-agent/{id}", s.handleTaskAgent)
	s.router.Post("/api/v1/task-agent/{id}/push", s.handleTaskAgent)
	s.router.Post("/api/v1/task-agent/{id}/publish", s.handleTaskAgent)

	// Domain route registrations (root router)
	s.registerSystemRoutes(s.router)
	s.registerTasksRootRoutes(s.router)
	s.registerDeployerRoutes(s.router)
	s.registerWordPressRoutes(s.router)
	s.registerCampaignRoutes(s.router)
	s.registerSponsorshipRoutes(s.router)
	s.registerAgentRoutes(s.router)
	s.registerVisitorChatRoutes(s.router)
	s.registerTeamRoutes(s.router)

	s.router.Get("/api/v1/item-preferences", s.handleItemPreferences)
	s.router.Patch("/api/v1/item-preferences", s.handleItemPreferences)
	s.router.Get("/api/inbox/conversations", s.handleInternalListContacts)
	s.router.Get("/api/conversations", s.handleInternalListContacts)

	s.router.Route("/api/v1", func(r chi.Router) {
		// Domain endpoints in v1
		s.registerSystemV1Routes(r)
		s.registerAuthV1Routes(r)
		s.registerTasksV1Routes(r)
		s.registerDeployerV1Routes(r)
		s.registerWordPressV1Routes(r)
		s.registerCampaignV1Routes(r)
		s.registerSponsorshipV1Routes(r)
		s.registerAgentV1Routes(r)
		s.registerTeamV1Routes(r)

		// Public aliases
		r.Get("/task-agent/{id}", s.handleTaskAgent)
		r.Post("/task-agent/{id}", s.handleTaskAgent)
		r.Post("/task-agent/{id}/push", s.handleTaskAgent)
		r.Post("/task-agent/{id}/publish", s.handleTaskAgent)
		r.Get("/item-preferences", s.handleItemPreferences)
		r.Patch("/item-preferences", s.handleItemPreferences)

		// Public endpoints
		r.Get("/integrations/github/oauth/callback", s.handleGitHubOAuthCallback)
		r.Get("/auth-status", s.handleGetHarnessAuthStatus)
		r.Post("/auth-init", s.handleInitHarnessAuth)
		r.Post("/auth-code", s.handleSubmitHarnessAuthCode)

		// Runner outbound endpoints (T5 - Authenticated via X-Runner-Token or Owner Token)
		r.Group(func(rr chi.Router) {
			rr.Use(s.runnerAuthMiddleware)
			rr.Post("/runner/heartbeat", s.handleRunnerHeartbeat)
			rr.Post("/runner/claim", s.handleRunnerClaim)
			rr.Post("/runner/attempts/{id}/visual-verification", s.handleRunnerVisualVerification)
			rr.Get("/runner/attempts/{id}/logs", s.handleRunnerGetStreamLogs)
			rr.Get("/runner/attempts/{id}/report", s.handleRunnerReadAttemptReport)
			rr.Post("/runner/attempts/{id}/publication/verify", s.handleRunnerVerifyPublication)
			rr.Post("/runner/attempts/{id}/logs", s.handleRunnerStreamLogs)
			rr.Post("/runner/attempts/{id}/steer", s.handleRunnerSteer)
			rr.Post("/runner/attempts/{id}/complete", s.handleRunnerComplete)
		})

		// Protected endpoints (Single-Owner access only)
		r.Group(func(pr chi.Router) {
			pr.Use(auth.Middleware(s.authSvc))

			pr.Get("/auth/me", s.handleAuthMe)
			pr.Patch("/auth/username", s.handleUsername)
			pr.Patch("/auth/email", s.handleEmail)
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

			// Resend Email Integration & Notification Settings (Chat Idle)
			pr.Get("/settings/resend", s.handleGetResendSettings)
			pr.Post("/settings/resend", s.handleSaveResendSettings)
			pr.Patch("/settings/resend", s.handleSaveResendSettings)
			pr.Post("/settings/resend/validate", s.handleValidateResendKey)
			pr.Post("/settings/resend/test", s.handleTestResendEmail)
			pr.Post("/chat/idle/check", s.handleTriggerChatIdleCheck)

			// Tasks endpoints (T4 & T5)
			s.registerTaskRoutes(pr)

			// Projects endpoints (T3)
			s.registerProjectRoutes(pr)

			// Sponsorship moderation endpoints (S02.4)
			s.registerSponsorshipProtectedRoutes(pr)

			// Conversational Inbox & Direct Messenger Hub (Sprint 17 & 18)
			s.registerInboxRoutes(pr)

			// Taste & Agent Memory Management (Sprint 43 S43-T2)
			s.registerTasteRoutes(pr)
		})
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
