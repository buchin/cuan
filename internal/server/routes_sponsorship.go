package server

import (
	"github.com/buchin/cuan/internal/auth"
	"github.com/go-chi/chi/v5"
)

// registerSponsorshipRoutes mounts public sponsorship intake, checkout, Dojo webhooks, and admin moderation endpoints on root router.
func (s *Server) registerSponsorshipRoutes(r chi.Router) {
	// Dojo Webhook Receiver (SSOT S02.1 spec: /api/webhooks/dojo)
	r.Post("/api/webhooks/dojo", s.handleDojoWebhook)

	// Public Sponsorship, Waiting List, and Logo Endpoints
	r.Post("/api/sponsors/upload-logo", s.handleUploadSponsorLogo)
	r.Post("/api/sponsors/intake", s.handleSponsorIntake)
	r.Post("/api/sponsors/checkout", s.handleSponsorCheckout)
	r.Get("/api/sponsors/current", s.handleListCurrentSponsors)
	r.Get("/api/sponsors/status", s.handleGetSponsorshipStatus)
	r.Get("/api/sponsors/batches", s.handleListBatches)
	r.Post("/api/leads", s.handleCreateLead)
	r.Post("/api/waiting-list", s.handleCreateLead)
	r.Get("/uploads/sponsors/logos/*", s.handleServeSponsorLogo)

	// Admin Sponsorship Moderation Endpoints (Single-Owner access only) on root router
	r.Group(func(ar chi.Router) {
		ar.Use(auth.Middleware(s.authSvc))
		ar.Get("/api/admin/sponsors", s.handleAdminListSponsors)
		ar.Post("/api/admin/sponsors/{id}/approve", s.handleAdminApproveSponsor)
		ar.Post("/api/admin/sponsors/{id}/reject", s.handleAdminRejectSponsor)
		ar.Post("/api/admin/sponsors/sync-readme", s.handleAdminSyncReadme)
		ar.Get("/api/admin/sponsors/{id}/activities", s.handleAdminListSponsorActivities)
		ar.Get("/api/admin/leads", s.handleAdminListLeads)
	})
	r.Route("/api/sponsors/admin", func(ar chi.Router) {
		ar.Use(auth.Middleware(s.authSvc))
		ar.Get("/sponsors", s.handleAdminListSponsors)
		ar.Post("/sponsors/{id}/approve", s.handleAdminApproveSponsor)
		ar.Post("/sponsors/{id}/reject", s.handleAdminRejectSponsor)
		ar.Post("/sponsors/sync-readme", s.handleAdminSyncReadme)
		ar.Get("/sponsors/{id}/activities", s.handleAdminListSponsorActivities)
		ar.Get("/leads", s.handleAdminListLeads)
	})
}

// registerSponsorshipV1Routes mounts public sponsorship routes inside /api/v1 router.
func (s *Server) registerSponsorshipV1Routes(r chi.Router) {
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
}

// registerSponsorshipProtectedRoutes mounts authenticated sponsorship admin endpoints inside /api/v1 protected router.
func (s *Server) registerSponsorshipProtectedRoutes(pr chi.Router) {
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
}
