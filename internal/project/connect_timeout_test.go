package project

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type persistBeforeCloneStore struct {
	*MemoryStore
	marker string
}

func (s *persistBeforeCloneStore) CreateProject(ctx context.Context, p *Project) error {
	if err := s.MemoryStore.CreateProject(ctx, p); err != nil {
		return err
	}
	return os.WriteFile(s.marker, []byte("saved"), 0600)
}

func TestConnectPersistsProjectBeforeCloneTimeout(t *testing.T) {
	_, client := setupMockGitHub(t)
	dir := t.TempDir()
	marker := filepath.Join(dir, "saved")
	bin := t.TempDir()
	script := "#!/bin/sh\nif [ ! -f '" + marker + "' ]; then echo 'project not saved' >&2; exit 8; fi\nexec sleep 10\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	store := &persistBeforeCloneStore{NewMemoryStore(), marker}
	service := NewService(store, client, "fixture-secret", dir)
	// The clone must leave five seconds to save its result within the request.
	ctx, cancel := context.WithTimeout(context.Background(), 5100*time.Millisecond)
	defer cancel()
	result, err := service.Connect(ctx, ConnectProjectRequest{Name: "Slow project", RepoURL: "https://github.com/buchin/cuan", Token: "fixture-token"})
	if err != nil {
		t.Fatal("slow clone lost project", err)
	}
	if result.CloneStatus != "failed" || result.LastError != "Repository download took too long. Your project is saved. Retry synchronization to continue." {
		t.Fatalf("unexpected timeout result: %+v", result)
	}
	saved, err := store.GetProject(ctx, result.ID)
	if err != nil || saved.ID == "" || saved.CloneStatus != "failed" {
		t.Fatal("project not retained", err)
	}
	if ctx.Err() != nil {
		t.Fatal("clone consumed persistence time budget")
	}
}

func TestConnectWithExpiredBudgetSavesProjectGracefully(t *testing.T) {
	_, client := setupMockGitHub(t)
	dir := t.TempDir()
	store := NewMemoryStore()
	service := NewService(store, client, "fixture-secret", dir)

	// Context with very small remaining budget (< 5s)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	result, err := service.Connect(ctx, ConnectProjectRequest{Name: "Tight deadline project", RepoURL: "https://github.com/buchin/cuan", Token: "fixture-token"})
	if err != nil {
		t.Fatal("connect failed despite project persistence guarantee:", err)
	}
	if result.CloneStatus != "failed" {
		t.Fatalf("expected clone_status failed, got %q", result.CloneStatus)
	}
	saved, err := store.GetProject(context.Background(), result.ID)
	if err != nil || saved == nil {
		t.Fatal("expected project to be persisted in store")
	}
}
