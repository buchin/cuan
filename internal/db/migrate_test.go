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
