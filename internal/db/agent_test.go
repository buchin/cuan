package db

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAgent_ValidationAndNormalization(t *testing.T) {
	// 1. Validation: Empty name
	inEmpty := &CreateAgentInput{Name: "   "}
	if err := inEmpty.Validate(); err != ErrEmptyAgentName {
		t.Fatalf("expected ErrEmptyAgentName, got %v", err)
	}

	// 2. Validation: Invalid status
	inBadStatus := &CreateAgentInput{Name: "Bot", Status: "destroyed"}
	if err := inBadStatus.Validate(); err != ErrInvalidAgentStatus {
		t.Fatalf("expected ErrInvalidAgentStatus, got %v", err)
	}

	// 3. Normalization & defaults
	inValid := &CreateAgentInput{
		Name:   "  Archie  ",
		Status: " ACTIVE ",
	}
	agent, err := inValid.ToEntity()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent.Name != "Archie" {
		t.Errorf("expected trimmed name 'Archie', got %q", agent.Name)
	}
	if agent.Status != AgentStatusActive {
		t.Errorf("expected normalized status 'active', got %q", agent.Status)
	}
	if agent.ModelID != DefaultAgentModelID {
		t.Errorf("expected default model ID %q, got %q", DefaultAgentModelID, agent.ModelID)
	}
	if agent.Provider != DefaultAgentProvider {
		t.Errorf("expected default provider %q, got %q", DefaultAgentProvider, agent.Provider)
	}
	if agent.ID == "" {
		t.Errorf("expected non-empty generated ID")
	}

	// 4. UpdateAgentInput validation & ApplyTo
	badUpdate := &UpdateAgentInput{Name: strPtr("   ")}
	if err := badUpdate.Validate(); err != ErrEmptyAgentName {
		t.Fatalf("expected ErrEmptyAgentName on update, got %v", err)
	}
	badStatusUpdate := &UpdateAgentInput{Status: strPtr("unsupported")}
	if err := badStatusUpdate.Validate(); err != ErrInvalidAgentStatus {
		t.Fatalf("expected ErrInvalidAgentStatus on update, got %v", err)
	}

	validUpdate := &UpdateAgentInput{
		Name:         strPtr("Archie Prime"),
		Role:         strPtr("Lead Architect"),
		SystemPrompt: strPtr("You are an expert architect."),
		ModelID:      strPtr("gemini-2.0-flash"),
		Status:       strPtr("INACTIVE"),
	}
	if err := validUpdate.ApplyTo(agent); err != nil {
		t.Fatalf("failed to apply update: %v", err)
	}
	if agent.Name != "Archie Prime" || agent.Role != "Lead Architect" || agent.ModelID != "gemini-2.0-flash" || agent.Status != AgentStatusInactive {
		t.Errorf("updated agent state mismatch: %+v", agent)
	}
}

func TestAgentConnection_ValidationAndNormalization(t *testing.T) {
	// 1. Validation: Empty user_id
	inEmptyUser := &CreateAgentConnectionInput{UserID: "  "}
	if err := inEmptyUser.Validate(); err != ErrEmptyUserID {
		t.Fatalf("expected ErrEmptyUserID, got %v", err)
	}

	// 2. Validation: Invalid status
	inBadStatus := &CreateAgentConnectionInput{UserID: "user_1", Status: "expired"}
	if err := inBadStatus.Validate(); err != ErrInvalidConnectionStatus {
		t.Fatalf("expected ErrInvalidConnectionStatus, got %v", err)
	}

	// 3. Normalization & defaults
	inValid := &CreateAgentConnectionInput{
		UserID:               "user_101",
		CredentialsEncrypted: "encrypted_blob_data",
		PlanHint:             "pay-as-you-go",
	}
	conn, err := inValid.ToEntity()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conn.Provider != DefaultConnectionProvider {
		t.Errorf("expected default provider %q, got %q", DefaultConnectionProvider, conn.Provider)
	}
	if conn.Status != ConnectionStatusActive {
		t.Errorf("expected default status %q, got %q", ConnectionStatusActive, conn.Status)
	}
	if conn.ID == "" {
		t.Errorf("expected generated connection ID")
	}

	// 4. UpdateAgentConnectionInput validation & ApplyTo
	badUpdate := &UpdateAgentConnectionInput{Status: strPtr("revoked")}
	if err := badUpdate.Validate(); err != ErrInvalidConnectionStatus {
		t.Fatalf("expected ErrInvalidConnectionStatus on update, got %v", err)
	}

	validUpdate := &UpdateAgentConnectionInput{
		Status:   strPtr("DISCONNECTED"),
		PlanHint: strPtr("enterprise"),
	}
	if err := validUpdate.ApplyTo(conn); err != nil {
		t.Fatalf("failed to apply connection update: %v", err)
	}
	if conn.Status != ConnectionStatusDisconnected || conn.PlanHint != "enterprise" {
		t.Errorf("updated connection mismatch: %+v", conn)
	}
}

