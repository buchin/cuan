package server

import (
	"github.com/go-chi/chi/v5"
)

// registerCampaignRoutes mounts batch campaigns, creator blueprints, zip inspector, and room discussions on root router.
func (s *Server) registerCampaignRoutes(r chi.Router) {
	// Creator Blueprint Engine Endpoints (Sprint 10 S10.2)
	r.Get("/api/creator/templates", s.handleListCreatorTemplates)
	r.Get("/api/creator/templates/{id}", s.handleGetCreatorTemplate)
	r.Post("/api/creator/projects/instantiate", s.handleInstantiateCreatorProject)

	// AI Script Inspector & ZIP Uploader Endpoints (Sprint 11 S11.2)
	r.Post("/api/projects/from-zip", s.handleCreateProjectFromZip)
	r.Post("/api/projects/inspect-zip", s.handleInspectZip)
	r.Post("/api/inspector/inspect-zip", s.handleInspectZip)

	// Batch Campaigns Engine Endpoints (Sprint 8 S08.2 & S08.3)
	r.Post("/api/campaigns", s.handleCreateBatchCampaign)
	r.Get("/api/campaigns", s.handleListBatchCampaigns)
	r.Get("/api/campaigns/{id}", s.handleGetBatchCampaign)
	r.Get("/api/campaigns/{id}/rollup", s.handleGetCampaignRollup)
	r.Get("/api/campaigns/{id}/events", s.handleCampaignEventsSSE)
	r.Post("/api/campaigns/{id}/execute", s.handleExecuteBatchCampaign)

	// Project Studio Discussions, Messages, Tasks & Real-Time SSE (Sprint 7 S07.2)
	r.Post("/api/projects/{id}/rooms", s.handleCreateProjectRoom)
	r.Get("/api/projects/{id}/rooms", s.handleListProjectRooms)
	r.Get("/api/rooms/{id}", s.handleGetProjectRoom)
	r.Put("/api/rooms/{id}", s.handleUpdateProjectRoom)
	r.Delete("/api/rooms/{id}", s.handleDeleteProjectRoom)
	r.Get("/api/projects/rooms/{id}", s.handleGetProjectRoom)
	r.Put("/api/projects/rooms/{id}", s.handleUpdateProjectRoom)
	r.Delete("/api/projects/rooms/{id}", s.handleDeleteProjectRoom)

	r.Post("/api/rooms/{id}/messages", s.handleCreateProjectMessage)
	r.Get("/api/rooms/{id}/messages", s.handleListProjectMessages)
	r.Post("/api/projects/rooms/{id}/messages", s.handleCreateProjectMessage)
	r.Get("/api/projects/rooms/{id}/messages", s.handleListProjectMessages)
	r.Delete("/api/messages/{id}", s.handleDeleteProjectMessage)

	r.Get("/api/rooms/{id}/events", s.handleStreamRoomEvents)
	r.Get("/api/projects/rooms/{id}/events", s.handleStreamRoomEvents)

	r.Post("/api/rooms/{id}/distill", s.handleDistillTaskFromRoom)
	r.Post("/api/projects/rooms/{id}/distill", s.handleDistillTaskFromRoom)
}

// registerCampaignV1Routes mounts campaigns, blueprints, and room routes inside /api/v1 router.
func (s *Server) registerCampaignV1Routes(r chi.Router) {
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

	// Batch Campaigns in v1 (Sprint 8 S08.2 & S08.3)
	r.Post("/campaigns", s.handleCreateBatchCampaign)
	r.Get("/campaigns", s.handleListBatchCampaigns)
	r.Get("/campaigns/{id}", s.handleGetBatchCampaign)
	r.Get("/campaigns/{id}/rollup", s.handleGetCampaignRollup)
	r.Get("/campaigns/{id}/events", s.handleCampaignEventsSSE)
	r.Post("/campaigns/{id}/execute", s.handleExecuteBatchCampaign)

	// Creator Blueprint Engine in v1 (Sprint 10 S10.2)
	r.Get("/creator/templates", s.handleListCreatorTemplates)
	r.Get("/creator/templates/{id}", s.handleGetCreatorTemplate)
	r.Post("/creator/projects/instantiate", s.handleInstantiateCreatorProject)

	// AI Script Inspector & ZIP Uploader in v1 (Sprint 11 S11.2)
	r.Post("/projects/from-zip", s.handleCreateProjectFromZip)
	r.Post("/projects/inspect-zip", s.handleInspectZip)
	r.Post("/inspector/inspect-zip", s.handleInspectZip)
}
