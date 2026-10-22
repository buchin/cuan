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

func coordinatorJSON(reply, kind string, payload any) string {
	var action any
	if kind != "" {
		action = map[string]any{"kind": kind, "payload": payload}
	}
	raw, _ := json.Marshal(map[string]any{"reply": reply, "topic_title": "", "action": action})
	return string(raw)
}

func TestCoordinatorResponseActionsUseExistingContracts(t *testing.T) {
	cases := []struct {
		kind    string
		payload any
		marker  string
	}{
		{"approval", inboxApprovalPlan{Title: "Build calculator", Description: "Deliver a downloadable HTML calculator and verify its calculations.", ProjectID: "default"}, "cuan-approval"},
		{"delegation", inboxDelegation{Title: "Build calculator", Description: "Deliver and verify the approved calculator.", ProjectID: "default", AuthorizationID: "owner-message"}, "cuan-delegate"},
		{"delegation", inboxDelegation{TaskID: "existing-task", Revision: "Gap: missing download. Area: existing output. Verify download. Finish when attached.", ProjectID: "default", AuthorizationID: "original-approval"}, "cuan-delegate"},
		{"memory", inboxMemoryAction{ContactID: "contact", Kind: "note", Body: "Enjoys hiking", AuthorizationID: "owner-message"}, "cuan-memory"},
		{"memory_search", inboxMemorySearch{Query: "hiking", AuthorizationID: "owner-message"}, "cuan-search"},
		{"task_draft", InboxTaskDraft{Title: "Draft calculator", Description: "Draft only", ProjectID: "default"}, "cuan-task"},
	}
	for _, c := range cases {
		t.Run(c.kind+c.marker, func(t *testing.T) {
			out, err := decodeInboxCoordinatorResponse(coordinatorJSON("Here is the requested plan.", c.kind, c.payload))
			if err != nil || !strings.HasPrefix(out, "Here is") || strings.Count(out, "```"+c.marker+"\n") != 1 {
				t.Fatalf("Invalid conversion: %s %v", out, err)
			}
			if c.kind == "delegation" {
				_, d, e := parseInboxDelegation(out)
				if e != nil || d.AuthorizationID == "" {
					t.Fatal("lost authorization")
				}
			}
		})
	}
	out, err := decodeInboxCoordinatorResponse(coordinatorJSON("**Useful answer**", "", nil))
	if err != nil || out != "**Useful answer**" {
		t.Fatal(out, err)
	}
}

func TestCoordinatorResponseRejectsAmbiguousOrUnmanagedActions(t *testing.T) {
	valid := coordinatorJSON("Ready", "approval", inboxApprovalPlan{Title: "Calculator", Description: "Approved scope", ProjectID: "default"})
	cases := []string{
		"I created files in my profile.",
		`{"reply":"Hi","topic_title":""}`,
		`{"reply":null,"topic_title":"","action":null}`,
		`{"reply":"First","reply":"Last","topic_title":"","action":null}`,
		`{"reply":"Hi","topic_title":"","action":{"kind":"approval","payload":{"title":"A","title":"B","description":"scope","project_id":"default"}}}`,

		coordinatorJSON("Download file:///private/result.html", "", nil),
		coordinatorJSON("```cuan-delegate\n{}\n```", "", nil),
		coordinatorJSON("Done", "execute", map[string]any{"command": "write"}),
		coordinatorJSON("Plan", "approval", map[string]any{"title": "Empty scope", "project_id": "default"}),
		coordinatorJSON("Plan", "approval", map[string]any{"title": "Mixed", "description": "scope", "project_id": "default", "authorization_message_id": "fake"}),
		coordinatorJSON("Go", "delegation", map[string]any{"title": "Missing authorization", "description": "scope", "project_id": "default"}),
		coordinatorJSON("Go", "delegation", map[string]any{"title": "Mixed project", "description": "scope", "project_id": "default", "project_name": "Other", "authorization_message_id": "owner"}),
		`{"reply":"Hi","action":{"kind":"memory","payload":{"contact_id":"a","owner_id":"injected"}}}`,
		`{"reply":"Hi","topic_title":"","action":null,"other":"injected"}`,
		valid + coordinatorJSON("Conflicting", "", nil),
		valid + " trailing prose",
	}
	for _, raw := range cases {
		if out, err := decodeInboxCoordinatorResponse(raw); err == nil {
			t.Fatalf("accepted invalid reply: %q", out)
		}
	}
	out, err := decodeInboxCoordinatorResponse("```json\n" + valid + "\n```\n" + valid)
	if err != nil || !strings.Contains(out, "cuan-approval") {
		t.Fatal("identical native echo rejected", err)
	}
}

