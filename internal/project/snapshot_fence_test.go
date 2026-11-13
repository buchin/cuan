package project

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Production stored instructions whose snapshot block ended with an extra
// newline after the closing fence. Parsing then read the fence as part of the
// JSON, every task for that project failed with "Task project context is
// invalid" before an attempt could start, and nothing ran.
func TestSnapshotParsesWithTrailingWhitespaceAfterFence(t *testing.T) {
	base := t.TempDir()
	svc := NewService(NewMemoryStore(), nil, "test-secret", base)
	ctx := context.Background()
	now := time.Now().UTC()
	p := &Project{ID: "proj_11112222333344445555666677778888", Name: "trailing", RepoURL: "https://github.com/example/repo", CreatedAt: now, UpdatedAt: now}
	if err := svc.store.CreateProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	resp, err := svc.Get(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	instructions := SnapshotProjectGuidance("Do the work", resp) + "\n"
	for _, variant := range []string{instructions, instructions + "\n\n", "  " + instructions} {
		got, err := svc.TaskContext(ctx, p.ID, variant)
		if err != nil {
			t.Fatalf("snapshot with trailing whitespace was rejected: %v", err)
		}
		if got == nil {
			t.Fatal("task context returned no project")
		}
	}
}

// A multi-line environment value is normal project configuration and must not
// break snapshot parsing.
func TestSnapshotParsesMultiLineEnvironment(t *testing.T) {
	base := t.TempDir()
	svc := NewService(NewMemoryStore(), nil, "test-secret", base)
	ctx := context.Background()
	now := time.Now().UTC()
	p := &Project{
		ID:        "proj_aaaabbbbccccddddeeeeffff11112222",
		Name:      "env",
		RepoURL:   "https://github.com/example/repo",
		Metadata:  []byte(`{"cuan_settings":{"environment":"A=1\nB=2\nC=3","workspace_revision":"initial"}}`),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := svc.store.CreateProject(ctx, p); err != nil {
		t.Fatal(err)
	}
	resp, err := svc.Get(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	instructions := SnapshotProjectGuidance("Do the work", resp)
	got, err := svc.TaskContext(ctx, p.ID, instructions)
	if err != nil {
		t.Fatalf("multi-line environment broke snapshot parsing: %v", err)
	}
	settings := SettingsFromMetadata(got.Metadata)
	if settings.Environment != "A=1\nB=2\nC=3" {
		t.Fatalf("environment not preserved: %q", settings.Environment)
	}
}

// The unstripped form must keep working so existing saved tasks stay readable.
func TestSnapshotParsesWhenFenceIsLastBytes(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "instructions.txt")
	body := "Project guidance snapshot:\n```cuan-project-context\n{\"project_id\":\"x\"}\n```"
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	start, size := findSnapshotMarker("\n\n" + body)
	if start < 0 {
		t.Fatal("marker not found")
	}
	if got := snapshotBody("\n\n"+body, start, size); got != `{"project_id":"x"}` {
		t.Fatalf("unexpected snapshot body: %q", got)
	}
}
