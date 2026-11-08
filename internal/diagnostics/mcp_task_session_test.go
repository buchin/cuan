package diagnostics

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/taskcli"
)

func TestTaskSessionMCPUsesOnlyBoundCapability(t *testing.T) {
	for _, format := range []string{"file", "inline"} {
		t.Run(format, func(t *testing.T) {
			t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
			t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")
			requests := 0
			reject := false
			var payload map[string]any
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Header.Get("Authorization") != "Bearer bound-capability" || !strings.HasPrefix(r.URL.Path, "/task/bound") {
					t.Error("escaped task identity")
				}
				if r.Method == http.MethodPost && r.URL.Path == "/task/bound" {
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
					}
				}
				w.Header().Set("Content-Type", "application/json")
				if reject {
					w.WriteHeader(http.StatusConflict)
					w.Write([]byte(`{"error":"attempt ended"}`))
					return
				}
				if r.Method == http.MethodGet {
					w.Write([]byte(`{"id":"bound","latest_attempt":{"id":"attempt"},"dod_checklist":[{"id":"criterion","text":"Original criterion","completed":true,"evidence":"Old claim"}]}`))
					return
				}
				w.Write([]byte(`{"task_id":"bound","accepted_report":true}`))
			}))
			defer api.Close()
			raw, _ := json.Marshal(taskcli.Capability{API: api.URL + "/task/bound", Token: "bound-capability", TaskID: "bound", AttemptID: "attempt"})
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)
			if format == "inline" {
				t.Setenv("CUAN_TASK_CAPABILITY_JSON", string(raw))
			} else {
				file := filepath.Join(t.TempDir(), "cap.json")
				if err := os.WriteFile(file, raw, 0600); err != nil {
					t.Fatal(err)
				}
				t.Setenv("CUAN_TASK_CAPABILITY_FILE", file)
			}
			ownerRequests := 0
			owner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ownerRequests++ }))
			defer owner.Close()
			old := globalCuanClient
			globalCuanClient = &CuanClient{BaseURL: owner.URL, AuthToken: "ambient-owner", HTTPClient: owner.Client()}
			t.Cleanup(func() { globalCuanClient = old })
			if _, err := callTaskSessionTool("get_current_task", nil); err != nil {
				t.Fatal(err)
			}
			template, err := callTaskSessionTool("prepare_current_task_report", nil)
			if err != nil {
				t.Fatal(err)
			}
			items := template.(map[string]any)["dod_checklist"].([]any)
			item := items[0].(map[string]any)
			if item["id"] != "criterion" || item["completed"] != false || item["evidence"] != "" {
				t.Fatal("template inherited success", template)
			}
			report := map[string]any{"state": "review", "summary": "Actual result", "complete_note": "Observed checks", "artifacts": []string{"output.md"}, "dod_checklist": []any{map[string]any{"id": "criterion", "text": "Original criterion", "completed": true, "evidence": "Observed output"}}}
			if _, err := callTaskSessionTool("report_current_task", report); err != nil {
				t.Fatal(err)
			}
			if payload["state"] != "review" || payload["summary"] != "Actual result" || payload["dod_checklist"] == nil || payload["artifacts"] == nil {
				t.Fatalf("report lost evidence: %+v", payload)
			}
			if _, err := callTaskSessionTool("push_current_task", nil); err != nil {
				t.Fatal(err)
			}
			if requests != 4 || ownerRequests != 0 {
				t.Fatalf("wrong identity or duplicate calls: bound=%d owner=%d", requests, ownerRequests)
			}
			for _, args := range []map[string]any{{"task_id": "foreign"}, {"api": "https://foreign.invalid"}, {"token": "foreign"}, {"instructions": "replace scope"}, {"state": "accepted"}, {"dod_checklist": []any{map[string]any{"instructions": "replace"}}}} {
				if _, err := callTaskSessionTool("report_current_task", args); err == nil {
					t.Fatal("unsafe report accepted")
				}
			}
			if _, err := callTaskSessionTool("get_current_task", map[string]any{"task_id": "foreign"}); err == nil {
				t.Fatal("cross-task read accepted")
			}
			if _, err := callTaskSessionTool("prepare_current_task_report", map[string]any{"task_id": "foreign"}); err == nil {
				t.Fatal("template accepted cross-task identity")
			}
			if requests != 4 {
				t.Fatal("invalid arguments caused network effects")
			}
			reject = true
			if _, err := callTaskSessionTool("report_current_task", map[string]any{"progress": "late"}); err == nil || !strings.Contains(err.Error(), "409") {
				t.Fatalf("rejected report claimed success: %v", err)
			}
			files, err := os.ReadDir(tmp)
			if err != nil || len(files) != 0 {
				t.Fatal("private report files retained", err)
			}
			var out, errOut bytes.Buffer
			request := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"get_current_task","arguments":{}}}` + "\n"
			RunMCP(strings.NewReader(request), &out, &errOut)
			if !strings.Contains(out.String(), `"isError":true`) || strings.Contains(out.String(), "bound-capability") {
				t.Fatal("MCP response lied or exposed credential")
			}
		})
	}
}
func TestTaskSessionMCPWithoutCapabilityNeverUsesOwner(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")
	for _, name := range []string{"get_current_task", "prepare_current_task_report", "report_current_task", "push_current_task", "download_current_task_artifact", "read_current_task_artifact"} {
		if _, err := callTaskSessionTool(name, map[string]any{"progress": "Checkpoint", "attachment_id": "att"}); err == nil {
			t.Fatalf("unbound tool %s accepted", name)
		}
	}
	// Owner writes remain a different interface, not a fallback in these tools.
	if taskSessionOwnerWriteError() != nil {
		t.Fatal("unbound owner tools unexpectedly disabled")
	}
}

func TestTaskSessionMCPArtifactTools(t *testing.T) {
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", "")

	artifactsDir := t.TempDir()
	sampleContent := "Hello from MCP artifact test"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/task/bound/artifacts/att_test" {
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("Content-Disposition", `attachment; filename="report.txt"`)
			w.Write([]byte(sampleContent))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	cap := taskcli.Capability{
		API:          server.URL + "/task/bound",
		Token:        "test-token",
		TaskID:       "bound",
		AttemptID:    "attempt",
		ArtifactsDir: artifactsDir,
	}
	raw, _ := json.Marshal(cap)
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", string(raw))

	// 1. download_current_task_artifact
	res, err := callTaskSessionTool("download_current_task_artifact", map[string]any{"attachment_id": "att_test"})
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	resMap, ok := res.(map[string]any)
	if !ok || resMap["attachment_id"] != "att_test" {
		t.Fatalf("unexpected download result: %+v", res)
	}
	savedPath, _ := resMap["saved_path"].(string)
	if savedPath == "" {
		t.Fatalf("saved path missing: %+v", res)
	}
	data, err := os.ReadFile(savedPath)
	if err != nil || string(data) != sampleContent {
		t.Fatalf("downloaded content mismatch: %s, %v", string(data), err)
	}

	// 2. read_current_task_artifact
	readRes, err := callTaskSessionTool("read_current_task_artifact", map[string]any{"attachment_id": "att_test"})
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	readMap, ok := readRes.(map[string]any)
	if !ok || readMap["content"] != sampleContent {
		t.Fatalf("read content mismatch: %+v", readRes)
	}

	// 3. Security boundaries: foreign task_id rejected
	if _, err := callTaskSessionTool("download_current_task_artifact", map[string]any{"attachment_id": "att_test", "task_id": "foreign"}); err == nil {
		t.Fatal("expected error when passing task_id")
	}
	if _, err := callTaskSessionTool("read_current_task_artifact", map[string]any{"attachment_id": "att_test", "task_id": "foreign"}); err == nil {
		t.Fatal("expected error when passing task_id")
	}

	// 4. Missing attachment_id rejected
	if _, err := callTaskSessionTool("download_current_task_artifact", map[string]any{}); err == nil {
		t.Fatal("expected error when attachment_id missing")
	}
	if _, err := callTaskSessionTool("read_current_task_artifact", map[string]any{}); err == nil {
		t.Fatal("expected error when attachment_id missing")
	}

	// 5. Test MCP rejects symlink parent output
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(artifactsDir, "symlink_dir")); err == nil {
		if _, err := callTaskSessionTool("download_current_task_artifact", map[string]any{
			"attachment_id": "att_test",
			"output":        "symlink_dir/escaped.txt",
		}); err == nil {
			t.Fatal("expected error when downloading to symlink parent")
		}
		if _, err := os.Stat(filepath.Join(outside, "escaped.txt")); err == nil {
			t.Fatal("wrote outside attempt artifacts via MCP")
		}
	}
}
