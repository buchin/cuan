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

	if len(migs) < 2 {
		t.Fatalf("expected at least 2 migrations, got %d", len(migs))
	}
	sessionsMig := migs[1]
	if sessionsMig.Version != "000002" {
		t.Errorf("expected version 000002, got %s", sessionsMig.Version)
	}
	if !strings.Contains(sessionsMig.UpSQL, "owner_sessions") {
		t.Errorf("expected 000002 UpSQL to create owner_sessions table")
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