func TestAgent_DTOConversion(t *testing.T) {
	now := time.Now().UTC()
	agent := &Agent{
		ID:           "ag_123",
		Name:         "Scrum Bot",
		Avatar:       "https://avatar.com/bot.png",
		Role:         "Scrum Master",
		SystemPrompt: "Facilitate standups",
		ModelID:      "gemini-1.5-pro",
		Provider:     "google",
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	dto := agent.ToDTO()
	if dto.ID != agent.ID || dto.Name != agent.Name || dto.Role != agent.Role || dto.ModelID != agent.ModelID {
		t.Errorf("agent DTO mismatch: %+v", dto)
	}

	connWithSecret := &AgentConnection{
		ID:                   "conn_123",
		UserID:               "user_admin",
		Provider:             "google",
		CredentialsEncrypted: "SUPER_SECRET_TOKEN",
		PlanHint:             "tier-3",
		Status:               "active",
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	connDTO := connWithSecret.ToDTO()
	if connDTO.ID != connWithSecret.ID || connDTO.UserID != connWithSecret.UserID {
		t.Errorf("connection DTO mismatch: %+v", connDTO)
	}
	if !connDTO.HasCredentials {
		t.Errorf("expected HasCredentials to be true")
	}

	connWithoutSecret := &AgentConnection{
		ID:                   "conn_456",
		UserID:               "user_guest",
		Provider:             "google",
		CredentialsEncrypted: "",
		PlanHint:             "free",
		Status:               "active",
	}
	if connWithoutSecret.ToDTO().HasCredentials {
		t.Errorf("expected HasCredentials to be false when encrypted credentials are empty")
	}
}

func TestMemoryAgentRepository_AgentCRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryAgentRepository()

	// 1. Create agent
	a1 := &Agent{
		ID:           "agent-01",
		Name:         "Antigravity",
		Role:         "Pair Programmer",
		SystemPrompt: "Code meticulously",
		ModelID:      "gemini-1.5-pro",
		Provider:     "google",
		Status:       "active",
		CreatedAt:    time.Now().UTC().Add(-1 * time.Hour),
	}
	if err := repo.CreateAgent(ctx, a1); err != nil {
		t.Fatalf("CreateAgent failed: %v", err)
	}

	// 2. Duplicate error
	if err := repo.CreateAgent(ctx, a1); err != ErrDuplicateAgent {
		t.Fatalf("expected ErrDuplicateAgent, got %v", err)
	}

	// 3. GetByID
	fetched, err := repo.GetAgentByID(ctx, "agent-01")
	if err != nil {
		t.Fatalf("GetAgentByID failed: %v", err)
	}
	if fetched.Name != "Antigravity" {
		t.Errorf("expected Antigravity, got %s", fetched.Name)
	}

	// 4. GetByID Not Found
	if _, err := repo.GetAgentByID(ctx, "non-existent"); err != ErrAgentNotFound {
		t.Fatalf("expected ErrAgentNotFound, got %v", err)
	}

	// 5. Add second agent
	a2 := &Agent{
		ID:           "agent-02",
		Name:         "Code Reviewer",
		Role:         "Auditor",
		SystemPrompt: "Check security and conventions",
		ModelID:      "claude-3-opus",
		Provider:     "anthropic",
		Status:       "inactive",
		CreatedAt:    time.Now().UTC(),
	}
	if err := repo.CreateAgent(ctx, a2); err != nil {
		t.Fatalf("CreateAgent a2 failed: %v", err)
	}

	// 6. List and filter
	listAll, err := repo.ListAgents(ctx, AgentFilter{})
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}
	if len(listAll) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(listAll))
	}
	// Verify sorting by CreatedAt DESC (a2 first, then a1)
	if listAll[0].ID != "agent-02" {
		t.Errorf("expected a2 first due to newer CreatedAt, got %s", listAll[0].ID)
	}

	// Filter by provider
	listAnthropic, err := repo.ListAgents(ctx, AgentFilter{Provider: "anthropic"})
	if err != nil || len(listAnthropic) != 1 || listAnthropic[0].ID != "agent-02" {
		t.Errorf("filter by provider failed: %+v", listAnthropic)
	}

	// Filter by status
	listActive, err := repo.ListAgents(ctx, AgentFilter{Status: "active"})
	if err != nil || len(listActive) != 1 || listActive[0].ID != "agent-01" {
		t.Errorf("filter by status failed: %+v", listActive)
	}

	// Filter by search
	listSearch, err := repo.ListAgents(ctx, AgentFilter{Search: "programmer"})
	if err != nil || len(listSearch) != 1 || listSearch[0].ID != "agent-01" {
		t.Errorf("filter by search failed: %+v", listSearch)
	}

	// Count
	cnt, err := repo.CountAgents(ctx, AgentFilter{Status: "active"})
	if err != nil || cnt != 1 {
		t.Errorf("CountAgents mismatch: %d, err: %v", cnt, err)
	}

	// 7. Update
	fetched.Name = "Antigravity 2.0"
	fetched.Status = "archived"
	if err := repo.UpdateAgent(ctx, fetched); err != nil {
		t.Fatalf("UpdateAgent failed: %v", err)
	}
	updated, _ := repo.GetAgentByID(ctx, "agent-01")
	if updated.Name != "Antigravity 2.0" || updated.Status != "archived" {
		t.Errorf("update mismatch: %+v", updated)
	}

	// Update non-existent
	if err := repo.UpdateAgent(ctx, &Agent{ID: "phantom", Name: "Ghost"}); err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound on phantom update, got %v", err)
	}

	// 8. Delete
	if err := repo.DeleteAgent(ctx, "agent-01"); err != nil {
		t.Fatalf("DeleteAgent failed: %v", err)
	}
	if _, err := repo.GetAgentByID(ctx, "agent-01"); err != ErrAgentNotFound {
		t.Errorf("expected deleted agent to return ErrAgentNotFound, got %v", err)
	}

	// Delete non-existent
	if err := repo.DeleteAgent(ctx, "agent-01"); err != ErrAgentNotFound {
		t.Errorf("expected ErrAgentNotFound on redundant delete, got %v", err)
	}
}

