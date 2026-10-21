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
	for _, tool := range []string{"", "view_file", "write_to_file", "run_command"} {
		t.Run(tool, func(t *testing.T) {
			home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
			result, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]any{"status": "SUCCESS", "response": coordinatorJSON("Here is a proposal", "approval", inboxApprovalPlan{Title: "Calculator", Description: "Deliver a downloadable calculator; verify the formulas.", ProjectID: "default"})}})
			event := ""
			if tool != "" {
				b, _ := json.Marshal(map[string]any{"event": "step_update", "step_update": map[string]any{"step_index": 1, "step_type": "tool", "state": "DONE", "tool_name": tool}})
				event = string(b) + "\n"
			}
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
