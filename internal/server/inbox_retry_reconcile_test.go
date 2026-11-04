package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/buchin/cuan/internal/config"
)

func TestInboxReconciliationRejectsToolsAndApplicationActions(t *testing.T) {
	for _, scenario := range []string{"answer", "tool", "delegation", "memory"} {
		t.Run(scenario, func(t *testing.T) {
			bin, home := t.TempDir(), t.TempDir()
			reply := coordinatorJSON("Current evidence checked; previous actions were not replayed.", "", nil)
			if scenario == "delegation" {
				reply = coordinatorJSON("Starting duplicate work", "delegation", map[string]string{"title": "Duplicate", "description": "Repeat previous action", "project_id": "default", "authorization_message_id": "source"})
			}
			if scenario == "memory" {
				reply = coordinatorJSON("Saving again", "memory", map[string]string{"content": "Unexpected mutation"})
			}
			result, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": reply}})
			prefix := ""
			if scenario == "tool" {
				prefix = `{"event":"step_update","step_update":{"step_index":1,"step_type":"tool","state":"DONE","tool_name":"run_command"}}` + "\n"
			}
			script := "#!/bin/sh\ncat <<'RESULT'\n" + prefix + string(result) + "\nRESULT\n"
			if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			installInboxSandboxStub(t, bin)
			t.Setenv("PATH", bin+":/usr/bin:/bin")
			s := &Server{cfg: &config.Config{Environment: "staging"}}
			ctx := context.WithValue(context.Background(), inboxRetryReconcileKey{}, true)
			out, failure := s.executeInboxCLI(ctx, "@cuan", "agy", "Saved facts", home, "", nil)
			if scenario == "answer" {
				if out == "" || failure != "" {
					t.Fatal("fresh answer rejected", failure)
				}
			} else if failure == "" || out != "" {
				t.Fatal("uncertain activity replay was accepted", scenario)
			}
		})
	}
}
