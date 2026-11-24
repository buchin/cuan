package server

import (
	"github.com/go-chi/chi/v5"
)

// registerDeployerRoutes mounts BYOS servers, studio deployments, domains, and database provisioner endpoints on root router.
func (s *Server) registerDeployerRoutes(r chi.Router) {
	// Caddy On-Demand TLS Security Gatekeeper (Sprint 5 S05.2)
	r.Get("/api/caddy/ask", s.handleCaddyAsk)
	r.Get("/api/domains/check-ask", s.handleCaddyAsk)

	// Studio Deployments Engine Endpoints (Sprint 3)
	r.Get("/api/deployments", s.handleListDeployments)
	r.Post("/api/deployments/upload", s.handleUploadDeploymentZip)
	r.Post("/api/deployments/git", s.handleCreateDeploymentGit)
	r.Post("/api/deployments", s.handleCreateDeployment)
	r.Get("/api/deployments/{id}", s.handleGetDeployment)
	r.Post("/api/deployments/{id}/start", s.handleStartDeployment)
	r.Post("/api/deployments/{id}/stop", s.handleStopDeployment)
	r.Post("/api/deployments/{id}/restart", s.handleRestartDeployment)
	r.Delete("/api/deployments/{id}", s.handleDeleteDeployment)
	r.Get("/api/deployments/{id}/logs", s.handleGetDeploymentLogs)
	r.Get("/api/deployments/{id}/health", s.handleGetDeploymentHealth)
	r.Post("/api/deployments/{id}/health/check-now", s.handleCheckDeploymentHealthNow)

	// BYOS Server Management Endpoints (Sprint 4 S04.2 & Sprint 6 S06.2)
	r.Get("/api/servers", s.handleListServers)
	r.Post("/api/servers", s.handleCreateServer)
	r.Get("/api/servers/{id}", s.handleGetServer)
	r.Get("/api/servers/{id}/metrics", s.handleGetServerMetrics)
	r.Post("/api/servers/{id}/test", s.handleTestServer)
	r.Delete("/api/servers/{id}", s.handleDeleteServer)

	// Custom Domain & DNS Verification Endpoints (Sprint 5 S05.3)
	r.Post("/api/deployments/{id}/domains", s.handleCreateDeploymentDomain)
	r.Get("/api/deployments/{id}/domains", s.handleListDeploymentDomains)
	r.Get("/api/domains", s.handleListDomains)
	r.Get("/api/domains/{id}", s.handleGetDomain)
	r.Post("/api/domains/{id}/verify", s.handleVerifyDomain)
	r.Delete("/api/domains/{id}", s.handleDeleteDomain)

	// Incident Alert Dispatcher & Multi-Channel Notification Engine Endpoints (Sprint 6 S06.3)
	r.Post("/api/projects/{id}/alert-channels", s.handleCreateAlertChannel)
	r.Get("/api/projects/{id}/alert-channels", s.handleListAlertChannels)
	r.Post("/api/alert-channels/{id}/test", s.handleTestAlertChannel)
	r.Delete("/api/alert-channels/{id}", s.handleDeleteAlertChannel)
	r.Get("/api/incidents", s.handleListIncidents)

	// Automated Database Provisioner & Environment Secrets Endpoints (Sprint 12 S12.2)
	r.Post("/api/projects/{id}/provision-db", s.handleProvisionProjectDatabase)
	r.Post("/api/projects/{id}/generate-env", s.handleGenerateProjectEnv)
	r.Get("/api/projects/{id}/env", s.handleGetProjectEnv)

	// Script Build Runner & Process Supervisor Endpoints (Sprint 12 S12.3)
	r.Post("/api/projects/{id}/script-deploy", s.handleScriptDeploy)
	r.Get("/api/projects/{id}/script-deploy/logs", s.handleScriptDeployLogs)
	r.Post("/api/projects/{id}/script-deploy/restart", s.handleScriptDeployRestart)
	r.Get("/api/projects/{id}/script-deploy/status", s.handleScriptDeployStatus)
	r.Get("/api/projects/{id}/deployments", s.handleGetProjectDeployments)
	r.Post("/api/projects/{id}/deployments", s.handleTriggerProjectDeployment)
	r.Get("/api/projects/{id}/deployments/{dep_id}", s.handleGetProjectDeploymentDetail)
	r.Post("/api/projects/{id}/deployments/{dep_id}/cancel", s.handleCancelProjectDeployment)
	r.Post("/api/projects/{id}/webhooks/deploy", s.handleProjectDeployWebhook)
}

// registerDeployerV1Routes mounts deployer, BYOS, and domain endpoints inside /api/v1 router.
func (s *Server) registerDeployerV1Routes(r chi.Router) {
	r.Get("/caddy/ask", s.handleCaddyAsk)

	// Observability Alert Channels & Incidents in v1
	r.Post("/projects/{id}/alert-channels", s.handleCreateAlertChannel)
	r.Get("/projects/{id}/alert-channels", s.handleListAlertChannels)
	r.Post("/alert-channels/{id}/test", s.handleTestAlertChannel)
	r.Delete("/alert-channels/{id}", s.handleDeleteAlertChannel)
	r.Get("/incidents", s.handleListIncidents)

	// Automated Database Provisioner & Environment Secrets in v1 (Sprint 12 S12.2)
	r.Post("/projects/{id}/provision-db", s.handleProvisionProjectDatabase)
	r.Post("/projects/{id}/generate-env", s.handleGenerateProjectEnv)
	r.Get("/projects/{id}/env", s.handleGetProjectEnv)

	// Script Build Runner & Process Supervisor in v1 (Sprint 12 S12.3)
	r.Post("/projects/{id}/script-deploy", s.handleScriptDeploy)
	r.Get("/projects/{id}/script-deploy/logs", s.handleScriptDeployLogs)
	r.Post("/projects/{id}/script-deploy/restart", s.handleScriptDeployRestart)
	r.Get("/projects/{id}/script-deploy/status", s.handleScriptDeployStatus)
	r.Get("/projects/{id}/deployments", s.handleGetProjectDeployments)
	r.Post("/projects/{id}/deployments", s.handleTriggerProjectDeployment)
	r.Get("/projects/{id}/deployments/{dep_id}", s.handleGetProjectDeploymentDetail)
	r.Post("/projects/{id}/deployments/{dep_id}/cancel", s.handleCancelProjectDeployment)
	r.Post("/projects/{id}/webhooks/deploy", s.handleProjectDeployWebhook)

	// Studio Deployments route group in /api/v1
	r.Route("/deployments", func(dr chi.Router) {
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
	r.Route("/servers", func(sr chi.Router) {
		sr.Get("/", s.handleListServers)
		sr.Post("/", s.handleCreateServer)
		sr.Get("/{id}", s.handleGetServer)
		sr.Get("/{id}/metrics", s.handleGetServerMetrics)
		sr.Post("/{id}/test", s.handleTestServer)
		sr.Delete("/{id}", s.handleDeleteServer)
	})

	// Custom Domain & DNS Verification route group in /api/v1
	r.Route("/domains", func(dr chi.Router) {
		dr.Get("/", s.handleListDomains)
		dr.Get("/{id}", s.handleGetDomain)
		dr.Post("/{id}/verify", s.handleVerifyDomain)
		dr.Delete("/{id}", s.handleDeleteDomain)
	})
}
