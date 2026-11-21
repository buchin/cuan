package server

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/config"
	"github.com/buchin/cuan/internal/workflow"
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
		{"task_cancel", inboxTaskCancelAction{TaskID: "task-123", Reason: "User decided to cancel"}, "cuan-task-cancel"},
		{"task_reopen", inboxTaskReopenAction{TaskID: "task-123", Reason: "User wants to retry"}, "cuan-task-reopen"},
		{"prompt_choices", InboxPromptChoices{Question: "Langkah selanjutnya?", Options: []InboxPromptOption{{ID: "done", Label: "Selesai (Done)", Variant: "primary"}, {ID: "revisit", Label: "Revisi Dulu", Variant: "outline"}}}, "cuan-prompt"},
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

func TestCoordinatorExplicitChoiceRequiresStructuredOptionsWithoutApproval(t *testing.T) {
	reply := "Saya merekomendasikan laravel-traffic-guardian; alternatifnya laravel-botbuster.\n\nSilakan pilih nama repositori yang ingin kita tetapkan di bawah ini untuk lanjut ke langkah berikutnya:"
	if _, err := decodeInboxCoordinatorResponse(coordinatorJSON(reply, "", nil)); err == nil {
		t.Fatal("missing choice buttons accepted")
	}
	wrong := coordinatorJSON(reply, "approval", inboxApprovalPlan{Title: "Choose a name", Description: "Create and publish a repository", ProjectID: "default"})
	if _, err := decodeInboxCoordinatorResponse(wrong); err == nil {
		t.Fatal("preference choice promoted to executable approval")
	}
	if out, failure := recoverCoordinatorReply(wrong, false); failure == "" || parseInboxApprovalPlan(out) != nil {
		t.Fatal("recovery promoted preference to approval")
	}
	if out, failure := recoverCoordinatorReply(coordinatorJSON(reply, "", nil), false); failure == "" || parseInboxApprovalPlan(out) != nil {
		t.Fatal("missing choices became executable approval", out)
	}
	choices := InboxPromptChoices{Options: []InboxPromptOption{{ID: "guardian", Label: "Pakai laravel-traffic-guardian", Variant: "primary"}, {ID: "botbuster", Label: "Pakai laravel-botbuster"}}, AllowOther: true}
	out, err := decodeInboxCoordinatorResponse(coordinatorJSON(reply, "prompt_choices", choices))
	if err != nil {
		t.Fatal(err)
	}
	_, parsed := splitInboxPromptChoices("agent", out)
	if parsed == nil || len(parsed.Options) != 2 || !parsed.AllowOther || parseInboxApprovalPlan(out) != nil {
		t.Fatal("lost choice semantics", out)
	}
	// A supported native provider may return a JSON-string payload or extra
	// transport fields. Recovery preserves choices instead of dropping them.
	payload, _ := json.Marshal(choices)
	lenient, _ := json.Marshal(map[string]any{"reply": reply, "extra": "transport", "action": map[string]any{"kind": "prompt_choices", "payload": string(payload)}})
	out, failure := recoverCoordinatorReply(string(lenient), false)
	if _, parsed := splitInboxPromptChoices("agent", out); failure != "" || parsed == nil || len(parsed.Options) != 2 {
		t.Fatal("recovery lost structured choices", failure, out)
	}
	if out, err := decodeInboxCoordinatorResponseForMode(coordinatorJSON(reply, "", nil), true); err != nil || out != reply {
		t.Fatal("read-only analysis gained an action", err)
	}
	for _, text := range []string{"Here is the requested naming recommendation.", "> Please choose one was quoted in the original document."} {
		if _, err := decodeInboxCoordinatorResponse(coordinatorJSON(text, "", nil)); err != nil {
			t.Fatal("ordinary answer rejected", text)
		}
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
	kinds := map[string]string{
		"cuan-approval":    "approval",
		"cuan-delegate":    "delegation",
		"cuan-memory":      "memory",
		"cuan-search":      "memory_search",
		"cuan-task":        "task_draft",
		"cuan-task-cancel": "task_cancel",
		"cuan-task-reopen": "task_reopen",
	}
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
	// Call 1: outputs 13 tools (exceeding coordinator max 12). Note: tool 13 triggers SIGKILL from cancel(),
	// but if the shell continues it would output invalid.
	// Call 2 (recovery): direct coordinator response with structured approval action.
	var firstBuilder strings.Builder
	firstBuilder.WriteString("{\"event\":\"init\",\"conversation_id\":\"11111111-2222-3333-4444-555555555555\"}\n")
	for i := 1; i <= 12; i++ {
		num := strconv.Itoa(i)
		firstBuilder.WriteString("{\"event\":\"step_update\",\"step_update\":{\"step_index\":" + num + ",\"state\":\"ACTIVE\",\"step_type\":\"tool\",\"tool_name\":\"read_file\"}}\n")
		firstBuilder.WriteString("{\"event\":\"step_update\",\"step_update\":{\"step_index\":" + num + ",\"state\":\"DONE\",\"step_type\":\"tool\",\"tool_name\":\"read_file\"}}\n")
	}
	firstBuilder.WriteString("{\"event\":\"step_update\",\"step_update\":{\"step_index\":13,\"state\":\"ACTIVE\",\"step_type\":\"tool\",\"tool_name\":\"read_file\"}}\n")
	firstOutput := firstBuilder.String()

	script := `#!/bin/sh
printf x >> "$HOME/calls"
if [ "$(cat "$HOME/calls")" = x ]; then
  cat <<'FIRST'
` + firstOutput + `FIRST
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
	t.Setenv("CUAN_CHAT_TOOL_BUDGET", "2")
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

	// Verify discussion closing questions automatically synthesize approval cards
	rawDiscussion := coordinatorJSON("Berikut beberapa tool koordinasi yang dibutuhkan: 1. **Inspeksi proyek dan task terstruktur**. Ada yang ingin diperdalam?", "", nil)
	outDiscussion, err := decodeInboxCoordinatorResponse(rawDiscussion)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	planDiscussion := parseInboxApprovalPlan(outDiscussion)
	if planDiscussion == nil {
		t.Fatalf("expected approval plan for discussion closing, got %s", outDiscussion)
	}
	if !strings.Contains(planDiscussion.Title, "Inspeksi proyek dan task terstruktur") {
		t.Errorf("unexpected title in synthesized plan: %q", planDiscussion.Title)
	}

	// Verify proactive technical solution proposal automatically synthesizes approval card
	rawSolution := coordinatorJSON("Pendekatan terbaik untuk implementasi di backend adalah kombinasi debounce dan collapse key per room agar tidak terjadi flooding push notification.", "", nil)
	outSolution, err := decodeInboxCoordinatorResponse(rawSolution)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	planSolution := parseInboxApprovalPlan(outSolution)
	if planSolution == nil {
		t.Fatalf("expected approval plan for technical solution proposal, got %s", outSolution)
	}
	if planSolution.Title == "" {
		t.Errorf("expected non-empty title in synthesized plan, got empty")
	}
}

func TestCoordinatorInstructionAutonomousErrorTriage(t *testing.T) {
	for name, prompt := range map[string]string{
		"inboxCoordinatorInstruction": inboxCoordinatorInstruction,
		"workflow.Coordinator":        workflow.Coordinator,
	} {
		t.Run(name, func(t *testing.T) {
			for _, required := range []string{
				"AUTONOMOUS ERROR TRIAGE",
				"NEVER deflect",
				"NEVER offload manual troubleshooting checklists",
				"Emit a delegation action immediately",
			} {
				if !strings.Contains(strings.ToLower(prompt), strings.ToLower(required)) {
					t.Fatalf("%s missing required directive %q", name, required)
				}
			}
		})
	}
}

func TestCoordinatorInstructionAffirmativeTriggersAndProactivePlan(t *testing.T) {
	for name, prompt := range map[string]string{
		"inboxCoordinatorInstruction": inboxCoordinatorInstruction,
		"workflow.Coordinator":        workflow.Coordinator,
	} {
		t.Run(name, func(t *testing.T) {
			for _, required := range []string{
				`"Gas" ke mana`,
				"kan ente sm",
				"synthesize",
			} {
				if !strings.Contains(strings.ToLower(prompt), strings.ToLower(required)) {
					t.Fatalf("%s missing required affirmative trigger directive %q", name, required)
				}
			}
		})
	}
}

func TestCoordinatorInstructionProactiveProblemSolving(t *testing.T) {
	for name, prompt := range map[string]string{
		"inboxCoordinatorInstruction": inboxCoordinatorInstruction,
		"workflow.Coordinator":        workflow.Coordinator,
	} {
		t.Run(name, func(t *testing.T) {
			for _, required := range []string{
				"PROACTIVE PROBLEM-SOLVING",
				"ZERO PASSIVE THEORIZING",
				"Project Managers own the technical solution",
				"emit an approval action",
			} {
				if !strings.Contains(strings.ToLower(prompt), strings.ToLower(required)) {
					t.Fatalf("%s missing required directive %q", name, required)
				}
			}
		})
	}
}

func TestCoordinatorResponseSynthesizesApprovalForAutonomousAnalysisSession(t *testing.T) {
	raw := coordinatorJSON("Inspeksi mendalam pada komponen web/src/components/layout/AppShell.tsx memerlukan analisis menyeluruh di luar batas chat. Saya dapat mendelegasikan sesi analisis mandiri ke tim dev untuk mengaudit struktur layout dan navbar tanpa memerlukan tangkapan layar manual. Apakah Anda setuju?", "", nil)
	out, err := decodeInboxCoordinatorResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	plan := parseInboxApprovalPlan(out)
	if plan == nil {
		t.Fatalf("expected approval plan to be synthesized for deep analysis fallback, got %s", out)
	}
	if plan.Title != "Sesi Analisis Mandiri" {
		t.Errorf("expected title 'Sesi Analisis Mandiri', got %q", plan.Title)
	}
}

func TestCoordinatorAdvisoryModeAllowsNullActionForComponentQuestions(t *testing.T) {
	advice := "Setelah memeriksa struktur UI di `features/inbox/`, lokasi penempatan terbaik untuk tombol **Mark all as read** adalah di Header Sub-Panel Inbox, persis di sebelah tombol Archive (baris 188) atau di dalam menu dropdown titik tiga ItemActions."
	raw := coordinatorJSON(advice, "", nil)
	out, err := decodeInboxCoordinatorResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error decoding advisory response: %v", err)
	}
	if strings.Contains(out, "cuan-approval") || strings.Contains(out, "cuan-delegate") {
		t.Fatalf("expected advisory mode to return clean chat reply without synthesized work markers, got %s", out)
	}
	if !strings.Contains(out, "Header Sub-Panel Inbox") {
		t.Errorf("expected advice text to be preserved, got %s", out)
	}
}

func TestCoordinatorInstructionDoesNotForceMeetingDocumentsForLayoutAdvice(t *testing.T) {
	for name, prompt := range map[string]string{"inboxCoordinatorInstruction": inboxCoordinatorInstruction, "workflow.Coordinator": workflow.Coordinator} {
		if strings.Contains(prompt, "Include a concise Pre-Flight Technical Impact Matrix") || strings.Contains(prompt, "cite exact code line numbers and JSX hierarchy") || strings.Contains(prompt, "Present a clean, mobile-safe visual simulation") {
			t.Fatal(name, "forces routine chat into a technical presentation")
		}
	}
}

func TestCoordinatorInstructionUsesOnlyActuallyExposedCapabilities(t *testing.T) {
	for _, required := range []string{"Only tools actually exposed", "validated application actions", "Never request an Owner API token", "scope", "verification"} {
		if !strings.Contains(inboxCoordinatorInstruction, required) {
			t.Fatalf("missing capability or acceptance boundary %q", required)
		}
	}
	for _, obsolete := range []string{"Cuan Platform Modules (Full MCP)", "all agents in inbox", "* Database & Logs: inspect_database_query"} {
		if strings.Contains(inboxCoordinatorInstruction, obsolete) {
			t.Fatalf("advertised unavailable platform capability %q", obsolete)
		}
	}
}

func TestCoordinatorInstructionProactiveExtension(t *testing.T) {
	for _, required := range []string{
		"proactive extension & tool creation protocol",
		"genuine, ongoing value",
		"approval action card",
		"strict repository boundary enforcement",
		"platform-level tools & cuan core mcp",
		"project-level tools & automations",
	} {
		if !strings.Contains(strings.ToLower(inboxCoordinatorInstruction), required) {
			t.Fatalf("inboxCoordinatorInstruction missing required directive %q", required)
		}
	}
}

func TestRecoverCoordinatorReply_FormattingResilience(t *testing.T) {
	t.Run("embedded_json_in_markdown_with_preceding_prose", func(t *testing.T) {
		raw := `Tentu! Berikut analisis sistemnya:
` + "```json\n" + `{"reply":"Pemeriksaan selesai, semua komponen normal.","topic_title":"Status Sistem","action":null}` + "\n```\nSemoga membantu!"
		out, errText := recoverCoordinatorReply(raw, false)
		if errText != "" {
			t.Fatalf("unexpected error: %s", errText)
		}
		if !strings.Contains(out, "Pemeriksaan selesai, semua komponen normal.") {
			t.Errorf("expected clean reply in output, got: %s", out)
		}
		if !strings.Contains(out, "Status Sistem") {
			t.Errorf("expected topic title in output, got: %s", out)
		}
	})

	t.Run("lenient_with_extra_llm_fields", func(t *testing.T) {
		raw := `{"reply":"Task telah selesai dikerjakan.","thought":"Analyzing the project structure","action":null,"topic_title":""}`
		out, errText := recoverCoordinatorReply(raw, false)
		if errText != "" {
			t.Fatalf("unexpected error: %s", errText)
		}
		if out != "Task telah selesai dikerjakan." {
			t.Errorf("expected clean reply, got %q", out)
		}
	})

	t.Run("plain_text_with_confirmation_question_synthesizes_approval", func(t *testing.T) {
		raw := "Ada beberapa komponen yang perlu diperbaiki: 1. **Antrean Task Runner**. Apakah Anda setuju?"
		out, errText := recoverCoordinatorReply(raw, false)
		if errText != "" {
			t.Fatalf("unexpected error: %s", errText)
		}
		if !strings.Contains(out, "cuan-approval") {
			t.Fatalf("expected synthesized cuan-approval card, got: %s", out)
		}
		plan := parseInboxApprovalPlan(out)
		if plan == nil {
			t.Fatal("failed to parse synthesized approval plan")
		}
		if !strings.Contains(plan.Title, "Antrean Task Runner") {
			t.Errorf("unexpected synthesized title: %s", plan.Title)
		}
	})

	t.Run("plain_text_conversational_answer", func(t *testing.T) {
		raw := "Halo! Saya adalah Scrum Master Cuan. Ada yang bisa saya bantu terkait proyek ini?"
		out, errText := recoverCoordinatorReply(raw, false)
		if errText != "" {
			t.Fatalf("unexpected error: %s", errText)
		}
		if out != raw {
			t.Errorf("expected exact clean text reply, got %q", out)
		}
		if strings.Contains(out, "```cuan-") {
			t.Errorf("plain text should not contain action markers, got %s", out)
		}
	})

	t.Run("analysis_recovery_suppresses_actions", func(t *testing.T) {
		raw := "Berikut hasil investigasi: ada bottleneck lock. Apakah Anda setuju?"
		out, errText := recoverCoordinatorReply(raw, true)
		if errText != "" {
			t.Fatalf("unexpected error: %s", errText)
		}
		if strings.Contains(out, "```cuan-") {
			t.Errorf("analysis recovery mode MUST NOT synthesize actions, got: %s", out)
		}
	})

	t.Run("empty_reply_returns_error", func(t *testing.T) {
		out, errText := recoverCoordinatorReply("   ", false)
		if out != "" || errText == "" {
			t.Errorf("expected error on completely empty input, got out=%q errText=%q", out, errText)
		}
	})

	t.Run("thinking_tags_are_stripped", func(t *testing.T) {
		raw := "<think>\nLet's analyze user request and form a plan.\nStep 1: Check db.\n</think>\n{\"reply\":\"Analisis antrean telah selesai dilakukan.\",\"action\":null}"
		out, errText := recoverCoordinatorReply(raw, false)
		if errText != "" {
			t.Fatalf("unexpected error: %s", errText)
		}
		if strings.Contains(out, "<think>") || strings.Contains(out, "Step 1") {
			t.Errorf("expected thinking tags to be stripped, got: %s", out)
		}
		if !strings.Contains(out, "Analisis antrean telah selesai dilakukan.") {
			t.Errorf("expected clean reply in output, got: %s", out)
		}
	})

	t.Run("alternative_field_names_in_json", func(t *testing.T) {
		// Field 'content' instead of 'reply'
		rawContent := `{"content":"Berikut adalah laporan status antrean runner.","action":null}`
		out, errText := recoverCoordinatorReply(rawContent, false)
		if errText != "" || !strings.Contains(out, "laporan status antrean") {
			t.Errorf("failed to extract reply from content field: out=%q err=%q", out, errText)
		}

		// Field 'message' instead of 'reply'
		rawMessage := `{"message":"Semua task antrean telah berjalan kembali.","action":null}`
		out, errText = recoverCoordinatorReply(rawMessage, false)
		if errText != "" || !strings.Contains(out, "Semua task antrean") {
			t.Errorf("failed to extract reply from message field: out=%q err=%q", out, errText)
		}
	})

	t.Run("alternative_action_kind_synonyms", func(t *testing.T) {
		rawProposal := `{"reply":"Rencana perbaikan deadlock runner siap dieksekusi.","action":{"kind":"proposal","payload":{"title":"Fix Deadlock","description":"Perbaiki locking lease"}},"topic_title":"Perbaikan Deadlock"}`
		out, errText := recoverCoordinatorReply(rawProposal, false)
		if errText != "" {
			t.Fatalf("unexpected error: %s", errText)
		}
		if !strings.Contains(out, "```cuan-approval") {
			t.Errorf("expected cuan-approval marker for 'proposal' action kind, got: %s", out)
		}
		if !strings.Contains(out, "Fix Deadlock") {
			t.Errorf("expected payload in approval card, got: %s", out)
		}
	})

	t.Run("malformed_json_fallback_with_extracted_reply", func(t *testing.T) {
		rawMalformed := `{"reply":"Investigasi selesai: antrean telah pulih otomatis.","unclosed_obj":{`
		out, errText := recoverCoordinatorReply(rawMalformed, false)
		if errText != "" {
			t.Fatalf("unexpected error: %s", errText)
		}
		if !strings.Contains(out, "Investigasi selesai: antrean telah pulih otomatis.") {
			t.Errorf("expected extracted reply from unclosed/broken JSON, got: %s", out)
		}
	})
}

