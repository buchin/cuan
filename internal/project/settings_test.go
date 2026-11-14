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
	initial := &Project{ID: "proj_settings", Name: "Original", DefaultBranch: "main", LocalPath: old, Metadata: json.RawMessage(`{"unrelated":{"keep":true},"cuan_settings":{"auto_deploy":false,"environment":"TARGET=original","deploy_branch":"original-release"}}`), SourceMetadata: json.RawMessage(`{"seed":"preserved"}`), UpdatedAt: time.Now().UTC()}
	if err := store.CreateProject(ctx, initial); err != nil {
		t.Fatal(err)
	}
	service := NewService(store, nil, "fixture-secret", filepath.Join(root, "workspaces"))
	p, _ := service.Get(ctx, initial.ID)
	before := SnapshotProjectGuidance("Approved work", p)
	enabled := true
	req := UpdateSettingsRequest{Name: "Renamed", Goal: "Useful outcome", Instructions: "Keep the existing scope", AutoDeploy: &enabled, Environment: "TARGET=latest", DeployBranch: "latest-release", DefaultBranch: "main", ExpectedUpdatedAt: p.UpdatedAt}
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
	originalContext, err := service.TaskContext(ctx, p.ID, before)
	if err != nil {
		t.Fatal(err)
	}
	newContext, err := service.TaskContext(ctx, p.ID, SnapshotProjectGuidance("New task", switched))
	if err != nil {
		t.Fatal(err)
	}
	oldSettings, newSettings := SettingsFromMetadata(originalContext.Metadata), SettingsFromMetadata(newContext.Metadata)
	if originalContext.LocalPath != old || oldSettings.Environment != "TARGET=original" || oldSettings.IsAutoDeployEnabled() || oldSettings.DeployBranch != "original-release" || oldSettings.Goal != "" {
		t.Fatal("old task drifted to current project context")
	}
	if newContext.LocalPath != switched.LocalPath || newSettings.Environment != "TARGET=latest" || !newSettings.IsAutoDeployEnabled() || newSettings.Goal != "Useful outcome" || newSettings.DeployBranch != "latest-release" {
		t.Fatal("new task lost updated settings")
	}
	if !strings.Contains(string(originalContext.Metadata), `"unrelated"`) {
		t.Fatal("task context discarded unrelated metadata")
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
	if err := os.Rename(old, old+"-retained"); err != nil {
		t.Fatal(err)
	}
	if retained, err := service.TaskContext(ctx, p.ID, before); err != nil || retained.LocalPath != old {
		t.Fatal("missing historical workspace fell back to a different branch", err)
	}
	if _, err := service.TaskContext(ctx, p.ID, SnapshotProjectGuidance("New task", switched)); err != nil {
		t.Fatal("missing old workspace broke the current project", err)
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
	wrapper := "#!/usr/bin/env python3\nimport os,sys\na=sys.argv[1:]\nassert not any('shared-fixture' in value for value in a)\nif a[0]=='clone': assert os.environ.get('GIT_CONFIG_KEY_0')=='http.https://github.com/.extraheader'\nif a[0]=='clone': a=['clone','--quiet'," + strconvQuote(fixture) + ",a[-1]]\nos.execv(" + strconvQuote(actualGit) + ",['git']+a)\n"
	os.WriteFile(filepath.Join(tools, "git"), []byte(wrapper), 0700)
	t.Setenv("PATH", tools+":"+os.Getenv("PATH"))
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "missing") {
			w.WriteHeader(404)
			return
		}
		if r.Header.Get("Authorization") != "Bearer shared-fixture" {
			t.Error("shared GitHub access missing")
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
	service.SetSharedRepositoryAccess(func(context.Context) (string, error) { return "shared-fixture", nil })
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
	savedConnection, _ := store.GetProject(ctx, p.ID)
	if savedConnection.EncryptedToken != "" {
		t.Fatal("shared credential copied into project")
	}
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

func TestProjectSettingsAutoDeploy(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	initial := &Project{ID: "proj_autodeploy", Name: "AutoDeploy Test", DefaultBranch: "main", Metadata: json.RawMessage(`{}`), UpdatedAt: time.Now().UTC()}
	if err := store.CreateProject(ctx, initial); err != nil {
		t.Fatal(err)
	}
	service := NewService(store, nil, "fixture-secret", t.TempDir())
	p, _ := service.Get(ctx, initial.ID)
	settings := SettingsFromMetadata(p.Metadata)
	if !settings.IsAutoDeployEnabled() {
		t.Fatal("expected auto_deploy to default to true")
	}

	falseVal := false
	req := UpdateSettingsRequest{Name: "AutoDeploy Test", AutoDeploy: &falseVal, ExpectedUpdatedAt: p.UpdatedAt}
	saved, err := service.UpdateSettings(ctx, p.ID, req)
	if err != nil {
		t.Fatal(err)
	}
	savedSettings := SettingsFromMetadata(saved.Metadata)
	if savedSettings.IsAutoDeployEnabled() {
		t.Fatal("expected auto_deploy to be false after explicit update")
	}

	trueVal := true
	req2 := UpdateSettingsRequest{Name: "AutoDeploy Test", AutoDeploy: &trueVal, ExpectedUpdatedAt: saved.UpdatedAt}
	saved2, err := service.UpdateSettings(ctx, p.ID, req2)
	if err != nil {
		t.Fatal(err)
	}
	savedSettings2 := SettingsFromMetadata(saved2.Metadata)
	if !savedSettings2.IsAutoDeployEnabled() {
		t.Fatal("expected auto_deploy to be true after update")
	}
}

func TestProjectSettingsDeployBranch(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	initial := &Project{ID: "proj_deploy_branch", Name: "DeployBranch Test", DefaultBranch: "dev", Metadata: json.RawMessage(`{}`), UpdatedAt: time.Now().UTC()}
	if err := store.CreateProject(ctx, initial); err != nil {
		t.Fatal(err)
	}
	service := NewService(store, nil, "fixture-secret", t.TempDir())
	p, _ := service.Get(ctx, initial.ID)

	// Update with custom deploy_branch
	req := UpdateSettingsRequest{
		Name:              "DeployBranch Test",
		DefaultBranch:     "dev",
		DeployBranch:      "main",
		ExpectedUpdatedAt: p.UpdatedAt,
	}
	saved, err := service.UpdateSettings(ctx, p.ID, req)
	if err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}
	settings := SettingsFromMetadata(saved.Metadata)
	if settings.DeployBranch != "main" {
		t.Fatalf("expected deploy_branch 'main', got %q", settings.DeployBranch)
	}

	// Update with invalid deploy branch name
	badReq := UpdateSettingsRequest{
		Name:              "DeployBranch Test",
		DeployBranch:      "-invalid-branch-name",
		ExpectedUpdatedAt: saved.UpdatedAt,
	}
	if _, err := service.UpdateSettings(ctx, p.ID, badReq); err == nil {
		t.Fatal("expected error for invalid branch name, got nil")
	}
}

func TestProjectSettingsDeployScript(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	initial := &Project{ID: "proj_deploy_script", Name: "DeployScript Test", DefaultBranch: "main", Metadata: json.RawMessage(`{}`), UpdatedAt: time.Now().UTC()}
	if err := store.CreateProject(ctx, initial); err != nil {
		t.Fatal(err)
	}
	service := NewService(store, nil, "fixture-secret", t.TempDir())
	p, _ := service.Get(ctx, initial.ID)

	// Update with custom deploy_script
	req := UpdateSettingsRequest{
		Name:              "DeployScript Test",
		DefaultBranch:     "main",
		DeployScript:      "bash scripts/release.sh",
		ExpectedUpdatedAt: p.UpdatedAt,
	}
	saved, err := service.UpdateSettings(ctx, p.ID, req)
	if err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}
	settings := SettingsFromMetadata(saved.Metadata)
	if settings.DeployScript != "bash scripts/release.sh" {
		t.Fatalf("expected deploy_script 'bash scripts/release.sh', got %q", settings.DeployScript)
	}

	// Update with overly long deploy script (>10,000 chars)
	longScript := strings.Repeat("echo 'a'\n", 1500)
	badReq := UpdateSettingsRequest{
		Name:              "DeployScript Test",
		DefaultBranch:     "main",
		DeployScript:      longScript,
		ExpectedUpdatedAt: saved.UpdatedAt,
	}
	if _, err := service.UpdateSettings(ctx, p.ID, badReq); err == nil {
		t.Fatal("expected error for overly long deploy script, got nil")
	}
}

