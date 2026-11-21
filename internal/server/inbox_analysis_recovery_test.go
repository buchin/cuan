package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/config"
)

func TestInboxDeniedToolContinuesAnalysisWithoutApplyingActions(t *testing.T) {
	for _, scenario := range []string{"read-only", "denied-again", "application-action", "topic-change", "execution-tool", "malformed-stream", "failed-process"} {
		t.Run(scenario, func(t *testing.T) {
			bin, home := t.TempDir(), t.TempDir()
			denied := `{"event":"step_update","step_update":{"step_index":2,"step_type":"tool","state":"DONE","tool_name":"run_command"}}
{"event":"result","result":{"status":"SUCCESS","response":"PRIVATE_REJECTED_PREAMBLE","denied_actions":[{"action":"shell_command","display_name":"PRIVATE_COMMAND"}]}}`
			recovery := coordinatorJSON("The saved outcome shows a denied terminal search. I can inspect the source with read-only search.", "", nil)
			if scenario == "application-action" {
				recovery = coordinatorJSON("Starting work", "delegation", map[string]string{"title": "Unexpected work", "description": "Change files", "project_id": "default", "authorization_message_id": "owner"})
			}
			if scenario == "topic-change" {
				recovery = strings.Replace(recovery, `"topic_title":""`, `"topic_title":"Changed title"`, 1)
			}
			second, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": recovery}})
			prefix := `{"event":"step_update","step_update":{"step_index":2,"step_type":"tool","state":"DONE","tool_name":"grep_search"}}` + "\n"
			if scenario == "execution-tool" {
				prefix = strings.Replace(prefix, "grep_search", "run_command", 1)
			}
			if scenario == "denied-again" {
				second = []byte(denied)
				prefix = ""
			}
			if scenario == "malformed-stream" {
				denied = "not-json\n" + denied
			}
			exit := "0"
			if scenario == "failed-process" {
				exit = "1"
			}
			script := "#!/bin/sh\nprintf x >> \"$HOME/calls\"\nif [ \"$(cat \"$HOME/calls\")\" = x ]; then\ncat <<'FIRST'\n" + denied + "\nFIRST\nexit " + exit + "\nelse\nprintf '%s\\n' \"$@\" > \"$HOME/recovery-args\"\ncat <<'SECOND'\n" + prefix + string(second) + "\nSECOND\nfi\n"
			if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			installInboxSandboxStub(t, bin)
			t.Setenv("PATH", bin+":/usr/bin:/bin")
			server := &Server{cfg: &config.Config{Environment: "staging"}}
			safe := true
			var trace []string
			out, failure := server.executeInboxCLIWithSafety(context.Background(), "Coordinator", "agy", "TARGET_ORIGINAL_CONTEXT", home, t.TempDir(), func(k, v string) { trace = append(trace, k+":"+v) }, &safe)
			calls, _ := os.ReadFile(filepath.Join(home, "calls"))
			wantCalls := "xx"
			if scenario == "malformed-stream" || scenario == "failed-process" {
				wantCalls = "x"
			}
			if string(calls) != wantCalls || safe {
				t.Fatal("unbounded recovery or execution uncertainty discarded", string(calls), safe)
			}
			if scenario == "read-only" {
				if failure != "" || !strings.Contains(out, "saved outcome") {
					t.Fatal(out, failure)
				}
				args, _ := os.ReadFile(filepath.Join(home, "recovery-args"))
				for _, required := range []string{"TARGET_ORIGINAL_CONTEXT", "Return action:null", "grep_search", "--sandbox"} {
					if !strings.Contains(string(args), required) {
						t.Fatal("lost recovery constraint", required)
					}
				}
				if strings.Contains(string(args), "PRIVATE_") {
					t.Fatal("untrusted failed reply included in retry")
				}
			} else if out != "" || failure == "" {
				t.Fatal("unsafe recovery accepted", out, failure)
			}
			joined := strings.Join(trace, "\n")
			if strings.Contains(joined, "PRIVATE_") || strings.Contains(joined, "recovery_safe") {
				t.Fatal("diagnostic leaked or execution declared safe", joined)
			}
		})
	}
}
