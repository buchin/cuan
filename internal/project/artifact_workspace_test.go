package project

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStandaloneArtifactWorkspaceDoesNotRequireOrModifyRepository(t *testing.T) {
	svc := NewService(NewMemoryStore(), nil, "", t.TempDir())
	source := t.TempDir()
	sentinel := filepath.Join(source, "existing.txt")
	if err := os.WriteFile(sentinel, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	p := &ProjectResponse{LocalPath: source, RepoURL: "file://" + source}
	scope := "Pekerjaan mandiri: buat berkas hasil.txt. Jangan membuka atau mengubah repositori aplikasi. Jangan deploy."
	first, err := svc.TaskExecutionWorkspace(context.Background(), "first", scope, p)
	if err != nil || first == source {
		t.Fatal("artifact required source checkout", first, err)
	}
	if err := os.WriteFile(filepath.Join(first, "hasil.txt"), []byte("saved result"), 0600); err != nil {
		t.Fatal(err)
	}
	repeated, err := svc.TaskExecutionWorkspace(context.Background(), "first", scope, p)
	if err != nil || first != repeated {
		t.Fatal("lost retained output", err)
	}
	second, err := svc.TaskExecutionWorkspace(context.Background(), "second", scope, p)
	if err != nil || second == first {
		t.Fatal("tasks share artifact workspace", err)
	}
	raw, err := os.ReadFile(sentinel)
	if err != nil || string(raw) != "preserve" {
		t.Fatal("source changed", err)
	}
	if _, err := os.Stat(filepath.Join(first, ".git")); !os.IsNotExist(err) {
		t.Fatal("created fake repository")
	}
}
