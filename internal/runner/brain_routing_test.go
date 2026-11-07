package runner

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/task"
)

func TestRunnerBrainQuotaIsNotUserCancellation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bin := t.TempDir()
	script := "#!/bin/sh\necho 'Individual quota reached. Resets in 2h0m0s.'\nsleep 2\n"
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	adapter, err := agent.NewAGYAdapter(filepath.Join(bin, "agy"))
	if err != nil {
		t.Fatal(err)
	}
	r := NewRunner(&config.Config{RunnerID: "test", RunnerServerURL: server.URL}, adapter, server.Client())
	claim := ClaimTaskResponse{Task: &task.Task{ID: "brain-quota-task"}, Attempt: &task.TaskAttempt{ID: "brain-quota-attempt"}}
	var count int64
	result, err := r.executeWithPTY(context.Background(), claim, claim.Task, claim.Attempt, "test", t.TempDir(), map[string]string{}, 3*time.Second, &count)
	if err != nil || result.Status != "QUOTA_LIMITED" || result.ExitCode != 75 {
		t.Fatalf("quota misclassified: %+v %v", result, err)
	}
	if atomic.LoadInt64(&count) == 0 {
		t.Fatal("missing quota evidence")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err = r.executeWithBrains(ctx, ClaimTaskResponse{OwnerID: "owner"}, claim.Task, claim.Attempt, "test", home, nil, time.Second, &count)
	if err == nil && result != nil && result.Status == "SUCCESS" {
		t.Fatal("cancelled execution continued")
	}
}

func TestRunnerBrainFailoverPreservesTaskAndWork(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for i := 0; i < 2; i++ {
		p, err := agent.CreateProviderAccount("owner", "brain")
		if err != nil {
			t.Fatal(err)
		}
		home, _ := agent.ProviderAccountHome("owner", p.ID)
		_ = os.MkdirAll(agent.AntigravityConfigDirPath(home), 0700)
		_ = os.WriteFile(agent.AntigravityCredentialsFilePath(home), []byte("fixture-credentials-only"), 0600)
		metaPath := filepath.Join(agent.ConfigHome(), ".local", "share", "cuan", "accounts", p.ID, "account.json")
		raw, _ := os.ReadFile(metaPath)
		var meta map[string]any
		_ = json.Unmarshal(raw, &meta)
		meta["state"] = "unknown"
		raw, _ = json.Marshal(meta)
		_ = os.WriteFile(metaPath, raw, 0600)
	}
	bin := t.TempDir()
	script := `#!/bin/sh
if [ ! -f "$WORKSPACE_DIR/progress" ]; then
  echo kept > "$WORKSPACE_DIR/progress"
  echo 'Individual quota reached. Resets in 2h0m0s.'
  sleep 2
else
  case "$*" in *'Continue the SAME approved task'*) echo resumed >> "$WORKSPACE_DIR/progress";; *) exit 9;; esac
  sleep 0.1
  exit 0
fi
`
	_ = os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700)
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{}`)) }))
	defer server.Close()
	adapter, _ := agent.NewAGYAdapter(filepath.Join(bin, "agy"))
	r := NewRunner(&config.Config{RunnerID: "test", RunnerServerURL: server.URL}, adapter, server.Client())
	work := t.TempDir()
	claim := ClaimTaskResponse{OwnerID: "owner", Task: &task.Task{ID: "brain-failover-task"}, Attempt: &task.TaskAttempt{ID: "brain-failover-attempt"}}
	var count int64
	result, err := r.executeWithBrains(context.Background(), claim, claim.Task, claim.Attempt, "original approved scope", work, map[string]string{}, 5*time.Second, &count)
	if err != nil || result.Status != "SUCCESS" {
		t.Fatalf("failover failed: %+v %v", result, err)
	}
	raw, _ := os.ReadFile(filepath.Join(work, "progress"))
	if string(raw) != "kept\nresumed\n" {
		t.Fatal("existing work lost or scope replayed", string(raw))
	}
	profiles, _ := agent.ListProviderAccounts("owner")
	limited := 0
	for _, p := range profiles {
		if p.State == "quota_limited" {
			t.Fatal("one route's quota disabled every model on its account")
		}
		if time.Now().Before(p.ModelCooldowns[""]) {
			limited++
			if time.Until(p.ModelCooldowns[""]) < 119*time.Minute {
				t.Fatal("provider reset deadline was not preserved")
			}
		}
	}
	if limited != 1 {
		t.Fatal("failed brain was not cooled down")
	}
}

func TestRunnerQuotaFallsBackToGeminiOnSameAccount(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	p, err := agent.CreateProviderAccount("owner", "Single brain")
	if err != nil {
		t.Fatal(err)
	}
	home, _ := agent.ProviderAccountHome("owner", p.ID)
	if err := os.MkdirAll(agent.AntigravityConfigDirPath(home), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agent.AntigravityCredentialsFilePath(home), []byte("fixture-only-credentials"), 0600); err != nil {
		t.Fatal(err)
	}
	metaPath := filepath.Join(agent.ConfigHome(), ".local", "share", "cuan", "accounts", p.ID, "account.json")
	raw, _ := os.ReadFile(metaPath)
	var meta map[string]any
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatal(err)
	}
	meta["state"], meta["model_id"] = "unknown", "claude-sonnet-test"
	raw, _ = json.Marshal(meta)
	if err := os.WriteFile(metaPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := agent.SetProviderAccountFallbackModel("owner", p.ID, "gemini-test"); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := `#!/bin/sh