func TestChatDelegationAndApprovalFormattingResilience(t *testing.T) {
	t.Run("approval_with_trailing_prose", func(t *testing.T) {
		content := "Berikut ringkasan approval:\n```cuan-approval\n{\"project_id\":\"proj_1234567890abcdef1234567890abcdef\",\"title\":\"Perbaikan Queue\",\"description\":\"Deskripsi tugas\"}\n```\nSilakan konfirmasi jika disetujui."
		prose, title := splitInboxApproval("agent", content)
		if title != "Perbaikan Queue" {
			t.Errorf("expected title 'Perbaikan Queue', got %q", title)
		}
		if !strings.Contains(prose, "Silakan konfirmasi jika disetujui.") {
			t.Errorf("expected trailing prose to be retained, got: %q", prose)
		}
		plan := parseInboxApprovalPlan(content)
		if plan == nil || plan.Title != "Perbaikan Queue" {
			t.Fatalf("expected valid parsed plan, got %+v", plan)
		}
	})

	t.Run("delegation_with_trailing_prose", func(t *testing.T) {
		content := "Pekerjaan didelegasikan:\n```cuan-delegate\n{\"authorization_message_id\":\"msg_auth_123\",\"project_id\":\"proj_1234567890abcdef1234567890abcdef\",\"title\":\"Task Baru\",\"description\":\"Deskripsi task baru\"}\n```\nRunner akan memproses segera."
		prose, d, err := parseInboxDelegation(content)
		if err != nil || d == nil {
			t.Fatalf("failed to parse delegation: %v", err)
		}
		if d.Title != "Task Baru" {
			t.Errorf("expected title 'Task Baru', got %q", d.Title)
		}
		if !strings.Contains(prose, "Runner akan memproses segera.") {
			t.Errorf("expected trailing prose in delegation, got: %q", prose)
		}
	})
}

