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
