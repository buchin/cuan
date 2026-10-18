package db

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestS25SQLAgentConnectionRoundTrip(t *testing.T) {
	dsn := os.Getenv("CUAN_S25_DATABASE_URL")
	if dsn == "" {
		t.Skip("explicit isolated S25 database required")
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Path != "/cuan_s25" {
		t.Fatal("test may run only against the isolated cuan_s25 database")
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal("cannot open isolated staging database")
	}
	defer database.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo := NewSQLAgentRepository(database)
	id := "s25_schema_test_" + GenerateUUID()
	connectionID := "conn_" + id
	// This is an explicitly inactive storage fixture, never an authenticated agent.
	entity := &Agent{ID: id, Name: "S25 inactive storage fixture", Harness: "agy",
		ConnectionID: connectionID, ModelID: connectionID, Provider: "agy",
		Enabled: false, Status: AgentStatusInactive}
	if err := repo.CreateAgent(ctx, entity); err != nil {
		t.Fatal(err)
	}
	defer repo.DeleteAgent(context.Background(), id)
	connection := &AgentConnection{ID: connectionID, UserID: "default", Provider: "agy",
		Status: ConnectionStatusDisconnected}
	if err := repo.CreateConnection(ctx, connection); err != nil {
		t.Fatal(err)
	}
	defer repo.DeleteConnection(context.Background(), connectionID)
	saved, err := repo.GetAgentByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ConnectionID != connectionID || saved.Harness != "agy" || saved.Enabled {
		t.Fatal("CLI connection reference did not survive the actual SQL round trip")
	}
	if saved.ToDTO().ConnectionID != connectionID {
		t.Fatal("public DTO lost the persisted connection reference")
	}
	t.Log("Actual PostgreSQL round trip preserved CLI connection linkage; inactive fixture cleaned up. This does not prove OAuth or a provider reply.")
}
