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

func TestIndependentArtifactBoundaries(t *testing.T) {
	for _, scenario := range []string{"partial response", "symlink parent"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			outside := t.TempDir()
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Disposition", `attachment; filename="result.txt"`)
				if scenario == "partial response" {
					w.Header().Set("Content-Range", "bytes 0-3/100")
					w.WriteHeader(http.StatusPartialContent)
				}
				w.Write([]byte("data"))
			}))
			defer api.Close()
			capability, _ := json.Marshal(Capability{API: api.URL, Token: "fixture", TaskID: "task", AttemptID: "attempt", ArtifactsDir: root})
			t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
			t.Setenv("CUAN_TASK_CAPABILITY_JSON", string(capability))
			args := []string{"artifact", "download", "attachment"}
			if scenario == "symlink parent" {
				if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
					t.Fatal(err)
				}
				args = []string{"artifact", "download", "--output", "link/escaped.txt", "attachment"}
			}
			var out, errs bytes.Buffer
			code := Run(args, &out, &errs)
			if code == 0 {
				t.Error("unsafe transfer accepted", scenario)
			}
			if _, err := os.Stat(filepath.Join(outside, "escaped.txt")); err == nil {
				t.Error("wrote outside attempt artifacts")
			}
		})
	}
}

func TestIndependentArtifactDocumentedOutput(t *testing.T) {
	root := t.TempDir()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="server-name.txt"`)
		w.Write([]byte("complete artifact"))
	}))
	defer api.Close()
	capability, _ := json.Marshal(Capability{API: api.URL, Token: "fixture", TaskID: "task", AttemptID: "attempt", ArtifactsDir: root})
	t.Setenv("CUAN_TASK_CAPABILITY_FILE", "")
	t.Setenv("CUAN_TASK_CAPABILITY_JSON", string(capability))
	var out, errs bytes.Buffer
	if code := Run([]string{"artifact", "download", "attachment", "--output", "requested.txt"}, &out, &errs); code != 0 {
		t.Fatalf("documented invocation failed: %s", errs.String())
	}
	data, err := os.ReadFile(filepath.Join(root, "requested.txt"))
	if err != nil || string(data) != "complete artifact" {
		t.Fatalf("documented --output was not honored: data=%q err=%v", data, err)
	}
	if _, err := os.Stat(filepath.Join(root, "server-name.txt")); !os.IsNotExist(err) {
		t.Fatalf("unexpected default output created: %v", err)
	}
	// A repeated download must fail and preserve the existing destination bytes.
	if err := os.WriteFile(filepath.Join(root, "requested.txt"), []byte("preserve me"), 0600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errs.Reset()
	if code := Run([]string{"artifact", "download", "attachment", "--output", "requested.txt"}, &out, &errs); code == 0 {
		t.Error("existing destination accepted")
	}
	data, err = os.ReadFile(filepath.Join(root, "requested.txt"))
	if err != nil || string(data) != "preserve me" {
		t.Fatalf("existing destination modified: data=%q err=%v", data, err)
	}
}
