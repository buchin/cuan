package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/project"
)

func TestParseInboxProject(t *testing.T) {
	// 1. Valid create
	validCreate := "Saya buatkan project baru.\n```cuan-project\n{\"action\":\"create\",\"name\":\"Shop App\"}\n```"
	clean, act, err := parseInboxProject(validCreate)
	if err != nil || act == nil {
		t.Fatalf("expected valid project parse, got: %v", err)
	}
	if clean != "Saya buatkan project baru." {
		t.Fatalf("unexpected clean text: %s", clean)
	}
	if act.Action != "create" || act.Name != "Shop App" {
		t.Fatalf("unexpected parsed action: %+v", act)
	}

	// 2. Valid set_env
	validEnv := "Set Stripe key.\n```cuan-project\n{\"action\":\"set_env\",\"project_id\":\"proj_123\",\"env_key\":\"STRIPE_SECRET\",\"env_value\":\"sk_test_abc\"}\n```"
	clean, act, err = parseInboxProject(validEnv)
	if err != nil || act == nil {
		t.Fatalf("expected valid set_env parse, got: %v", err)
	}
	if act.Action != "set_env" || act.ProjectID != "proj_123" || act.EnvKey != "STRIPE_SECRET" || act.EnvValue != "sk_test_abc" {
		t.Fatalf("unexpected parsed action: %+v", act)
	}

	// 3. Valid archive & unarchive
	validArchive := "Arsipkan project.\n```cuan-project\n{\"action\":\"archive\",\"project_id\":\"proj_123\"}\n```"
	_, act, err = parseInboxProject(validArchive)
	if err != nil || act == nil || act.Action != "archive" {
		t.Fatalf("expected valid archive parse, got: %v", err)
	}

	// 4. Invalid: create without name
	badCreate := "```cuan-project\n{\"action\":\"create\",\"name\":\"\"}\n```"
	if _, _, err := parseInboxProject(badCreate); err == nil {
		t.Fatal("expected error for empty name on create")
	}

	// 5. Invalid: set_env without env_key
	badEnv := "```cuan-project\n{\"action\":\"set_env\",\"project_id\":\"proj_123\",\"env_key\":\"\"}\n```"
	if _, _, err := parseInboxProject(badEnv); err == nil {
		t.Fatal("expected error for empty env_key")
	}
}

func TestApplyInboxProject(t *testing.T) {
	ctx := context.Background()
	s, _ := setupTestChatServer(t)
	projStore := project.NewMemoryStore()
	baseDir := t.TempDir()
	s.projectSvc = project.NewService(projStore, nil, "test-auth-secret", baseDir)

	now := time.Now().UTC()
	err := projStore.CreateProject(ctx, &project.Project{
		ID:            "proj_test1234567890123456789012345678",
		Name:          "Demo Workspace",
		ProjectType:   "blank",
		CreatedAt:     now,
		UpdatedAt:     now,
		DefaultBranch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}

	q := db.InboxRequest{
		RoomID:    "room-test",
		OwnerID:   "owner-1",
		MessageID: "msg-1",
	}

	// 1. Create project
	createContent := "Membuat project baru.\n```cuan-project\n{\"action\":\"create\",\"name\":\"Landing Page\"}\n```"
	createReply := s.applyInboxProject(ctx, q, createContent)
	if !strings.Contains(createReply, "Project Baru Berhasil Dibuat") || !strings.Contains(createReply, "Landing Page") {
		t.Fatalf("unexpected create reply: %s", createReply)
	}

	// 2. Set Env
	envContent := "Konfigurasi token API.\n```cuan-project\n{\"action\":\"set_env\",\"project_id\":\"proj_test1234567890123456789012345678\",\"env_key\":\"PORT\",\"env_value\":\"8080\"}\n```"
	envReply := s.applyInboxProject(ctx, q, envContent)
	if !strings.Contains(envReply, "Environment Variable Diperbarui") || !strings.Contains(envReply, "PORT") {
		t.Fatalf("unexpected env reply: %s", envReply)
	}

	// Verify env stored
	projResp, err := s.projectSvc.Get(ctx, "proj_test1234567890123456789012345678")
	if err != nil {
		t.Fatal(err)
	}
	settings := project.SettingsFromMetadata(projResp.Metadata)
	if !strings.Contains(settings.Environment, "PORT=8080") {
		t.Fatalf("expected PORT=8080 in environment, got: %s", settings.Environment)
	}

	// 3. Archive project
	archiveContent := "Arsipkan project.\n```cuan-project\n{\"action\":\"archive\",\"project_id\":\"proj_test1234567890123456789012345678\"}\n```"
	archiveReply := s.applyInboxProject(ctx, q, archiveContent)
	if !strings.Contains(archiveReply, "Project Diarsipkan") {
		t.Fatalf("unexpected archive reply: %s", archiveReply)
	}

	projArchived, _ := s.projectSvc.Get(ctx, "proj_test1234567890123456789012345678")
	if !projArchived.IsArchived {
		t.Fatal("expected project to be archived")
	}

	// 4. Unarchive project
	unarchiveContent := "Aktifkan kembali.\n```cuan-project\n{\"action\":\"unarchive\",\"project_id\":\"proj_test1234567890123456789012345678\"}\n```"
	unarchiveReply := s.applyInboxProject(ctx, q, unarchiveContent)
	if !strings.Contains(unarchiveReply, "Project Diaktifkan Kembali") {
		t.Fatalf("unexpected unarchive reply: %s", unarchiveReply)
	}

	projRestored, _ := s.projectSvc.Get(ctx, "proj_test1234567890123456789012345678")
	if projRestored.IsArchived {
		t.Fatal("expected project to be unarchived")
	}
}