func TestParseEnvironment(t *testing.T) {
	raw := `
# Production environment configuration
APP_NAME=Dojo
APP_ENV="production"
APP_DEBUG=false
APP_URL='https://dojo.cc'

DOJO_VPS_PUBLIC_IP=43.157.241.102
DOJO_VPS_PASSWORD="falcon-27%-panda"
`
	parsed := ParseEnvironment(raw)
	if parsed["APP_NAME"] != "Dojo" {
		t.Errorf("expected APP_NAME Dojo, got %q", parsed["APP_NAME"])
	}
	if parsed["APP_ENV"] != "production" {
		t.Errorf("expected APP_ENV production, got %q", parsed["APP_ENV"])
	}
	if parsed["APP_DEBUG"] != "false" {
		t.Errorf("expected APP_DEBUG false, got %q", parsed["APP_DEBUG"])
	}
	if parsed["APP_URL"] != "https://dojo.cc" {
		t.Errorf("expected APP_URL https://dojo.cc, got %q", parsed["APP_URL"])
	}
	if parsed["DOJO_VPS_PUBLIC_IP"] != "43.157.241.102" {
		t.Errorf("expected DOJO_VPS_PUBLIC_IP 43.157.241.102, got %q", parsed["DOJO_VPS_PUBLIC_IP"])
	}
	if parsed["DOJO_VPS_PASSWORD"] != "falcon-27%-panda" {
		t.Errorf("expected DOJO_VPS_PASSWORD falcon-27%%-panda, got %q", parsed["DOJO_VPS_PASSWORD"])
	}
}

func TestProjectSettingsEnvironment(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	initial := &Project{ID: "proj_env_test", Name: "Env Test", DefaultBranch: "main", Metadata: json.RawMessage(`{}`), UpdatedAt: time.Now().UTC()}
	if err := store.CreateProject(ctx, initial); err != nil {
		t.Fatal(err)
	}
	service := NewService(store, nil, "fixture-secret", t.TempDir())
	p, _ := service.Get(ctx, initial.ID)

	envContent := "DOJO_VPS_PUBLIC_IP=43.157.241.102\nDOJO_VPS_PASSWORD=secret"
	req := UpdateSettingsRequest{
		Name:              "Env Test",
		Environment:       envContent,
		ExpectedUpdatedAt: p.UpdatedAt,
	}
	saved, err := service.UpdateSettings(ctx, p.ID, req)
	if err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}
	settings := SettingsFromMetadata(saved.Metadata)
	if settings.Environment != envContent {
		t.Fatalf("expected environment %q, got %q", envContent, settings.Environment)
	}

	snapshot := SnapshotProjectGuidance("Do work", saved)
	if !strings.Contains(snapshot, "DOJO_VPS_PUBLIC_IP") {
		t.Fatalf("expected snapshot to include environment, got %s", snapshot)
	}
}
