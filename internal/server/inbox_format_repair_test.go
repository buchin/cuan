package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
 case "$*" in *"--print-timeout 40s"*) ;; *) exit 26;; esac
 case "$2" in *"PRIVATE_CONVERSATION_CONTEXT"*|*"AGENTIC SCRUM RULES"*) exit 27;; esac
 [ "$PWD" = "$HOME/.cuan-chat-workspace" ] || exit 28
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
			out, failure := s.executeInboxCLI(context.Background(), "Coordinator", "agy", "PRIVATE_CONVERSATION_CONTEXT: Compare the already inspected options", home, workspace, func(k, v string) { trace = append(trace, k+":"+v) })
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

func TestInboxFormatRepairHasIndependentBoundedDeadline(t *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	bounded, stop := inboxFormatRepairContext(parent)
	defer stop()
	deadline, ok := bounded.Deadline()
	if !ok || time.Until(deadline) > 45*time.Second || time.Until(deadline) < 44*time.Second {
		t.Fatal("repair inherited the investigation budget", deadline)
	}
	short, shortCancel := context.WithTimeout(context.Background(), time.Second)
	defer shortCancel()
	child, childCancel := inboxFormatRepairContext(short)
	defer childCancel()
	a, _ := short.Deadline()
	b, _ := child.Deadline()
	if !a.Equal(b) {
		t.Fatal("repair extended parent deadline")
	}
}

func TestInboxFormatRepairStopsSilentProviderWithoutReplay(t *testing.T) {
	home, bin := t.TempDir(), t.TempDir()
	script := `#!/bin/sh
printf x >> "$HOME/silent-format-calls"
printf '%s\n' '{"event":"step_update","step_update":{"step_index":1,"state":"DONE","step_type":"agent_response","duration_seconds":0.01}}'
sleep 10
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	ctx = context.WithValue(ctx, inboxFormatRepairKey{}, coordinatorJSON("Saved findings.", "", nil))
	ctx = context.WithValue(ctx, inboxSchemaRetryKey{}, true)
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	started := time.Now()
	out, failure := s.executeInboxCLI(ctx, "Coordinator", "agy", "Use saved findings only", home, "", nil)
	calls, _ := os.ReadFile(filepath.Join(home, "silent-format-calls"))
	if !strings.Contains(failure, "formatted in time") || out != "" || string(calls) != "x" || time.Since(started) > 2*time.Second {
		t.Fatal("silent format stage did not stop without replay", failure, string(calls), time.Since(started))
	}
}

func TestInboxChoiceFormatRepairUsesExistingNamedOptions(t *testing.T) {
	home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
	reply := "Saya merekomendasikan laravel-traffic-guardian; alternatifnya laravel-botbuster. Silakan pilih nama repositori di bawah ini."
	choices := InboxPromptChoices{Options: []InboxPromptOption{{ID: "guardian", Label: "Pakai laravel-traffic-guardian", Variant: "primary"}, {ID: "botbuster", Label: "Pakai laravel-botbuster"}}, AllowOther: true}
	wrap := func(response string) string {
		raw, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": response}})
		return string(raw)
	}
	initial, repaired := wrap(coordinatorJSON(reply, "", nil)), wrap(coordinatorJSON(reply, "prompt_choices", choices))
	script := "#!/bin/sh\nprintf x >> \"$HOME/choice-calls\"\nif [ ! -e \"$HOME/choice-first\" ]; then\n touch \"$HOME/choice-first\"\n printf '%s\\n' '" + initial + "'\nelse\n" + `case "$2" in *"TRANSPORT FORMAT REPAIR ONLY"*) ;; *) exit 23;; esac
case "$2" in *"laravel-traffic-guardian"*"laravel-botbuster"*) ;; *) exit 24;; esac
for arg in "$@"; do [ "$arg" != "--add-dir" ] || exit 25; done
` + "printf '%s\\n' '" + repaired + "'\nfi\n"
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	out, failure := s.executeInboxCLI(context.Background(), "Coordinator", "agy", "Recommend the repository name", home, workspace, nil)
	_, parsed := splitInboxPromptChoices("agent", out)
	calls, _ := os.ReadFile(filepath.Join(home, "choice-calls"))
	if failure != "" || parsed == nil || len(parsed.Options) != 2 || string(calls) != "xx" || parseInboxApprovalPlan(out) != nil {
		t.Fatal("choice repair did not preserve safe named options", failure, string(calls), out)
	}
}
