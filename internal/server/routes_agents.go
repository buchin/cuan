package server

import (
	"github.com/go-chi/chi/v5"
)

// registerAgentRoutes mounts agent personas, sessions, provider connections, and pairings on root router.
func (s *Server) registerAgentRoutes(r chi.Router) {
	// Agent CLI Account Onboarding & Host Detection (Sprint 22 S22.1)
	r.Get("/api/agents/binaries", s.handleGetAgentBinaries)
	r.Post("/api/agents/pair", s.handlePairAgentDevice)
	r.Get("/api/agents/sessions", s.handleListAgentSessions)
	r.Delete("/api/agents/sessions/{id}", s.handleRevokeAgentSession)

	// Agent Personas & Connection Engine Endpoints (Sprint 9 S09.2)
	r.Get("/api/agents", s.handleListAgents)
	r.Post("/api/agents", s.handleCreateAgent)
	r.Get("/api/agents/auth-status", s.handleGetHarnessAuthStatus)
	r.Post("/api/agents/auth-init", s.handleInitHarnessAuth)
	r.Post("/api/agents/auth-code", s.handleSubmitHarnessAuthCode)
	r.Get("/api/agents/commandcode/callback", s.handleCommandCodeAuthCallback)
	r.Get("/api/agents/models", s.handleListProviderModels)
	r.Get("/api/auth-status", s.handleGetHarnessAuthStatus)
	r.Post("/api/auth-init", s.handleInitHarnessAuth)
	r.Post("/api/auth-code", s.handleSubmitHarnessAuthCode)
	r.Post("/auth-init", s.handleInitHarnessAuth)
	r.Post("/auth-code", s.handleSubmitHarnessAuthCode)
	r.Post("/api/agents/connect-cli", s.handleConnectCLIAgent)
	r.Get("/api/agents/{id}", s.handleGetAgent)
	r.Put("/api/agents/{id}", s.handleUpdateAgent)
	r.Patch("/api/agents/{id}", s.handlePatchAgent)
	r.Post("/api/agents/{id}/connection/credentials", s.handleReplaceAgentCredentials)
	r.Delete("/api/agents/{id}", s.handleDeleteAgent)

	r.Get("/api/agent-connections", s.handleListAgentConnections)
	r.Post("/api/agent-connections", s.handleCreateAgentConnection)
	r.Get("/api/agent-connections/{id}", s.handleGetAgentConnection)
	r.Delete("/api/agent-connections/{id}", s.handleDeleteAgentConnection)
	r.Post("/api/agent-connections/{id}/verify", s.handleVerifyAgentConnection)

	// Backward-compatibility aliases for /api/agents/connections
	r.Get("/api/agents/connections", s.handleListAgentConnections)
	r.Post("/api/agents/connections", s.handleCreateAgentConnection)
	r.Get("/api/agents/connections/{id}", s.handleGetAgentConnection)
	r.Delete("/api/agents/connections/{id}", s.handleDeleteAgentConnection)
	r.Post("/api/agents/connections/{id}/verify", s.handleVerifyAgentConnection)
}

// registerAgentV1Routes mounts agent routes inside /api/v1 router.
func (s *Server) registerAgentV1Routes(r chi.Router) {
	r.Route("/provider-accounts", func(ar chi.Router) {
		ar.Get("/", s.handleListProviderAccounts)
		ar.Post("/", s.handleCreateProviderAccount)
		ar.Post("/{accountID}/auth-init", s.handleProviderAccountAuthInit)
		ar.Post("/{accountID}/auth-code", s.handleProviderAccountAuthCode)
		ar.Post("/{accountID}/verify", s.handleVerifyProviderAccount)
		ar.Patch("/{accountID}/failover", s.handleProviderAccountFailover)
		ar.Patch("/{accountID}/model-fallback", s.handleProviderAccountFailover)
		ar.Get("/{accountID}/usage", s.handleProviderAccountUsage)
	})

	r.Route("/agents", func(ar chi.Router) {
		ar.Get("/", s.handleListAgents)
		ar.Post("/", s.handleCreateAgent)
		ar.Get("/auth-status", s.handleGetHarnessAuthStatus)
		ar.Post("/auth-init", s.handleInitHarnessAuth)
		ar.Post("/auth-code", s.handleSubmitHarnessAuthCode)
		ar.Get("/commandcode/callback", s.handleCommandCodeAuthCallback)
		ar.Get("/models", s.handleListProviderModels)
		ar.Post("/connect-cli", s.handleConnectCLIAgent)
		ar.Post("/{id}/connection/verify", s.handleVerifyAgentProfile)
		ar.Get("/{id}", s.handleGetAgent)
		ar.Put("/{id}", s.handleUpdateAgent)
		ar.Patch("/{id}", s.handlePatchAgent)
		ar.Post("/{id}/connection/credentials", s.handleReplaceAgentCredentials)
		ar.Delete("/{id}", s.handleDeleteAgent)

		// Nested /api/v1/agents/connections
		ar.Get("/connections", s.handleListAgentConnections)
		ar.Post("/connections", s.handleCreateAgentConnection)
		ar.Get("/connections/{id}", s.handleGetAgentConnection)
		ar.Delete("/connections/{id}", s.handleDeleteAgentConnection)
		ar.Post("/connections/{id}/verify", s.handleVerifyAgentConnection)

		ar.Get("/binaries", s.handleGetAgentBinaries)
		ar.Post("/pair", s.handlePairAgentDevice)
		ar.Get("/sessions", s.handleListAgentSessions)
		ar.Delete("/sessions/{id}", s.handleRevokeAgentSession)
	})

	r.Route("/agent-connections", func(cr chi.Router) {
		cr.Get("/", s.handleListAgentConnections)
		cr.Post("/", s.handleCreateAgentConnection)
		cr.Get("/{id}", s.handleGetAgentConnection)
		cr.Delete("/{id}", s.handleDeleteAgentConnection)
		cr.Post("/{id}/verify", s.handleVerifyAgentConnection)
	})
}
