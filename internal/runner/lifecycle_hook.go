package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// prepareAGYReportHook keeps lifecycle configuration outside both the registered
// repository and shared account HOME. Discovery is scoped to this invocation's
// additional workspace root; the parent removes it when execution ends.
func prepareAGYReportHook(parent string, env map[string]string, deadline time.Time) (string, map[string]string, error) {
	child := make(map[string]string, len(env)+2)
	for k, v := range env {
		child[k] = v
	}
	if env["CUAN_TASK_EXECUTABLE"] == "" || (env["CUAN_TASK_CAPABILITY_JSON"] == "" && env["CUAN_TASK_CAPABILITY_FILE"] == "") {
		return "", child, nil
	}
	root := filepath.Join(parent, "lifecycle")
	configDir := filepath.Join(root, ".agents")
	if err := os.Mkdir(root, 0700); err != nil {
		return "", nil, fmt.Errorf("prepare lifecycle root: %w", err)
	}
	if err := os.Mkdir(configDir, 0700); err != nil {
		return "", nil, fmt.Errorf("prepare lifecycle configuration: %w", err)
	}
	data, err := json.Marshal(map[string]any{"cuan-final-report": map[string]any{"Stop": []map[string]any{{"type": "command", "command": `"$CUAN_TASK_EXECUTABLE" task stop-hook`, "timeout": 12}}}})
	if err != nil {
		return "", nil, err
	}
	if err = os.WriteFile(filepath.Join(configDir, "hooks.json"), data, 0600); err != nil {
		return "", nil, err
	}
	child["CUAN_TASK_LIFECYCLE_DIR"] = root
	child["CUAN_TASK_REPORT_DEADLINE"] = deadline.UTC().Format(time.RFC3339Nano)
	return root, child, nil
}