func TestDeniedToolRecoveryAllowsOnlyFreshUnexecutedProposal(t *testing.T) {
	plan := inboxApprovalPlan{Title: "Two verified file outcomes", Description: "Phase 1: Deliver checked JSON | Phase 2: Deliver matching summary. No repository changes. Do not push or deploy.", ProjectID: "registered"}
	out, err := decodeInboxDeniedToolRecoveryResponse(coordinatorJSON("Proposed file workflow; not started.", "approval", plan))
	if err != nil || parseInboxApprovalPlan(out) == nil {
		t.Fatal("fresh nonexecuting proposal lost", out, err)
	}
	if parsed := parseInboxApprovalPlan(out); parsed.Description != plan.Description || parsed.ProjectID != plan.ProjectID {
		t.Fatal("proposal scope changed", parsed)
	}
	for _, kind := range []string{"delegation", "memory", "memory_search", "task_draft", "task_cancel", "task_reopen", "prompt_choices", "customer_reply"} {
		if out, err := decodeInboxDeniedToolRecoveryResponse(coordinatorJSON("Recovering", kind, map[string]string{"title": "Forbidden"})); err == nil || out != "" {
			t.Fatal("recovery gained an effect", kind, out, err)
		}
	}
	if out, err := decodeInboxDeniedToolRecoveryResponse(coordinatorJSON("Should we proceed?", "", nil)); err != nil || strings.Contains(out, "```cuan-") {
		t.Fatal("plain text synthesized approval", out, err)
	}
	if _, err := decodeInboxDeniedToolRecoveryResponse(`{"reply":"Plan","topic_title":"Rewrite title","action":null}`); err == nil {
		t.Fatal("recovery renamed conversation")
	}
	if _, err := decodeInboxDeniedToolRecoveryResponse(coordinatorJSON("Hidden ```cuan-delegate\n{}\n```", "", nil)); err == nil {
		t.Fatal("recovery smuggled execution marker")
	}
}