func TestCoordinatorCLIRequiresSchemaAndRejectsNativeExecution(t *testing.T) {
	for _, tool := range []string{"", "view_file", "finish", "write_to_file", "run_command"} {
		t.Run(tool, func(t *testing.T) {
			home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
			result, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]any{"status": "SUCCESS", "response": coordinatorJSON("Here is a proposal", "approval", inboxApprovalPlan{Title: "Calculator", Description: "Deliver a downloadable calculator; verify the formulas.", ProjectID: "default"})}})
			event := ""
			if tool != "" {
				b, _ := json.Marshal(map[string]any{"event": "step_update", "step_update": map[string]any{"step_index": 1, "step_type": "tool", "state": "DONE", "tool_name": tool}})
				event = string(b) + "\n"
			}
			event = "{\"event\":\"step_update\",\"step_update\":{\"step_index\":0,\"state\":\"DONE\",\"step_type\":\"user_input\"}}\n" + event + "{\"event\":\"step_update\",\"step_update\":{\"step_index\":2,\"state\":\"DONE\",\"step_type\":\"finish\"}}\n"
			script := "#!/bin/sh\ncase \"$*\" in *--json-schema*) ;; *) exit 8;; esac\ncat <<'RESULT'\n" + event + string(result) + "\nRESULT\n"
			if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			installInboxSandboxStub(t, bin)
			t.Setenv("PATH", bin+":/usr/bin:/bin")
			s := &Server{cfg: &config.Config{Environment: "staging"}}
			safe := false
			out, failure := s.executeInboxCLIWithSafety(context.Background(), "Coordinator", "agy", "Owner requests a proposal", home, workspace, nil, &safe)
			if tool == "write_to_file" || tool == "run_command" {
				if out != "" || !strings.Contains(failure, "outside the chat workflow") || safe {
					t.Fatal("native execution accepted or replayable", out, failure, safe)
				}
			} else {
				if failure != "" || parseInboxApprovalPlan(out) == nil || !safe {
					t.Fatal("valid proposal rejected", out, failure, safe)
				}
			}
		})
	}
}

// Fixtures describe the persisted UI contract; emit the new provider transport
// independently so lifecycle tests still assert real application effects.
func coordinatorFixtureResponse(content string) string {
	kinds := map[string]string{"cuan-approval": "approval", "cuan-delegate": "delegation", "cuan-memory": "memory", "cuan-search": "memory_search", "cuan-task": "task_draft"}
	for marker, kind := range kinds {
		start := strings.Index(content, "```"+marker+"\n")
		if start < 0 {
			continue
		}
		body := content[start+len("```"+marker+"\n"):]
		end := strings.Index(body, "```")
		if end < 0 {
			panic("bad fixture")
		}
		var payload any
		if json.Unmarshal([]byte(strings.TrimSpace(body[:end])), &payload) != nil {
			panic("bad fixture payload")
		}
		return coordinatorJSON(strings.TrimSpace(content[:start]), kind, payload)
	}
	return coordinatorJSON(content, "", nil)
}

func TestCoordinatorRuntimeMetadataCannotHideATool(t *testing.T) {
	for _, kind := range []string{"user_input", "finish"} {
		var stream inboxRuntimeStream
		stream.Write([]byte(`{"event":"step_update","step_update":{"step_type":"` + kind + `","tool_name":"run_command","step_index":1,"state":"DONE"}}` + "\n"))
		if !stream.unsafeTool {
			t.Fatal("tool hidden in lifecycle metadata", kind)
		}
	}
}

