package project

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestManagedWorkspaceCreationAndReuse(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	store := NewMemoryStore()
	svc := NewService(store, nil, "", base)
	id := "proj_0123456789abcdef0123456789abcdef"
	p, err := svc.CreateManagedWorkspace(ctx, id, "Website")
	if err != nil {
		t.Fatal(err)
	}
	if p.LocalPath != filepath.Join(base, id) || p.RepoURL != "" {
		t.Fatal(p)
	}
	if info, err := os.Stat(p.LocalPath); err != nil || !info.IsDir() {
		t.Fatal(err)
	}
	reused, err := svc.CreateManagedWorkspace(ctx, "proj_1123456789abcdef0123456789abcdef", "website")
	if err != nil || reused.ID != id {
		t.Fatal(reused, err)
	}
	if _, err := svc.CreateManagedWorkspace(ctx, id, "Different"); err == nil {
		t.Fatal("conflicting retry accepted")
	}
	if _, err := svc.CreateManagedWorkspace(ctx, "../escape", "Bad"); err == nil {
		t.Fatal("invalid ID accepted")
	}
	projects, _ := svc.List(ctx)
	if len(projects) != 1 {
		t.Fatal(projects)
	}
	p0, _ := store.GetProject(ctx, id)
	p0.IsArchived = true
	if err := store.UpdateProject(ctx, p0); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateManagedWorkspace(ctx, id, "Website"); err == nil {
		t.Fatal("archived workspace reused")
	}
}