func TestMemoryAgentRepository_ConnectionCRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryAgentRepository()

	// 1. Create connection
	c1 := &AgentConnection{
		ID:                   "conn-01",
		UserID:               "user_alpha",
		Provider:             "google",
		CredentialsEncrypted: "token_123",
		PlanHint:             "pro",
		Status:               "active",
		CreatedAt:            time.Now().UTC().Add(-10 * time.Minute),
	}
	if err := repo.CreateConnection(ctx, c1); err != nil {
		t.Fatalf("CreateConnection failed: %v", err)
	}

	// 2. Duplicate error
	if err := repo.CreateConnection(ctx, c1); err != ErrDuplicateConnection {
		t.Fatalf("expected ErrDuplicateConnection, got %v", err)
	}

	// 3. GetByID
	fetched, err := repo.GetConnectionByID(ctx, "conn-01")
	if err != nil {
		t.Fatalf("GetConnectionByID failed: %v", err)
	}
	if fetched.UserID != "user_alpha" {
		t.Errorf("expected user_alpha, got %s", fetched.UserID)
	}

	// 4. GetByUserAndProvider
	byUser, err := repo.GetConnectionByUserAndProvider(ctx, "user_alpha", "google")
	if err != nil {
		t.Fatalf("GetConnectionByUserAndProvider failed: %v", err)
	}
	if byUser.ID != "conn-01" {
		t.Errorf("expected conn-01, got %s", byUser.ID)
	}

	// Not found
	if _, err := repo.GetConnectionByUserAndProvider(ctx, "user_alpha", "anthropic"); err != ErrAgentConnectionNotFound {
		t.Errorf("expected ErrAgentConnectionNotFound, got %v", err)
	}

	// 5. List with filters
	list, err := repo.ListConnections(ctx, AgentConnectionFilter{UserID: "user_alpha"})
	if err != nil || len(list) != 1 {
		t.Fatalf("ListConnections failed: len=%d, err=%v", len(list), err)
	}

	cnt, err := repo.CountConnections(ctx, AgentConnectionFilter{UserID: "user_alpha"})
	if err != nil || cnt != 1 {
		t.Errorf("CountConnections failed: %d, err: %v", cnt, err)
	}

	// 6. Update connection
	fetched.PlanHint = "enterprise"
	fetched.Status = "invalid"
	if err := repo.UpdateConnection(ctx, fetched); err != nil {
		t.Fatalf("UpdateConnection failed: %v", err)
	}
	updated, _ := repo.GetConnectionByID(ctx, "conn-01")
	if updated.PlanHint != "enterprise" || updated.Status != "invalid" {
		t.Errorf("updated connection mismatch: %+v", updated)
	}

	// 7. Delete connection
	if err := repo.DeleteConnection(ctx, "conn-01"); err != nil {
		t.Fatalf("DeleteConnection failed: %v", err)
	}
	if _, err := repo.GetConnectionByID(ctx, "conn-01"); err != ErrAgentConnectionNotFound {
		t.Errorf("expected ErrAgentConnectionNotFound, got %v", err)
	}
}

