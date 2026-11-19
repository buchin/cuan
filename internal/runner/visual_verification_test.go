package runner

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/buchin/cuan/internal/config"
	cuangit "github.com/buchin/cuan/internal/git"
	"github.com/buchin/cuan/internal/task"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// These tiny PNGs exercise transport/integrity only, never application UI proof.
func nativeVisualFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	artifacts := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "scripts"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "component.tsx"), []byte("real component fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 3, 4))); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nset -eu\n[ -z \"${CUAN_RUNNER_TOKEN:-}\" ]\n[ -z \"${CUAN_AUTH_TOKEN:-}\" ]\n[ -d \"$HOME\" ]\nprintf '%s' '" + base64.StdEncoding.EncodeToString(imageData.Bytes()) + "' | base64 -d > \"$1/capture.png\"\nprintf '%s' '{\"components\":[\"component.tsx\"],\"captures\":[\"capture.png\"]}' > \"$1/visual-evidence.json\"\n"
	if err := os.WriteFile(filepath.Join(root, "scripts/verify-ui.sh"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return root, artifacts
}
func TestNativeVisualRecipeAndCaptureIntegrity(t *testing.T) {
	t.Setenv("CUAN_RUNNER_TOKEN", "must-not-be-inherited")
	t.Setenv("CUAN_AUTH_TOKEN", "must-not-be-inherited")
	for _, scenario := range []string{"valid", "changed recipe", "symlink recipe", "existing output", "escaping manifest", "unknown manifest field", "invalid PNG", "missing component", "too many captures", "script failure", "changed output"} {
		t.Run(scenario, func(t *testing.T) {
			root, artifacts := nativeVisualFixture(t)
			recipe, err := VisualRecipeAt(root, strings.Repeat("a", 40))
			if err != nil || recipe == nil {
				t.Fatal(err)
			}
			script := filepath.Join(root, "scripts/verify-ui.sh")
			raw, err := os.ReadFile(script)
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "changed recipe":
				os.WriteFile(script, append(raw, []byte("# Changed recipe\n")...), 0700)
			case "symlink recipe":
				os.Rename(script, script+"-saved")
				os.Symlink(script+"-saved", script)
			case "existing output":
				os.Mkdir(filepath.Join(artifacts, "native-visual"), 0700)
				os.WriteFile(filepath.Join(artifacts, "native-visual/checkpoint"), []byte("PRESERVE"), 0600)
			case "escaping manifest":
				raw = bytes.ReplaceAll(raw, []byte(`"captures":["capture.png"]`), []byte(`"captures":["../capture.png"]`))
			case "unknown manifest field":
				raw = bytes.ReplaceAll(raw, []byte(`"components":[`), []byte(`"untrusted":true,"components":[`))
			case "invalid PNG":
				raw = append(raw, []byte("printf not-a-PNG > \"$1/capture.png\"\n")...)
			case "missing component":
				os.Remove(filepath.Join(root, "component.tsx"))
			case "too many captures":
				captures := make([]string, 33)
				for i := range captures {
					captures[i] = `"capture.png"`
				}
				raw = bytes.ReplaceAll(raw, []byte(`"captures":["capture.png"]`), []byte(`"captures":[`+strings.Join(captures, ",")+`]`))
			case "script failure":
				raw = []byte("#!/bin/sh\nexit 7\n")
			}
			if scenario != "changed recipe" && scenario != "symlink recipe" {
				if err := os.WriteFile(script, raw, 0700); err != nil {
					t.Fatal(err)
				}
				recipe, err = VisualRecipeAt(root, strings.Repeat("a", 40))
				if err != nil {
					t.Fatal(err)
				}
			}
			receipt, err := runNativeVisualVerification(context.Background(), root, artifacts, strings.Repeat("b", 64), recipe, map[string]string{"CUAN_RUNNER_TOKEN": "also-not-forwarded"})
			valid := scenario == "valid" || scenario == "changed output"
			if (err == nil) != valid {
				t.Fatalf("unexpected native capture outcome: %v", err)
			}
			if valid {
				if !receipt.Valid() || receipt.Captures[0].Width != 3 || receipt.Captures[0].Height != 4 || !unchangedNativeVisualArtifacts(artifacts, receipt) {
					t.Fatal("native capture did not retain actual file dimensions/hash")
				}
				if scenario == "changed output" {
					os.WriteFile(filepath.Join(artifacts, receipt.Captures[0].Path), []byte("replaced"), 0600)
					if unchangedNativeVisualArtifacts(artifacts, receipt) {
						t.Fatal("replaced capture accepted")
					}
				}
			}
			if scenario == "existing output" {
				data, _ := os.ReadFile(filepath.Join(artifacts, "native-visual/checkpoint"))
				if string(data) != "PRESERVE" {
					t.Fatal("existing output was replaced")
				}
			}
		})
	}
}

// Opt-in real-project QA. This is not a task retry or task completion receipt.
func TestNativeVisualRegisteredProjectRecipe(t *testing.T) {
	root := os.Getenv("CUAN_REAL_VISUAL_PROJECT")
	if root == "" || testing.Short() {
		t.Skip("set CUAN_REAL_VISUAL_PROJECT to exercise the real project browser recipe")
	}
	raw, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	commit := strings.TrimSpace(string(raw))
	if err := cuangit.NewClient().VerifySnapshotCommit(context.Background(), root, commit, commit); err != nil {
		t.Fatal("real-project QA requires clean committed source", err)
	}
	baseline, err := repositoryFingerprint(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	recipe, err := VisualRecipeAt(root, commit)
	if err != nil || recipe == nil {
		t.Fatal("project recipe unavailable", err)
	}
	artifacts := os.Getenv("CUAN_REAL_VISUAL_OUTPUT")
	if artifacts == "" {
		artifacts = t.TempDir()
	} else if err := os.Mkdir(artifacts, 0700); err != nil {
		t.Fatal("use a new private artifact directory", err)
	}
	environment := map[string]string{}
	for _, key := range []string{"CUAN_BROWSER_MODULE", "CUAN_CHROMIUM_EXECUTABLE", "CUAN_BROWSER_NO_SANDBOX", "PLAYWRIGHT_BROWSERS_PATH"} {
		environment[key] = os.Getenv(key)
	}
	receipt, err := runNativeVisualVerification(context.Background(), root, artifacts, baseline, recipe, environment)
	if err != nil {
		t.Fatal(err)
	}
	final, err := repositoryFingerprint(context.Background(), root)
	if err != nil || final != baseline || !unchangedNativeVisualArtifacts(artifacts, receipt) {
		t.Fatal("real visual check changed source or capture files", err)
	}
	data, _ := json.MarshalIndent(receipt, "", "  ")
	if err := os.WriteFile(filepath.Join(artifacts, "verification.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	t.Logf("Real-component captures checked: %d; source=%s; output=%s", len(receipt.Captures), commit, artifacts)
}

func TestNativeVisualDeliveryReconcilesSavedReceiptWithoutResending(t *testing.T) {
	for _, scenario := range []string{"recorded response lost", "unrecorded", "wrong attempt", "different receipt", "forbidden"} {
		t.Run(scenario, func(t *testing.T) {
			root, artifacts := nativeVisualFixture(t)
			recipe, err := VisualRecipeAt(root, strings.Repeat("a", 40))
			if err != nil {
				t.Fatal(err)
			}
			receipt, err := runNativeVisualVerification(context.Background(), root, artifacts, strings.Repeat("b", 64), recipe, nil)
			if err != nil {
				t.Fatal(err)
			}
			var posts, gets atomic.Int32
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Runner-Token") != "fixture-runner-token" {
					t.Error("native evidence did not use runner transport")
				}
				if r.Method == "POST" {
					posts.Add(1)
					if scenario == "forbidden" {
						w.WriteHeader(403)
						return
					}
					w.WriteHeader(503)
					return
				}
				gets.Add(1)
				if r.URL.Query().Get("task_id") != "task" || r.URL.Query().Get("runner_id") != "runner" {
					t.Error("reconciliation read another execution")
				}
				id := "attempt"
				if scenario == "wrong attempt" {
					id = "other"
				}
				saved := receipt
				if scenario == "unrecorded" {
					saved = nil
				}
				if scenario == "different receipt" {
					copy := *receipt
					copy.Baseline = strings.Repeat("c", 64)
					saved = &copy
				}
				json.NewEncoder(w).Encode(map[string]any{"task_id": "task", "attempt_id": id, "native_visual_verification": saved})
			}))
			defer api.Close()
			r := NewRunner(&config.Config{RunnerID: "runner", RunnerServerURL: api.URL, RunnerToken: "fixture-runner-token"}, nil)
			r.httpClient = api.Client()
			claim := ClaimTaskResponse{Task: &task.Task{ID: "task"}, Attempt: &task.TaskAttempt{ID: "attempt"}, TaskAgent: &TaskAgentCapability{ArtifactsDir: artifacts}}
			err = r.publishNativeVisualArtifacts(context.Background(), claim, receipt)
			if (err == nil) != (scenario == "recorded response lost") || posts.Load() != 1 {
				t.Fatal("ambiguous native upload was replayed or falsely confirmed", err, posts.Load())
			}
			if scenario == "forbidden" && gets.Load() != 0 {
				t.Fatal("reconciled denied native upload")
			}
		})
	}
}
