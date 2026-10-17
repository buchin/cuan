package db

import (
	"strings"
	"testing"
)

func TestLoadMigrations(t *testing.T) {
	migs, err := LoadMigrations()
	if err != nil {
		t.Fatalf("LoadMigrations() unexpected error: %v", err)
	}

	if len(migs) == 0 {
		t.Fatalf("expected at least 1 migration, got 0")
	}

	baseline := migs[0]
	if baseline.Version != "000001" {
		t.Errorf("expected version 000001, got %s", baseline.Version)
	}
	if baseline.Name != "baseline" {
		t.Errorf("expected name baseline, got %s", baseline.Name)
	}
	if !strings.Contains(baseline.UpSQL, "CREATE TABLE IF NOT EXISTS cuan_schema_migrations") {
		t.Errorf("expected baseline UpSQL to create cuan_schema_migrations table")
	}
	if !strings.Contains(baseline.UpSQL, "CREATE TABLE IF NOT EXISTS owners") {
		t.Errorf("expected baseline UpSQL to create owners table")
	}
	if !strings.Contains(baseline.UpSQL, "CREATE TABLE IF NOT EXISTS tasks") {
		t.Errorf("expected baseline UpSQL to create tasks table")
	}
	if !strings.Contains(baseline.DownSQL, "DROP TABLE IF EXISTS") {
		t.Errorf("expected baseline DownSQL to drop tables")
	}

	if len(migs) < 7 {
		t.Fatalf("expected at least 7 migrations, got %d", len(migs))
	}
	sessionsMig := migs[1]
	if sessionsMig.Version != "000002" {
		t.Errorf("expected version 000002, got %s", sessionsMig.Version)
	}
	if !strings.Contains(sessionsMig.UpSQL, "owner_sessions") {
		t.Errorf("expected 000002 UpSQL to create owner_sessions table")
	}

	sponsorshipMig := migs[6]
	if sponsorshipMig.Version != "000007" {
		t.Errorf("expected version 000007, got %s", sponsorshipMig.Version)
	}
	if !strings.Contains(sponsorshipMig.UpSQL, "sponsorship_batches") {
		t.Errorf("expected 000007 UpSQL to create sponsorship_batches table")
	}
	if !strings.Contains(sponsorshipMig.UpSQL, "sponsors") {
		t.Errorf("expected 000007 UpSQL to create sponsors table")
	}
	if !strings.Contains(sponsorshipMig.UpSQL, "sponsorship_orders") {
		t.Errorf("expected 000007 UpSQL to create sponsorship_orders table")
	}
	if !strings.Contains(sponsorshipMig.UpSQL, "waiting_list_leads") {
		t.Errorf("expected 000007 UpSQL to create waiting_list_leads table")
	}

	if len(migs) < 8 {
		t.Fatalf("expected at least 8 migrations, got %d", len(migs))
	}
	deployerMig := migs[7]
	if deployerMig.Version != "000008" {
		t.Errorf("expected version 000008, got %s", deployerMig.Version)
	}
	if !strings.Contains(deployerMig.UpSQL, "deployments") {
		t.Errorf("expected 000008 UpSQL to create deployments table")
	}
	if !strings.Contains(deployerMig.UpSQL, "deployment_logs") {
		t.Errorf("expected 000008 UpSQL to create deployment_logs table")
	}
	if !strings.Contains(deployerMig.DownSQL, "DROP TABLE IF EXISTS deployments") {
		t.Errorf("expected 000008 DownSQL to drop deployments table")
	}

	if len(migs) < 9 {
		t.Fatalf("expected at least 9 migrations, got %d", len(migs))
	}
	byosMig := migs[8]
	if byosMig.Version != "000009" {
		t.Errorf("expected version 000009, got %s", byosMig.Version)
	}
	if byosMig.Name != "servers_byos" {
		t.Errorf("expected name servers_byos, got %s", byosMig.Name)
	}
	if !strings.Contains(byosMig.UpSQL, "CREATE TABLE IF NOT EXISTS servers") {
		t.Errorf("expected 000009 UpSQL to create servers table")
	}
	if !strings.Contains(byosMig.UpSQL, "ALTER TABLE deployments ADD COLUMN IF NOT EXISTS server_id") {
		t.Errorf("expected 000009 UpSQL to alter deployments table")
	}
	if !strings.Contains(byosMig.DownSQL, "DROP TABLE IF EXISTS servers CASCADE") {
		t.Errorf("expected 000009 DownSQL to drop servers table")
	}
	if !strings.Contains(byosMig.DownSQL, "ALTER TABLE deployments DROP COLUMN IF EXISTS server_id") {
		t.Errorf("expected 000009 DownSQL to drop server_id column")
	}

	if len(migs) < 10 {
		t.Fatalf("expected at least 10 migrations, got %d", len(migs))
	}
	customDomainsMig := migs[9]
	if customDomainsMig.Version != "000010" {
		t.Errorf("expected version 000010, got %s", customDomainsMig.Version)
	}
	if customDomainsMig.Name != "custom_domains" {
		t.Errorf("expected name custom_domains, got %s", customDomainsMig.Name)
	}
	if !strings.Contains(customDomainsMig.UpSQL, "CREATE TABLE IF NOT EXISTS custom_domains") {
		t.Errorf("expected 000010 UpSQL to create custom_domains table")
	}
	if !strings.Contains(customDomainsMig.UpSQL, "REFERENCES deployments(id) ON DELETE CASCADE") {
		t.Errorf("expected 000010 UpSQL to reference deployments(id) with CASCADE")
	}
	if !strings.Contains(customDomainsMig.UpSQL, "idx_custom_domains_deployment_id") {
		t.Errorf("expected 000010 UpSQL to create index on deployment_id")
	}
	if !strings.Contains(customDomainsMig.UpSQL, "idx_custom_domains_domain") {
		t.Errorf("expected 000010 UpSQL to create index on domain")
	}
	if !strings.Contains(customDomainsMig.DownSQL, "DROP TABLE IF EXISTS custom_domains CASCADE") {
		t.Errorf("expected 000010 DownSQL to drop custom_domains table")
	}

	if len(migs) < 11 {
		t.Fatalf("expected at least 11 migrations, got %d", len(migs))
	}
	obsMig := migs[10]
	if obsMig.Version != "000011" {
		t.Errorf("expected version 000011, got %s", obsMig.Version)
	}
	if obsMig.Name != "observability_alerts" {
		t.Errorf("expected name observability_alerts, got %s", obsMig.Name)
	}
	if !strings.Contains(obsMig.UpSQL, "CREATE TABLE IF NOT EXISTS health_checks") {
		t.Errorf("expected 000011 UpSQL to create health_checks table")
	}
	if !strings.Contains(obsMig.UpSQL, "CREATE TABLE IF NOT EXISTS server_metrics") {
		t.Errorf("expected 000011 UpSQL to create server_metrics table")
	}
	if !strings.Contains(obsMig.UpSQL, "CREATE TABLE IF NOT EXISTS alert_channels") {
		t.Errorf("expected 000011 UpSQL to create alert_channels table")
	}
	if !strings.Contains(obsMig.UpSQL, "CREATE TABLE IF NOT EXISTS alert_incidents") {
		t.Errorf("expected 000011 UpSQL to create alert_incidents table")
	}
	if !strings.Contains(obsMig.DownSQL, "DROP TABLE IF EXISTS health_checks CASCADE") {
		t.Errorf("expected 000011 DownSQL to drop health_checks table")
	}
	if !strings.Contains(obsMig.DownSQL, "DROP TABLE IF EXISTS server_metrics CASCADE") {
		t.Errorf("expected 000011 DownSQL to drop server_metrics table")
	}
	if !strings.Contains(obsMig.DownSQL, "DROP TABLE IF EXISTS alert_channels CASCADE") {
		t.Errorf("expected 000011 DownSQL to drop alert_channels table")
	}
	if !strings.Contains(obsMig.DownSQL, "DROP TABLE IF EXISTS alert_incidents CASCADE") {
		t.Errorf("expected 000011 DownSQL to drop alert_incidents table")
	}

	if len(migs) < 12 {
		t.Fatalf("expected at least 12 migrations, got %d", len(migs))
	}
	studioMig := migs[11]
	if studioMig.Version != "000012" {
		t.Errorf("expected version 000012, got %s", studioMig.Version)
	}
	if studioMig.Name != "project_discussions_tasks" {
		t.Errorf("expected name project_discussions_tasks, got %s", studioMig.Name)
	}
	if !strings.Contains(studioMig.UpSQL, "CREATE TABLE IF NOT EXISTS project_rooms") {
		t.Errorf("expected 000012 UpSQL to create project_rooms table")
	}
	if !strings.Contains(studioMig.UpSQL, "CREATE TABLE IF NOT EXISTS project_messages") {
		t.Errorf("expected 000012 UpSQL to create project_messages table")
	}
	if !strings.Contains(studioMig.UpSQL, "CREATE TABLE IF NOT EXISTS project_tasks") {
		t.Errorf("expected 000012 UpSQL to create project_tasks table")
	}
	if !strings.Contains(studioMig.UpSQL, "parent_task_id UUID REFERENCES project_tasks(id)") {
		t.Errorf("expected 000012 UpSQL to have parent_task_id FK")
	}
	if !strings.Contains(studioMig.UpSQL, "batch_id VARCHAR(255)") {
		t.Errorf("expected 000012 UpSQL to have batch_id")
	}
	if !strings.Contains(studioMig.UpSQL, "tags JSONB NOT NULL DEFAULT '[]'::jsonb") {
		t.Errorf("expected 000012 UpSQL to have tags JSONB")
	}
	if !strings.Contains(studioMig.DownSQL, "DROP TABLE IF EXISTS project_tasks CASCADE") {
		t.Errorf("expected 000012 DownSQL to drop project_tasks table")
	}
	if !strings.Contains(studioMig.DownSQL, "DROP TABLE IF EXISTS project_messages CASCADE") {
		t.Errorf("expected 000012 DownSQL to drop project_messages table")
	}
	if !strings.Contains(studioMig.DownSQL, "DROP TABLE IF EXISTS project_rooms CASCADE") {
		t.Errorf("expected 000012 DownSQL to drop project_rooms table")
	}

	if len(migs) < 13 {
		t.Fatalf("expected at least 13 migrations, got %d", len(migs))
	}
	batchMig := migs[12]
	if batchMig.Version != "000013" {
		t.Errorf("expected version 000013, got %s", batchMig.Version)
	}
	if batchMig.Name != "batch_campaigns" {
		t.Errorf("expected name batch_campaigns, got %s", batchMig.Name)
	}
	if !strings.Contains(batchMig.UpSQL, "CREATE TABLE IF NOT EXISTS batch_campaigns") {
		t.Errorf("expected 000013 UpSQL to create batch_campaigns table")
	}
	if !strings.Contains(batchMig.UpSQL, "template_payload JSONB") {
		t.Errorf("expected 000013 UpSQL to have template_payload JSONB")
	}
	if !strings.Contains(batchMig.UpSQL, "target_criteria JSONB") {
		t.Errorf("expected 000013 UpSQL to have target_criteria JSONB")
	}
	if !strings.Contains(batchMig.UpSQL, "idx_batch_campaigns_status") {
		t.Errorf("expected 000013 UpSQL to create idx_batch_campaigns_status")
	}
	if !strings.Contains(batchMig.DownSQL, "DROP TABLE IF EXISTS batch_campaigns CASCADE") {
		t.Errorf("expected 000013 DownSQL to drop batch_campaigns table")
	}

	if len(migs) < 14 {
		t.Fatalf("expected at least 14 migrations, got %d", len(migs))
	}
	agentMig := migs[13]
	if agentMig.Version != "000014" {
		t.Errorf("expected version 000014, got %s", agentMig.Version)
	}
	if agentMig.Name != "agent_hub_terminal" {
		t.Errorf("expected name agent_hub_terminal, got %s", agentMig.Name)
	}
	if !strings.Contains(agentMig.UpSQL, "CREATE TABLE IF NOT EXISTS agents") {
		t.Errorf("expected 000014 UpSQL to create agents table")
	}
	if !strings.Contains(agentMig.UpSQL, "CREATE TABLE IF NOT EXISTS agent_connections") {
		t.Errorf("expected 000014 UpSQL to create agent_connections table")
	}
	if !strings.Contains(agentMig.UpSQL, "idx_agents_status") {
		t.Errorf("expected 000014 UpSQL to create idx_agents_status")
	}
	if !strings.Contains(agentMig.UpSQL, "idx_agent_connections_user_id") {
		t.Errorf("expected 000014 UpSQL to create idx_agent_connections_user_id")
	}
	if !strings.Contains(agentMig.DownSQL, "DROP TABLE IF EXISTS agent_connections CASCADE") {
		t.Errorf("expected 000014 DownSQL to drop agent_connections table")
	}
	if len(migs) < 15 {
		t.Fatalf("expected at least 15 migrations, got %d", len(migs))
	}
	creatorMig := migs[14]
	if creatorMig.Version != "000015" {
		t.Errorf("expected version 000015, got %s", creatorMig.Version)
	}
	if creatorMig.Name != "creator_blueprints_execution" {
		t.Errorf("expected name creator_blueprints_execution, got %s", creatorMig.Name)
	}
	if !strings.Contains(creatorMig.UpSQL, "ADD COLUMN IF NOT EXISTS project_type") {
		t.Errorf("expected 000015 UpSQL to add project_type column")
	}
	if !strings.Contains(creatorMig.UpSQL, "ADD COLUMN IF NOT EXISTS metadata JSONB") {
		t.Errorf("expected 000015 UpSQL to add metadata column")
	}
	if !strings.Contains(creatorMig.UpSQL, "idx_projects_project_type") {
		t.Errorf("expected 000015 UpSQL to create idx_projects_project_type")
	}
	if !strings.Contains(creatorMig.UpSQL, "ADD COLUMN IF NOT EXISTS agent_output") {
		t.Errorf("expected 000015 UpSQL to add agent_output column")
	}
	if !strings.Contains(creatorMig.UpSQL, "ADD COLUMN IF NOT EXISTS execution_status") {
		t.Errorf("expected 000015 UpSQL to add execution_status column")
	}
	if !strings.Contains(creatorMig.UpSQL, "idx_project_tasks_execution_status") {
		t.Errorf("expected 000015 UpSQL to create idx_project_tasks_execution_status")
	}
	if !strings.Contains(creatorMig.DownSQL, "DROP COLUMN IF EXISTS execution_status") {
		t.Errorf("expected 000015 DownSQL to drop execution_status column")
	}
	if !strings.Contains(creatorMig.DownSQL, "DROP COLUMN IF EXISTS metadata") {
		t.Errorf("expected 000015 DownSQL to drop metadata column")
	}

	if len(migs) < 16 {
		t.Fatalf("expected at least 16 migrations, got %d", len(migs))
	}
	modularMig := migs[15]
	if modularMig.Version != "000016" {
		t.Errorf("expected version 000016, got %s", modularMig.Version)
	}
	if modularMig.Name != "modular_workspaces_and_publishing" {
		t.Errorf("expected name modular_workspaces_and_publishing, got %s", modularMig.Name)
	}
	if !strings.Contains(modularMig.UpSQL, "ADD COLUMN IF NOT EXISTS enabled_modules") {
		t.Errorf("expected 000016 UpSQL to add enabled_modules column")
	}
	if !strings.Contains(modularMig.UpSQL, "ADD COLUMN IF NOT EXISTS source_type") {
		t.Errorf("expected 000016 UpSQL to add source_type column")
	}
	if !strings.Contains(modularMig.UpSQL, "ADD COLUMN IF NOT EXISTS source_metadata") {
		t.Errorf("expected 000016 UpSQL to add source_metadata column")
	}
	if !strings.Contains(modularMig.UpSQL, "CREATE TABLE IF NOT EXISTS project_integrations") {
		t.Errorf("expected 000016 UpSQL to create project_integrations table")
	}
	if !strings.Contains(modularMig.UpSQL, "ADD COLUMN IF NOT EXISTS scheduled_at") {
		t.Errorf("expected 000016 UpSQL to add scheduled_at column")
	}
	if !strings.Contains(modularMig.UpSQL, "ADD COLUMN IF NOT EXISTS published_at") {
		t.Errorf("expected 000016 UpSQL to add published_at column")
	}
	if !strings.Contains(modularMig.UpSQL, "ADD COLUMN IF NOT EXISTS publication_url") {
		t.Errorf("expected 000016 UpSQL to add publication_url column")
	}
	if !strings.Contains(modularMig.DownSQL, "DROP TABLE IF EXISTS project_integrations CASCADE") {
		t.Errorf("expected 000016 DownSQL to drop project_integrations table")
	}
	if !strings.Contains(modularMig.DownSQL, "DROP COLUMN IF EXISTS publication_url") {
		t.Errorf("expected 000016 DownSQL to drop publication_url column")
	}
	if !strings.Contains(modularMig.DownSQL, "DROP COLUMN IF EXISTS enabled_modules") {
		t.Errorf("expected 000016 DownSQL to drop enabled_modules column")
	}

	if len(migs) < 17 {
		t.Fatalf("expected at least 17 migrations, got %d", len(migs))
	}
	deployerMig17 := migs[16]
	if deployerMig17.Version != "000017" {
		t.Errorf("expected version 000017, got %s", deployerMig17.Version)
	}
	if deployerMig17.Name != "script_deployer_and_provisioner" {
		t.Errorf("expected name script_deployer_and_provisioner, got %s", deployerMig17.Name)
	}
	if !strings.Contains(deployerMig17.UpSQL, "CREATE TABLE IF NOT EXISTS script_deployments") {
		t.Errorf("expected 000017 UpSQL to create script_deployments table")
	}
	if !strings.Contains(deployerMig17.UpSQL, "CREATE TABLE IF NOT EXISTS provisioned_databases") {
		t.Errorf("expected 000017 UpSQL to create provisioned_databases table")
	}
	if !strings.Contains(deployerMig17.UpSQL, "idx_script_deployments_project_id") {
		t.Errorf("expected 000017 UpSQL to create idx_script_deployments_project_id")
	}
	if !strings.Contains(deployerMig17.UpSQL, "idx_provisioned_databases_project_id") {
		t.Errorf("expected 000017 UpSQL to create idx_provisioned_databases_project_id")
	}
	if !strings.Contains(deployerMig17.DownSQL, "DROP TABLE IF EXISTS provisioned_databases CASCADE") {
		t.Errorf("expected 000017 DownSQL to drop provisioned_databases table")
	}
	if !strings.Contains(deployerMig17.DownSQL, "DROP TABLE IF EXISTS script_deployments CASCADE") {
		t.Errorf("expected 000017 DownSQL to drop script_deployments table")
	}

	if len(migs) < 18 {
		t.Fatalf("expected at least 18 migrations, got %d", len(migs))
	}
	wpMig18 := migs[17]
	if wpMig18.Version != "000018" {
		t.Errorf("expected version 000018, got %s", wpMig18.Version)
	}
	if wpMig18.Name != "wordpress_turnkey" {
		t.Errorf("expected name wordpress_turnkey, got %s", wpMig18.Name)
	}
	if !strings.Contains(wpMig18.UpSQL, "CREATE TABLE IF NOT EXISTS wordpress_deployments") {
		t.Errorf("expected 000018 UpSQL to create wordpress_deployments table")
	}
	if !strings.Contains(wpMig18.UpSQL, "CREATE TABLE IF NOT EXISTS wordpress_plugins_themes") {
		t.Errorf("expected 000018 UpSQL to create wordpress_plugins_themes table")
	}
	if !strings.Contains(wpMig18.UpSQL, "CREATE TABLE IF NOT EXISTS wordpress_webmaster_actions") {
		t.Errorf("expected 000018 UpSQL to create wordpress_webmaster_actions table")
	}
	if !strings.Contains(wpMig18.UpSQL, "idx_wordpress_deployments_project_id") {
		t.Errorf("expected 000018 UpSQL to create idx_wordpress_deployments_project_id")
	}
	if !strings.Contains(wpMig18.UpSQL, "idx_wordpress_plugins_themes_slug") {
		t.Errorf("expected 000018 UpSQL to create idx_wordpress_plugins_themes_slug")
	}
	if !strings.Contains(wpMig18.UpSQL, "idx_wordpress_webmaster_actions_action_type") {
		t.Errorf("expected 000018 UpSQL to create idx_wordpress_webmaster_actions_action_type")
	}
	if !strings.Contains(wpMig18.DownSQL, "DROP TABLE IF EXISTS wordpress_webmaster_actions CASCADE") {
		t.Errorf("expected 000018 DownSQL to drop wordpress_webmaster_actions table")
	}
	if !strings.Contains(wpMig18.DownSQL, "DROP TABLE IF EXISTS wordpress_plugins_themes CASCADE") {
		t.Errorf("expected 000018 DownSQL to drop wordpress_plugins_themes table")
	}
	if !strings.Contains(wpMig18.DownSQL, "DROP TABLE IF EXISTS wordpress_deployments CASCADE") {
		t.Errorf("expected 000018 DownSQL to drop wordpress_deployments table")
	}
}

func TestDryRun(t *testing.T) {
	upPlan, err := DryRun("up")
	if err != nil {
		t.Fatalf("DryRun(up) unexpected error: %v", err)
	}
	if !strings.Contains(upPlan, "[UP] Migration: 000001 (baseline)") {
		t.Errorf("up plan missing baseline header")
	}

	downPlan, err := DryRun("down")
	if err != nil {
		t.Fatalf("DryRun(down) unexpected error: %v", err)
	}
	if !strings.Contains(downPlan, "[DOWN] Migration: 000001 (baseline)") {
		t.Errorf("down plan missing baseline header")
	}
}
