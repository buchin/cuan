package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/config"
)

func TestInboxFormatRepairRetainsCandidateAndDoesNotReinvestigate(t *testing.T) {
	for _, unexpectedTool := range []bool{false, true} {
		t.Run(map[bool]string{false: "format only", true: "tool rejected"}[unexpectedTool], func(t *testing.T) {
			home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
			script := `#!/bin/sh
if [ ! -e "$HOME/first-response" ]; then
 touch "$HOME/first-response"
 printf '%s\n' '{"event":"result","result":{"status":"SUCCESS","response":"{\"reply\":\"Verified source finding.\",\"topic_title\":\"\",\"action\":null,\"extra\":true}"}}'
else
 case "$2" in *"TRANSPORT FORMAT REPAIR ONLY"*"Verified source finding."*) ;; *) exit 23;; esac
 for arg in "$@"; do [ "$arg" != "--add-dir" ] || exit 24; done
 printf x >> "$HOME/format-count"
`
			if unexpectedTool {
				script += `printf '%s\n' '{"event":"step_update","step_update":{"step_index":1,"state":"ACTIVE","step_type":"tool","tool_name":"read_file"}}'
sleep 1
`
			}
			script += `printf '%s\n' '{"event":"result","result":{"status":"SUCCESS","response":"{\"reply\":\"Verified source finding.\",\"topic_title\":\"\",\"action\":null}"}}'
fi
`
			if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			installInboxSandboxStub(t, bin)
			t.Setenv("PATH", bin+":/usr/bin:/bin")
			s := &Server{cfg: &config.Config{Environment: "staging"}}
			var trace []string
			out, failure := s.executeInboxCLI(context.Background(), "Coordinator", "agy", "Compare the already inspected options", home, workspace, func(k, v string) { trace = append(trace, k+":"+v) })
			count, _ := os.ReadFile(filepath.Join(home, "format-count"))
			if string(count) != "x" {
				t.Fatal("did not use one bounded repair", string(count), failure)
			}
			if unexpectedTool {
				if failure == "" || out != "" {
					t.Fatal("formatting tool execution accepted", out, failure)
				}
			} else {
				if failure != "" || out != "Verified source finding." {
					t.Fatal("lost verified candidate", out, failure)
				}
			}
			joined := strings.Join(trace, "\n")
			if !strings.Contains(joined, "response_recovery") || strings.Contains(joined, "Verified source finding.") {
				t.Fatal("missing safe trace or leaked candidate", joined)
			}
		})
	}
}
