package db

import "database/sql"

// Repositories aggregates all repository interfaces in cuan internal/db.
type Repositories struct {
	Servers        ServerRepository
	Domains        DomainRepository
	Observability  ObservabilityRepository
	ProjectStudio  ProjectStudioRepository
	BatchCampaigns BatchCampaignRepository
	Agents              AgentRepository
	Projects            ProjectRepository
	ProjectIntegrations ProjectIntegrationRepository
}

// NewSQLRepositories constructs all SQL-backed repositories.
func NewSQLRepositories(sqlDB *sql.DB) *Repositories {
	return &Repositories{
		Servers:             NewSQLServerRepository(sqlDB),
		Domains:             NewSQLDomainRepository(sqlDB),
		Observability:       NewSQLObservabilityRepository(sqlDB),
		ProjectStudio:       NewSQLProjectStudioRepository(sqlDB),
		BatchCampaigns:      NewSQLBatchCampaignRepository(sqlDB),
		Agents:              NewSQLAgentRepository(sqlDB),
		Projects:            NewSQLProjectRepository(sqlDB),
		ProjectIntegrations: NewSQLProjectIntegrationRepository(sqlDB),
	}
}

// NewMemoryRepositories constructs all thread-safe in-memory repositories.
func NewMemoryRepositories() *Repositories {
	return &Repositories{
		Servers:             NewMemoryServerRepository(),
		Domains:             NewMemoryDomainRepository(),
		Observability:       NewMemoryObservabilityRepository(),
		ProjectStudio:       NewMemoryProjectStudioRepository(),
		BatchCampaigns:      NewMemoryBatchCampaignRepository(),
		Agents:              NewMemoryAgentRepository(),
		Projects:            NewMemoryProjectRepository(),
		ProjectIntegrations: NewMemoryProjectIntegrationRepository(),
	}
}
