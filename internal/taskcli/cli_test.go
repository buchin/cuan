package taskcli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestTaskToolUsesOnlyPrivateCapability(t *testing.T) {
	const token = "opaque-test-capability"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token || r.URL.RawQuery != "" {
			t.Error("credential transport")
		}
		if r.Method == "POST" {
			var data map[string]any
			json.NewDecoder(r.Body).Decode(&data)
			if data["summary"] != "Actual work" || data["state"] != "review" || data["result_kind"] != "answer" {
				t.Error(data)
			}
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "cap.json")
	data, _ := json.Marshal(Capability{API: server.URL, Token: token, TaskID: "bound", AttemptID: "attempt"})
	os.WriteFile(path, data, 0600)
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", path)
	var out, err bytes.Buffer
	if Run([]string{"report", "--summary", "Actual work", "--state", "review", "--result-kind", "answer"}, &out, &err) != 0 {
		t.Fatal(err.String())
	}
	if bytes.Contains(out.Bytes(), []byte(token)) {
		t.Fatal("credential leaked")
	}
	os.Chmod(path, 0644)
	if Run([]string{"get"}, &out, &err) == 0 {
		t.Fatal("nonprivate capability accepted")
	}
	if Run([]string{"--help"}, &out, &err) != 0 {
		t.Fatal("help unavailable")
	}
}

func TestTaskPushUsesBoundCapabilityEndpoint(t *testing.T) {
	called := false
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != "POST" || r.URL.Path != "/bound/push" || r.Header.Get("Authorization") != "Bearer fixture" {
			t.Error("push escaped task capability")
		}
		w.Write([]byte(`{"pushed":true}`))
	}))
	defer api.Close()
	path := filepath.Join(t.TempDir(), "capability.json")
	raw, _ := json.Marshal(Capability{API: api.URL + "/bound", Token: "fixture", TaskID: "bound"})
	os.WriteFile(path, raw, 0600)
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", path)
	var out, errOut bytes.Buffer
	if Run([]string{"push"}, &out, &errOut) != 0 || !called {
		t.Fatal(errOut.String())
	}
	called = false
	if Run([]string{"push", "--state", "done"}, &out, &errOut) != 2 || called {
		t.Fatal("push accepted report mutation")
	}
}

func TestTaskToolAcceptsRunnerInlineCapability(t *testing.T) {
	called := false
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Header.Get("Authorization") != "Bearer inline-token" {
			t.Fatal("inline capability was not used")
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer api.Close()
	raw, _ := json.Marshal(Capability{API: api.URL, Token: "inline-token", TaskID: "bound"})
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", string(raw))
	var out, errOut bytes.Buffer
	if Run([]string{"get"}, &out, &errOut) != 0 || !called {
		t.Fatal(errOut.String())
	}
}

func TestTaskReportSendsRepeatedArtifactPaths(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data map[string]any
		json.NewDecoder(r.Body).Decode(&data)
		paths, ok := data["artifacts"].([]any)
		if !ok || len(paths) != 2 || paths[0] != "output.html" || paths[1] != "README.md" {
			t.Error("missing artifact paths", data)
		}
		w.Write([]byte(`{"ok":true}`))
	}))
	defer api.Close()
	path := filepath.Join(t.TempDir(), "capability.json")
	raw, _ := json.Marshal(Capability{API: api.URL, Token: "fixture", TaskID: "bound"})
	os.WriteFile(path, raw, 0600)
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", path)
	var out, errOut bytes.Buffer
	if Run([]string{"report", "--state", "done", "--artifact", "output.html", "--artifact", "README.md"}, &out, &errOut) != 0 {
		t.Fatal(errOut.String())
	}
	if Run([]string{"get", "--artifact", "output.html"}, &out, &errOut) != 2 {
		t.Fatal("get accepted delivery flags")
	}
}
