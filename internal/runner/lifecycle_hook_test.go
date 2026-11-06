package runner

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReportHookConfigurationIsPrivateAndInvocationScoped(t *testing.T) {
	parent := t.TempDir()
	deadline := time.Now().Add(time.Minute)
	env := map[string]string{"HOME": "/account-profile", "CUAN_TASK_EXECUTABLE": "/runner path/cuan", "CUAN_TASK_CAPABILITY_JSON": "private-capability"}
	root, child, err := prepareAGYReportHook(parent, env, deadline)
	if err != nil {
		t.Fatal(err)
	}
	if env["CUAN_TASK_LIFECYCLE_DIR"] != "" || child["HOME"] != env["HOME"] || child["CUAN_TASK_REPORT_DEADLINE"] != deadline.UTC().Format(time.RFC3339Nano) {
		t.Fatal("shared environment changed or deadline lost")
	}
	info, err := os.Stat(root)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("lifecycle root is not private")
	}
	raw, err := os.ReadFile(filepath.Join(root, ".agents", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config map[string]struct {
		Stop []struct {
			Command string
			Timeout int
		}
	}
	if json.Unmarshal(raw, &config) != nil {
		t.Fatal("invalid native config")
	}
	hook := config["cuan-final-report"].Stop
	if len(hook) != 1 || hook[0].Command != `"$CUAN_TASK_EXECUTABLE" task stop-hook` || hook[0].Timeout != 12 {
		t.Fatal("unexpected hook contract")
	}
	for _, item := range []string{"private-capability", "/account-profile"} {
		if bytes.Contains(raw, []byte(item)) {
			t.Fatal("hook config contains private profile data")
		}
	}
	other, _, err := prepareAGYReportHook(t.TempDir(), env, deadline)
	if err != nil || other == root {
		t.Fatal("invocations share lifecycle state")
	}
}

func TestReportHookRequiresBoundCapability(t *testing.T) {
	parent := t.TempDir()
	root, child, err := prepareAGYReportHook(parent, map[string]string{"HOME": "profile"}, time.Now())
	if err != nil || root != "" || child["CUAN_TASK_LIFECYCLE_DIR"] != "" {
		t.Fatal("unbound hook registered")
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatal("unbound session mutated")
	}
}