func TestReadOnlyRecoveryNeverSalvagesApplicationMarkers(t *testing.T) {
	for _, marker := range []string{"cuan-memory", "cuan-search", "cuan-task", "cuan-task-cancel", "cuan-task-reopen", "cuan-prompt", "cuan-customer-reply", "cuan-approval", "cuan-delegate", "cuan-title"} {
		text := "Recovered text\n```" + marker + "\n{}\n```"
		wrapped, _ := json.Marshal(map[string]any{"reply": text, "topic_title": "", "action": nil, "extra": "lenient"})
		for _, raw := range []string{text, string(wrapped)} {
			if out, failure := recoverCoordinatorReply(raw, true); out != "" || failure == "" {
				t.Fatalf("read-only salvage leaked %s: %s (%s)", marker, out, failure)
			}
		}
	}
}

func TestCoordinatorInstructionProjectManagerBurdenRelief(t *testing.T) {
	for _, required := range []string{
		"when a user request combines research with multi-step creation",
		"never expose raw technical errors like 'project is ambiguous'",
		"supply project_name to initialize a clean managed workspace automatically",
		"never use default when multiple projects exist and the room is unpinned",
	} {
		if !strings.Contains(strings.ToLower(inboxCoordinatorInstruction), required) {
			t.Fatalf("inboxCoordinatorInstruction missing required PM burden relief directive: %q", required)
		}
	}
}