func TestMemoryAgentRepository_ConcurrencyThreadSafety(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryAgentRepository()

	var wg sync.WaitGroup
	workers := 20
	iterations := 50

	for i := 0; i < workers; i++ {
		wg.Add(1)
		workerID := i
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				agentID := fmt.Sprintf("agent-%d-%d", workerID, j)
				agent := &Agent{
					ID:           agentID,
					Name:         fmt.Sprintf("Bot %d-%d", workerID, j),
					Role:         "Tester",
					SystemPrompt: "Testing concurrent memory store",
					ModelID:      "gemini-1.5-pro",
					Provider:     "google",
					Status:       "active",
				}
				_ = repo.CreateAgent(ctx, agent)
				_, _ = repo.GetAgentByID(ctx, agentID)
				_, _ = repo.ListAgents(ctx, AgentFilter{Limit: 5})
				_, _ = repo.CountAgents(ctx, AgentFilter{})

				connID := fmt.Sprintf("conn-%d-%d", workerID, j)
				conn := &AgentConnection{
					ID:                   connID,
					UserID:               fmt.Sprintf("user-%d", workerID),
					Provider:             "google",
					CredentialsEncrypted: "secret",
					PlanHint:             "payg",
					Status:               "active",
				}
				_ = repo.CreateConnection(ctx, conn)
				_, _ = repo.GetConnectionByID(ctx, connID)
				_, _ = repo.ListConnections(ctx, AgentConnectionFilter{Limit: 5})
			}
		}()
	}

	wg.Wait()

	totalAgents, err := repo.CountAgents(ctx, AgentFilter{})
	if err != nil {
		t.Fatalf("CountAgents failed after concurrency test: %v", err)
	}
	expected := workers * iterations
	if totalAgents != expected {
		t.Errorf("expected %d agents, got %d", expected, totalAgents)
	}
}

func TestRepositories_AgentIntegration(t *testing.T) {
	memRepos := NewMemoryRepositories()
	if memRepos.Agents == nil {
		t.Fatalf("expected MemoryRepositories.Agents to be initialized")
	}

	// Verify we can use memRepos.Agents
	ctx := context.Background()
	testAgent := &Agent{
		ID:     "integrated-agent",
		Name:   "Integrated Bot",
		Status: "active",
	}
	if err := memRepos.Agents.CreateAgent(ctx, testAgent); err != nil {
		t.Fatalf("failed to create agent via Repositories.Agents: %v", err)
	}

	retrieved, err := memRepos.Agents.GetAgentByID(ctx, "integrated-agent")
	if err != nil || retrieved.Name != "Integrated Bot" {
		t.Errorf("retrieved agent mismatch: %+v, err: %v", retrieved, err)
	}
}

func strPtr(s string) *string {
	return &s
}
