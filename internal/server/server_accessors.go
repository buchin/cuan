package server

import (
	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/auth"
	"github.com/buchin/cuan/internal/campaign"
	"github.com/buchin/cuan/internal/creator"
	"github.com/buchin/cuan/internal/crypto"
	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
	"github.com/buchin/cuan/internal/domain"
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

func (s *Server) CatalogCache() *agent.ModelCatalogCache {
	if s.catalogCache == nil {
		s.catalogCache = agent.DefaultCatalogCache()
	}
	return s.catalogCache
}

func (s *Server) SetCatalogCache(c *agent.ModelCatalogCache) {
	s.catalogCache = c
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
			Manager:      m,
			AttachOnly:   true,
			AuthService:  s.authSvc,
			TaskResolver: s.resolveTaskID,
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
