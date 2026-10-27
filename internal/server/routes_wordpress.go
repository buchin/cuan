package server

import (
	"github.com/go-chi/chi/v5"
)

// registerWordPressRoutes mounts all WordPress cloud deployments, assets, scripts, and webmaster endpoints on root router.
func (s *Server) registerWordPressRoutes(r chi.Router) {
	// WordPress CMS Publishing Engine & Multi-Format Exporter Endpoints (Sprint 11 S11.3)
	r.Post("/api/projects/{id}/integrations/wordpress", s.handleSetupWordPressIntegration)
	r.Get("/api/projects/{id}/integrations/wordpress", s.handleGetWordPressIntegration)
	r.Post("/api/tasks/{id}/publish", s.handlePublishTask)
	r.Post("/api/projects/tasks/{id}/publish", s.handlePublishTask)
	r.Get("/api/tasks/{id}/export", s.handleExportTask)
	r.Get("/api/projects/tasks/{id}/export", s.handleExportTask)

	// Turnkey WordPress Cloud & Orchestrator Endpoints (Sprint 13 S13.2)
	r.Post("/api/projects/{id}/wordpress/deploy", s.handleWordPressDeploy)
	r.Get("/api/projects/{id}/wordpress", s.handleGetWordPressDeployment)
	r.Get("/api/projects/{id}/wordpress/credentials", s.handleGetWordPressCredentials)
	r.Post("/api/projects/{id}/wordpress/restart", s.handleRestartWordPressDeployment)
	r.Get("/api/projects/{id}/wordpress/health", s.handleHealthCheckWordPressDeployment)
	r.Post("/api/projects/{id}/wordpress/stop", s.handleStopWordPressDeployment)
	r.Delete("/api/projects/{id}/wordpress", s.handleDeleteWordPressDeployment)

	// AI Webmaster Engine Endpoints (Sprint 13 S13.3)
	// Assets (Themes & Plugins)
	r.Post("/api/projects/{id}/wordpress/assets/upload", s.handleUploadWordPressAsset)
	r.Get("/api/projects/{id}/wordpress/assets", s.handleListWordPressAssets)
	r.Post("/api/projects/{id}/wordpress/assets/{asset_id}/status", s.handleToggleWordPressAssetStatus)
	r.Delete("/api/projects/{id}/wordpress/assets/{asset_id}", s.handleDeleteWordPressAsset)

	// Safe Script Injector (mu-plugins)
	r.Post("/api/projects/{id}/wordpress/scripts", s.handleInjectWordPressScript)
	r.Get("/api/projects/{id}/wordpress/scripts", s.handleListWordPressScripts)
	r.Post("/api/projects/{id}/wordpress/scripts/{script_id}/toggle", s.handleToggleWordPressScript)
	r.Delete("/api/projects/{id}/wordpress/scripts/{script_id}", s.handleDeleteWordPressScript)

	// Config Tweaker (wp-config.php)
	r.Post("/api/projects/{id}/wordpress/config", s.handleTweakWordPressConfig)
	r.Get("/api/projects/{id}/wordpress/config", s.handleGetWordPressConfig)

	// Webmaster Activity History
	r.Get("/api/projects/{id}/wordpress/actions", s.handleListWordPressWebmasterActions)
}

// registerWordPressV1Routes mounts WordPress endpoints inside /api/v1 router.
func (s *Server) registerWordPressV1Routes(r chi.Router) {
	// WordPress CMS Integration & Task Publishing / Export in v1 (Sprint 11 S11.3)
	r.Post("/projects/{id}/integrations/wordpress", s.handleSetupWordPressIntegration)
	r.Get("/projects/{id}/integrations/wordpress", s.handleGetWordPressIntegration)

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
}
