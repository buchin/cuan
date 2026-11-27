package db

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRemoteMCPConnectorMigrationLoaded(t *testing.T) {
	migs, err := LoadMigrations()
	if err != nil {
		t.Fatalf("LoadMigrations() unexpected error: %v", err)
	}
	var found *Migration
	for i := range migs {
		if migs[i].Version == "000048" {
			found = &migs[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("expected migration 000048_remote_mcp_connectors to be loaded")
	}
	if !strings.Contains(found.UpSQL, "CREATE TABLE IF NOT EXISTS remote_mcp_connectors") {
		t.Errorf("expected 000048 UpSQL to create remote_mcp_connectors table")
	}
	if !strings.Contains(found.DownSQL, "DROP TABLE IF EXISTS remote_mcp_connectors") {
		t.Errorf("expected 000048 DownSQL to drop remote_mcp_connectors table")
	}
}

func TestMemoryRemoteMCPConnectorRepository_CRUD(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRemoteMCPConnectorRepository()

	if err := repo.UpsertRemoteMCPConnector(ctx, &RemoteMCPConnector{}); !errors.Is(err, ErrRemoteMCPInvalidInput) {
		t.Fatalf("expected ErrRemoteMCPInvalidInput for empty connector, got %v", err)
	}

	c := &RemoteMCPConnector{
		ID:              "mcp_tebar",
		Name:            "Tebar",
		Description:     "Tebar MCP",
		BaseURL:         "https://api.tebar.app/mcp",
		Transport:       "http",
		APIKeyEncrypted: "enc-secret",
		Enabled:         true,
	}
	if err := repo.UpsertRemoteMCPConnector(ctx, c); err != nil {
		t.Fatalf("UpsertRemoteMCPConnector failed: %v", err)
	}

	got, err := repo.GetRemoteMCPConnector(ctx, "mcp_tebar")
	if err != nil {
		t.Fatalf("GetRemoteMCPConnector failed: %v", err)
	}
	if got.Name != "Tebar" || got.BaseURL != "https://api.tebar.app/mcp" || !got.Enabled {
		t.Fatalf("unexpected connector: %+v", got)
	}

	// Update existing
	c.Name = "Tebar Updated"
	c.Enabled = false
	if err := repo.UpsertRemoteMCPConnector(ctx, c); err != nil {
		t.Fatalf("UpsertRemoteMCPConnector update failed: %v", err)
	}

	list, err := repo.ListRemoteMCPConnectors(ctx)
	if err != nil {
		t.Fatalf("ListRemoteMCPConnectors failed: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Tebar Updated" || list[0].Enabled {
		t.Fatalf("unexpected list result: %+v", list)
	}

	if err := repo.DeleteRemoteMCPConnector(ctx, "mcp_tebar"); err != nil {
		t.Fatalf("DeleteRemoteMCPConnector failed: %v", err)
	}
	if _, err := repo.GetRemoteMCPConnector(ctx, "mcp_tebar"); !errors.Is(err, ErrRemoteMCPNotFound) {
		t.Fatalf("expected ErrRemoteMCPNotFound after delete, got %v", err)
	}
}
