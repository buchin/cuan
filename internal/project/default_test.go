package project

import (
	"context"
	"testing"
)

func TestEnsureDefaultCreatesAndPreservesConfiguration(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	svc := NewService(store, nil, "", t.TempDir())
	created, err := svc.EnsureDefault(ctx)
	if err != nil || created.ID != "default" || created.Name != "Default" {
		t.Fatal(created, err)
	}
	p, err := store.GetProject(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	p.Name = "Existing project"
	p.RepoURL = "https://example.invalid/existing"
	p.LocalPath = "/existing/workspace"
	p.IsArchived = true
	if err = store.UpdateProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	existing, err := svc.EnsureDefault(ctx)
	if err != nil || existing.Name != p.Name || existing.RepoURL != p.RepoURL || existing.LocalPath != p.LocalPath || !existing.IsArchived {
		t.Fatal("default configuration changed", existing, err)
	}
	projects, err := svc.List(ctx)
	if err != nil || len(projects) != 1 {
		t.Fatal("duplicate Default projects", projects, err)
	}
}

func TestDefaultSettingsKeepNeutralIdentity(t *testing.T) {
	ctx := context.Background()
	svc := NewService(NewMemoryStore(), nil, "", t.TempDir())
	p, err := svc.EnsureDefault(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []UpdateSettingsRequest{{Name: "Cuan", ExpectedUpdatedAt: p.UpdatedAt}, {Name: "Default", RepoURL: "https://example.invalid/repo", ExpectedUpdatedAt: p.UpdatedAt}} {
		if _, err = svc.UpdateSettings(ctx, p.ID, r); err == nil {
			t.Fatal("Default accepted a project-specific identity")
		}
	}
	got, err := svc.UpdateSettings(ctx, p.ID, UpdateSettingsRequest{Name: "Default", Goal: "General work", ExpectedUpdatedAt: p.UpdatedAt})
	if err != nil || got.Name != "Default" || got.RepoURL != "" {
		t.Fatal(got, err)
	}
}