if [ "$1" = models ]; then printf 'claude-sonnet-test\ngemini-test\n'; exit 0; fi
printf '%s\n' "$CUAN_BRAIN_MODEL" >> "$WORKSPACE_DIR/models-used"
if [ "$CUAN_BRAIN_MODEL" = claude-sonnet-test ]; then
 echo kept > "$WORKSPACE_DIR/progress"
 echo 'Individual quota reached. Resets in 2h0m0s.'
 sleep 2
elif [ "$CUAN_BRAIN_MODEL" = gemini-test ]; then
 [ "$(cat "$WORKSPACE_DIR/progress")" = kept ] || exit 22
 case "$*" in *'Continue the SAME approved task'*) echo resumed >> "$WORKSPACE_DIR/progress";; *) exit 23;; esac
 sleep 0.1
 exit 0
else exit 24
fi
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{}`)) }))
	defer api.Close()
	adapter, err := agent.NewAGYAdapter(filepath.Join(bin, "agy"))
	if err != nil {
		t.Fatal(err)
	}
	r := NewRunner(&config.Config{RunnerID: "fallback-fixture", RunnerServerURL: api.URL}, adapter, api.Client())
	work := t.TempDir()
	claim := ClaimTaskResponse{OwnerID: "owner", Task: &task.Task{ID: "same-account-fallback-task"}, Attempt: &task.TaskAttempt{ID: "same-account-fallback-attempt"}}
	var count int64
	result, err := r.executeWithBrains(context.Background(), claim, claim.Task, claim.Attempt, "Original approved scope", work, map[string]string{}, 8*time.Second, &count)
	if err != nil || result == nil || result.Status != "SUCCESS" {
		t.Fatal("same-account model fallback failed", result, err)
	}
	used, _ := os.ReadFile(filepath.Join(work, "models-used"))
	progress, _ := os.ReadFile(filepath.Join(work, "progress"))
	if string(used) != "claude-sonnet-test\ngemini-test\n" || string(progress) != "kept\nresumed\n" {
		t.Fatal("model order or retained work changed", string(used), string(progress))
	}
	saved, err := agent.GetProviderAccount("owner", p.ID)
	if err != nil || saved.ModelID != "claude-sonnet-test" || !time.Now().Before(saved.ModelCooldowns["claude-sonnet-test"]) || time.Now().Before(saved.ModelCooldowns["gemini-test"]) {
		t.Fatal("quota or primary preference crossed models", saved, err)
	}
}