func TestCoordinatorInvalidSchemaRetriesOnlyOnceWithoutEffects(t *testing.T) {
	for _, scenario := range []string{"repaired", "still-invalid", "unsafe", "unsafe-retry"} {
		t.Run(scenario, func(t *testing.T) {
			bin, home := t.TempDir(), t.TempDir()
			result, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]any{"status": "SUCCESS", "response": coordinatorJSON("Recovered answer", "", nil)}})
			first := `{"event":"result","result":{"status":"SUCCESS","response":"invalid transport"}}`
			if scenario == "unsafe" {
				first = "{\"event\":\"step_update\",\"step_update\":{\"step_index\":1,\"step_type\":\"tool\",\"state\":\"DONE\",\"tool_name\":\"run_command\"}}\n" + first
			}
			if scenario == "unsafe-retry" {
				result = append([]byte("{\"event\":\"step_update\",\"step_update\":{\"step_index\":1,\"step_type\":\"tool\",\"state\":\"DONE\",\"tool_name\":\"run_command\"}}\n"), result...)
			}
			if scenario == "still-invalid" {
				result = []byte(first)
			}
			script := "#!/bin/sh\nprintf x >> \"$HOME/calls\"\nif [ \"$(cat \"$HOME/calls\")\" = x ]; then\ncat <<'FIRST'\n" + first + "\nFIRST\nelse\ncat <<'SECOND'\n" + string(result) + "\nSECOND\nfi\n"
			if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			installInboxSandboxStub(t, bin)
			t.Setenv("PATH", bin+":/usr/bin:/bin")
			s := &Server{cfg: &config.Config{Environment: "staging"}}
			safe := false
			out, failure := s.executeInboxCLIWithSafety(context.Background(), "Coordinator", "agy", "Answer the question", home, t.TempDir(), nil, &safe)
			calls, _ := os.ReadFile(filepath.Join(home, "calls"))
			if scenario == "repaired" {
				if out != "Recovered answer" || failure != "" || !safe || string(calls) != "xx" {
					t.Fatal(out, failure, safe, string(calls))
				}
			} else {
				want := "xx"
				if scenario == "unsafe" {
					want = "x"
				}
				if strings.HasPrefix(scenario, "unsafe") && safe {
					t.Fatal("unsafe retry outcome was declared safe")
				}
				if out != "" || failure == "" || string(calls) != want {
					t.Fatal("unbounded/unsafe repair", out, failure, string(calls))
				}
			}
		})
	}
}

func TestCoordinatorToolBudgetRecovery(t *testing.T) {
	home, bin := t.TempDir(), t.TempDir()
	// Script:
	// Call 1: outputs 4 tools (exceeding max 3). Note: tool 4 triggers SIGKILL from cancel(),
	// but if the shell continues it would output invalid.
	// Call 2 (recovery): direct coordinator response with structured approval action.
	script := `#!/bin/sh
printf x >> "$HOME/calls"
if [ "$(cat "$HOME/calls")" = x ]; then
  cat <<'FIRST'
{"event":"init","conversation_id":"11111111-2222-3333-4444-555555555555"}
{"event":"step_update","step_update":{"step_index":1,"state":"ACTIVE","step_type":"tool","tool_name":"read_file"}}
{"event":"step_update","step_update":{"step_index":1,"state":"DONE","step_type":"tool","tool_name":"read_file"}}
{"event":"step_update","step_update":{"step_index":2,"state":"ACTIVE","step_type":"tool","tool_name":"view_file"}}
{"event":"step_update","step_update":{"step_index":2,"state":"DONE","step_type":"tool","tool_name":"view_file"}}
{"event":"step_update","step_update":{"step_index":3,"state":"ACTIVE","step_type":"tool","tool_name":"grep_search"}}
{"event":"step_update","step_update":{"step_index":3,"state":"DONE","step_type":"tool","tool_name":"grep_search"}}
{"event":"step_update","step_update":{"step_index":4,"state":"ACTIVE","step_type":"tool","tool_name":"read_file"}}
FIRST
  sleep 10
else
  cat <<'SECOND'
{"event":"result","result":{"status":"SUCCESS","response":"{\"reply\":\"Direct recovery answer\",\"topic_title\":\"\",\"action\":{\"kind\":\"approval\",\"payload\":{\"title\":\"Fix shortcut\",\"description\":\"Add keydown handler\",\"project_id\":\"default\"}}}"}}
SECOND
fi
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	safe := false
	out, failure := s.executeInboxCLIWithSafety(context.Background(), "Coordinator", "agy", "General question", home, t.TempDir(), nil, &safe)
	calls, _ := os.ReadFile(filepath.Join(home, "calls"))
	if failure != "" || !safe || string(calls) != "xx" || parseInboxApprovalPlan(out) == nil || !strings.Contains(out, "Direct recovery answer") {
		t.Fatalf("expected direct recovery proposal with approval card, got out=%q failure=%q safe=%v calls=%s", out, failure, safe, string(calls))
	}
}

func TestCoordinatorResponseSynthesizesApprovalForConfirmationQuestions(t *testing.T) {
	raw := coordinatorJSON("Untuk pekerjaan **implementasi shortcut Cmd/Ctrl + Enter pada form Studio**, saya siap langsung menugaskan dan menjalankan eksekusinya ke tim dev sekarang. Apakah Anda ingin saya langsung jalankan eksekusinya?", "", nil)
	out, err := decodeInboxCoordinatorResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	plan := parseInboxApprovalPlan(out)
	if plan == nil {
		t.Fatalf("expected approval plan to be synthesized, got %s", out)
	}
	if !strings.Contains(plan.Title, "implementasi shortcut Cmd/Ctrl + Enter pada form Studio") {
		t.Errorf("unexpected title: %q", plan.Title)
	}
	if plan.ProjectName != "Cuan" {
		t.Errorf("expected project Cuan, got %q", plan.ProjectName)
	}
}
