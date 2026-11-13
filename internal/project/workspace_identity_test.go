package project

import (
	"context"
	"os"
	"path/filepath"
	"time"
	"testing"
)

// A project must never adopt another project's directory. Production hit this:
// "cuan" stored a path pointing at the archived "default" workspace, so every
// task failed with "project unavailable" even though a directory existed. The
// fallback derived candidates from the basename of the stored path, which let a
// project inherit whichever workspace happened to sit at that name.
func TestEnsureValidLocalPathNeverAdoptsAnotherProjectDirectory(t *testing.T) {
	base := t.TempDir()
	foreign := filepath.Join(base, "default")
	if err := os.MkdirAll(foreign, 0o755); err != nil {
		t.Fatal(err)
	}

	svc := NewService(NewMemoryStore(), nil, "test-secret", base)
	ctx := context.Background()
	now := time.Now().UTC()
	p := &Project{
		ID:        "proj_0123456789abcdef0123456789abcdef",
		Name:      "cuan",
		LocalPath: foreign,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := svc.store.CreateProject(ctx, p); err != nil {
		t.Fatal(err)
	}

	got, err := svc.Get(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got.LocalPath) == filepath.Clean(foreign) {
		t.Fatalf("project adopted another project's directory: %s", got.LocalPath)
	}
	if want := filepath.Join(base, p.ID); filepath.Clean(got.LocalPath) != filepath.Clean(want) {
		t.Fatalf("expected own workspace %s, got %s", want, got.LocalPath)
	}
}

// A project whose own workspace already exists must keep using it.
func TestEnsureValidLocalPathKeepsOwnWorkspace(t *testing.T) {
	base := t.TempDir()
	svc := NewService(NewMemoryStore(), nil, "test-secret", base)
	ctx := context.Background()
	now := time.Now().UTC()
	p := &Project{ID: "proj_aaaabbbbccccddddeeeeffff00001111", Name: "own", CreatedAt: now, UpdatedAt: now}
	p.LocalPath = filepath.Join(base, p.ID)
	if err := os.MkdirAll(p.LocalPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := svc.store.CreateProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got.LocalPath) != filepath.Clean(p.LocalPath) {
		t.Fatalf("own workspace was replaced: %s", got.LocalPath)
	}
}
