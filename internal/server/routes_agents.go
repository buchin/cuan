package server

import (
	"github.com/go-chi/chi/v5"
)

// registerAgentRoutes mounts agent personas, sessions, provider connections, and pairings.
func (s *Server) registerAgentRoutes(r chi.Router) {
	// Agent CLI Account Onboarding & Host Detection (Sprint 22 S22.1)
	r.Get("/api/agents/binaries", s.handleGetAgentBinaries)
	r.Get("/api/v1/agents/binaries", s.handleGetAgentBinaries)
	r.Post("/api/agents/pair", s.handlePairAgentDevice)
	r.Post("/api/v1/agents/pair", s.handlePairAgentDevice)
	r.Get("/api/agents/sessions", s.handleListAgentSessions)
	r.Get("/api/v1/agents/sessions", s.handleListAgentSessions)
	r.Delete("/api/agents/sessions/{id}", s.handleRevokeAgentSession)
	r.Delete("/api/v1/agents/sessions/{id}", s.handleRevokeAgentSession)

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
