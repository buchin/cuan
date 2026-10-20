package project

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cuangit "github.com/buchin/cuan/internal/git"
)

func TestProjectSettingsPreserveTaskWorkspaceAndGuidance(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	root := t.TempDir()
	old := filepath.Join(root, "original")
	os.MkdirAll(old, 0700)
	os.WriteFile(filepath.Join(old, "retained.txt"), []byte("original work"), 0600)
	initial := &Project{ID: "proj_settings", Name: "Original", DefaultBranch: "main", LocalPath: old, Metadata: json.RawMessage(`{"unrelated":{"keep":true}}`), SourceMetadata: json.RawMessage(`{"seed":"preserved"}`), UpdatedAt: time.Now().UTC()}
	if err := store.CreateProject(ctx, initial); err != nil {
		t.Fatal(err)
	}
	service := NewService(store, nil, "fixture-secret", filepath.Join(root, "workspaces"))
	p, _ := service.Get(ctx, initial.ID)
	before := SnapshotProjectGuidance("Approved work", p)
	req := UpdateSettingsRequest{Name: "Renamed", Goal: "Useful outcome", Instructions: "Keep the existing scope", DefaultBranch: "main", ExpectedUpdatedAt: p.UpdatedAt}
	saved, err := service.UpdateSettings(ctx, p.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Name != "Renamed" || !strings.Contains(string(saved.Metadata), "\"unrelated\"") {
		t.Fatal("settings lost unrelated metadata")
	}
	after := SnapshotProjectGuidance("New work", saved)
	revised := PreserveProjectGuidance(before, "Revised approved work")
	if strings.Contains(revised, "Useful outcome") || !strings.Contains(revised, "workspace_revision") {
		t.Fatal("revision lost original guidance")
	}
	if strings.Contains(before, "Useful outcome") || !strings.Contains(after, "Useful outcome") {
		t.Fatal("task guidance not frozen")
	}
	if _, err = service.UpdateSettings(ctx, p.ID, req); !errors.Is(err, ErrSettingsConflict) {
		t.Fatal("stale save accepted", err)
	}
	req.ExpectedUpdatedAt = saved.UpdatedAt
	req.DefaultBranch = "next"
	switched, err := service.UpdateSettings(ctx, p.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	if switched.LocalPath == old {
		t.Fatal("reused old workspace")
	}
	previous, err := service.TaskWorkspace(ctx, p.ID, before)
	if err != nil || previous.LocalPath != old {
		t.Fatal("original task binding lost", err)
	}
	latest, err := service.TaskWorkspace(ctx, p.ID, SnapshotProjectGuidance("New task", switched))
	if err != nil || latest.LocalPath != switched.LocalPath {
		t.Fatal("new task did not use new workspace", err)
	}
	restarted := NewService(store, nil, "fixture-secret", filepath.Join(root, "workspaces"))
	recovered, err := restarted.TaskWorkspace(ctx, p.ID, before)
	if err != nil || recovered.LocalPath != old {
		t.Fatal("recovery lost original workspace", err)
	}
	data, _ := os.ReadFile(filepath.Join(old, "retained.txt"))
	if string(data) != "original work" {
		t.Fatal("old workspace touched")
	}
	if !strings.Contains(string(switched.SourceMetadata), "\"seed\"") {
		t.Fatal("connection lost source metadata")
	}
	// Historical paths must come from authenticated server-owned bindings.
	tampered, _ := store.GetProject(ctx, p.ID)
	var source map[string]json.RawMessage
	json.Unmarshal(tampered.SourceMetadata, &source)
	source["cuan_workspace_versions"] = json.RawMessage(`{"initial":"untrusted path"}`)
	tampered.SourceMetadata, _ = json.Marshal(source)
	store.UpdateProject(ctx, tampered)
	if _, err = restarted.TaskWorkspace(ctx, p.ID, before); err == nil {
		t.Fatal("unverified historical path accepted")
	}
}

func TestProjectSettingsRepositoryChangesAreVerifiedAndIsolated(t *testing.T) {
	ctx := context.Background()
	actualGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	fixture := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command(actualGit, args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatal(string(output), err)
		}
	}
	run("init", "--initial-branch=trunk", fixture)
	os.WriteFile(filepath.Join(fixture, "README.md"), []byte("Fixture repository"), 0600)
	run("-C", fixture, "add", "README.md")
	run("-C", fixture, "commit", "-m", "Create disposable repository fixture")
	// Only clone is redirected to the local fixture. All git operations are real.
	tools := t.TempDir()
	wrapper := "#!/usr/bin/env python3\nimport os,sys\na=sys.argv[1:]\nif a[0]=='clone': a=['clone','--quiet'," + strconvQuote(fixture) + ",a[-1]]\nos.execv(" + strconvQuote(actualGit) + ",['git']+a)\n"
	os.WriteFile(filepath.Join(tools, "git"), []byte(wrapper), 0700)
	t.Setenv("PATH", tools+":"+os.Getenv("PATH"))
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "missing") {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"name":"fixture","full_name":"owner/fixture","default_branch":"trunk","owner":{"login":"owner"}}`))
	}))
	defer api.Close()
	client := cuangit.NewClient()
	client.SetAPIBaseURL(api.URL)
	store := NewMemoryStore()
	p := &Project{ID: "proj_connect", Name: "Original", DefaultBranch: "main", UpdatedAt: time.Now().UTC(), Metadata: json.RawMessage("{}"), SourceMetadata: json.RawMessage("{}")}
	store.CreateProject(ctx, p)
	service := NewService(store, client, "fixture-secret", t.TempDir())
	original, _ := service.Get(ctx, p.ID)
	req := UpdateSettingsRequest{Name: "Connected", RepoURL: "owner/fixture", ExpectedUpdatedAt: original.UpdatedAt}
	connected, err := service.UpdateSettings(ctx, p.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(connected.LocalPath, "README.md")); err != nil || string(data) != "Fixture repository" {
		t.Fatal("repository not prepared")
	}
	if connected.DefaultBranch != "trunk" {
		t.Fatalf("expected GitHub default trunk, got %q", connected.DefaultBranch)
	}
	preserved, err := service.UpdateSettings(ctx, p.ID, UpdateSettingsRequest{Name: "Renamed", RepoURL: "owner/fixture", ExpectedUpdatedAt: connected.UpdatedAt})
	if err != nil || preserved.DefaultBranch != "trunk" || preserved.LocalPath != connected.LocalPath {
		t.Fatal("omitted branch changed existing repository workspace", err)
	}
	connected = preserved
	oldPath := connected.LocalPath
	req.Name = "Must not save"
	req.RepoURL = "owner/missing"
	req.ExpectedUpdatedAt = connected.UpdatedAt
	if _, err = service.UpdateSettings(ctx, p.ID, req); err == nil {
		t.Fatal("inaccessible repository accepted")
	}
	retained, _ := service.Get(ctx, p.ID)
	if retained.Name != "Renamed" || retained.LocalPath != oldPath {
		t.Fatal("failed connection changed saved state")
	}
	req.RepoURL = "owner/fixture"
	req.DefaultBranch = "missing-branch"
	if _, err = service.UpdateSettings(ctx, p.ID, req); err == nil {
		t.Fatal("missing branch accepted")
	}
	retained, _ = service.Get(ctx, p.ID)
	if retained.LocalPath != oldPath {
		t.Fatal("failed branch changed workspace")
	}
}

func strconvQuote(value string) string { raw, _ := json.Marshal(value); return string(raw) }
