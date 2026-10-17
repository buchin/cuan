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
