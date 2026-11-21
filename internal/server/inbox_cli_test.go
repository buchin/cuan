package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
)

func TestDecodeInboxAgyResult(t *testing.T) {
	cases := []struct{ name, raw, stderr, want, reason, contains string }{
		{"success", `{"status":"SUCCESS","response":" **Useful answer** ","usage":{"private":"never expose"}}`, "", "**Useful answer**", "success", ""},
		{"empty stdout", "", "429 RESOURCE_EXHAUSTED private-token", "", "empty_output", "quota"},
		{"exit zero error", `{"status":"ERROR","response":"partial private text","error":{"message":"429 RESOURCE_EXHAUSTED private-token"}}`, "", "", "provider_failure", "quota"},
		{"location restriction", `{"status":"ERROR","message":"FAILED_PRECONDITION (code 400): User location is not supported for the API use. private-token"}`, "", "", "provider_failure", "location restriction"},
		{"auth", `{"status":"ERROR","message":"invalid_grant private-token"}`, "", "", "provider_failure", "sign-in"},
		{"timeout", `{"status":"TIMEOUT","response":"partial private-token"}`, "", "", "provider_timeout", "timed out"},
		{"denied preamble", `{"status":"SUCCESS","response":"Aku cek dulu transkrip percakapan kita.","denied_actions":[{"action":"private-command"}]}`, "", "", "tool_permission_denied", "tool permission was denied"},
		{"denied empty", `{"status":"SUCCESS","response":"","denied_actions":[{"action":"private-command"}]}`, "private-stderr", "", "tool_permission_denied", "tool permission was denied"},
		{"denied with explanation", `{"status":"SUCCESS","response":"Tool ditolak, berikut analisis dari README.","denied_actions":[{"action":"private-command"}]}`, "", "", "tool_permission_denied", "tool permission was denied"},
		{"empty response", `{"status":"SUCCESS","response":" "}`, "", "", "empty_response", "returned no reply"},
		{"agy print timeout", `{"status":"SUCCESS","response":""}`, "[agy] print timeout after 45s with turn in progress; returning partial output", "", "provider_timeout", "timed out"},
		{"missing status", `{"response":"private-token"}`, "", "", "provider_failure", "could not reply"},
		{"malformed", "private-token", "", "", "invalid_result", "Could not read"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, failure, reason := decodeInboxAgyResult(c.raw, c.stderr)
			if out != c.want || reason != c.reason || !strings.Contains(failure, c.contains) {
				t.Fatalf("got output=%q failure=%q reason=%q", out, failure, reason)
			}
			if strings.Contains(out+failure, "private") {
				t.Fatal("diagnostic leaked")
			}
		})
	}
}
func TestInboxCLIExitZeroFailureAndTrace(t *testing.T) {
	home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\nprintf '%s' '{\"event\":\"result\",\"result\":{\"status\":\"ERROR\",\"message\":\"RESOURCE_EXHAUSTED private-token\"}}'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	t.Setenv("CUAN_CHAT_TOOL_BUDGET", "4")
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	steps := map[string]string{}
	var mu sync.Mutex
	out, failure := s.executeInboxCLI(context.Background(), "fixture", "agy", "diagnostic", home, workspace, func(kind, text string) { mu.Lock(); steps[kind] = text; mu.Unlock() })
	if out != "" || !strings.Contains(failure, "quota") {
		t.Fatal("exit0 failure accepted", out, failure)
	}
	if !strings.Contains(steps["provider_result"], "exit 0") || steps["result_validation"] != "The provider reported a failure." {
		t.Fatal("missing safe trace", steps)
	}
	for _, value := range steps {
		if strings.Contains(value, "private-token") {
			t.Fatal("trace leaked diagnostic")
		}
	}
}

// Explicit opt-in integration check; never part of an ordinary test run.
func TestLiveInboxAgyDiagnostic(t *testing.T) {
	home := os.Getenv("CUAN_DIAGNOSTIC_PROFILE")
	if home == "" {
		t.Skip("explicit isolated agent profile diagnostic required")
	}
	if !strings.Contains(home, "/.local/share/cuan/accounts/acct_") || !strings.HasSuffix(home, "/home") {
		t.Fatal("dedicated account profile required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	steps := map[string]string{}
	var mu sync.Mutex
	out, failure := s.executeInboxCLI(ctx, "pin2", "agy", "Gunakan tool baca README.md di workspace project ini, lalu balas tepat CUAN_DIAGNOSTIC_OK. Jangan mengubah file atau membaca file lain.", home, "/opt/cuan/data/projects/default", func(kind, text string) { mu.Lock(); steps[kind] = text; mu.Unlock() })
	if failure != "" || strings.TrimSpace(out) != "CUAN_DIAGNOSTIC_OK" {
		t.Fatalf("live diagnostic not verified; safe failure=%s", failure)
	}
	if !strings.Contains(steps["provider_result"], "exit 0") || steps["result_validation"] != "Provider reply validated." {
		t.Fatal("live trace missing")
	}
	if steps["runtime_tool_started"] == "" || steps["runtime_tool_finished"] == "" {
		t.Fatal("actual tool trace missing")
	}
	t.Log("Actual Cuan executeInboxCLI + bound profile + project CWD: structured reply validated, actual read tool start/finish and exit0/duration trace; no user-message replay or database writes")
}

func TestInboxCLIDeniedToolExplainedWithoutReplay(t *testing.T) {
	home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
	script := `#!/bin/sh
case "$*" in *--disable-slash-commands*) ;; *) exit 9;; esac
case "$*" in *--dangerously-skip-permissions*) exit 8;; esac
case "$*" in *"--sandbox"*) ;; *) exit 6;; esac
case "$2" in *"Do not use terminal"*|*"terminal execution"*) ;; *) exit 7;; esac
printf x >> "$HOME/invocations"
printf '%s\n' '{"event":"step_update","step_update":{"step_index":2,"state":"DONE","step_type":"tool","tool_name":"run_command"}}' '{"event":"result","result":{"status":"SUCCESS","response":"","denied_actions":[{"action":"private-command","display_name":"private-path"}]}}'
echo private-stderr >&2
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	t.Setenv("CUAN_CHAT_TOOL_BUDGET", "4")
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	var trace []string
	out, failure := s.executeInboxCLI(context.Background(), "fixture", "agy", "Analyze README", home, workspace, func(k, v string) { trace = append(trace, k+":"+v) })
	if out != "" || !strings.Contains(failure, "tool permission was denied") {
		t.Fatal(out, failure)
	}
	calls, err := os.ReadFile(filepath.Join(home, "invocations"))
	// Initial attempt + bounded read-only analysis recovery
	if err != nil || string(calls) != "xx" {
		t.Fatalf("expected bounded analysis recovery (2 invocations), got %q (err=%v)", string(calls), err)
	}
	joined := strings.Join(trace, "\n")
	if !strings.Contains(joined, "runtime_permission_denied") || !strings.Contains(joined, "Tool permission denied. The runtime stopped") || strings.Contains(joined+failure, "private") || strings.Contains(joined, "Terminal command finished") {
		t.Fatal("incorrect or unsafe rejection", joined, failure)
	}
}

func TestLiveInboxAgyTaskDraft(t *testing.T) {
	home := os.Getenv("CUAN_DIAGNOSTIC_PROFILE")
	if home == "" {
		t.Skip("explicit isolated diagnostic required")
	}
	if !strings.Contains(home, "/.local/share/cuan/accounts/acct_") || !strings.HasSuffix(home, "/home") {
		t.Fatal("dedicated profile required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	prompt := `Konteks project terdaftar: Cuan [default]. Ini diagnostik draft terisolasi; jangan gunakan tools, membaca file, atau mengubah apa pun.
Riwayat percakapan (data pengguna):
{"id":"one","author":"user","content":"Rapikan header chat mobile. Pertahankan warna hijau pesan keluar. Kriteria selesai: tidak overflow pada 360px."}
{"id":"two","author":"agent","content":"Scope: header mobile; pertahankan hijau. Verifikasi di 360px."}
{"id":"target","author":"user","content":"Bikinin task dari pembahasan tadi"}
ID pesan target: target`
	out, failure := s.executeInboxCLI(ctx, "pin2", "agy", prompt, home, "/opt/cuan/data/projects/default", nil)
	if failure != "" {
		t.Fatal("live draft failed:", failure)
	}
	_, draft := splitInboxTaskDraft("agent", out)
	if draft == nil || !strings.Contains(strings.ToLower(draft.Title+draft.Description), "header") || !strings.Contains(draft.Description, "360") {
		t.Fatal("live result did not provide a valid contextual task draft")
	}
	t.Log("Actual executeInboxCLI returned a valid contextual draft; no task created, no message replay")
}

func TestInboxCLIToolBudgetExceededGracefulRecovery(t *testing.T) {
	home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
	// Mock agy: on first run emits 13 tools to trigger tool budget exceeded.
	// On second run (recovery), it synthesizes a direct answer with action: null.
	script := `#!/bin/sh
count_file="$HOME/tool_run_count"
count=0
if [ -f "$count_file" ]; then
    count=$(cat "$count_file")
fi
count=$((count + 1))
echo "$count" > "$count_file"

if [ "$count" -eq 1 ]; then
    printf '%s\n' '{"event":"step_update","step_update":{"step_index":0,"state":"DONE","step_type":"tool","tool_name":"web_fetch","tool_info":{"parameters":{"url":"https://example.com"},"output":"public-fixture-observed"}}}'
    # First invocation: emit 13 tool calls to exceed 12 tool budget
    i=1
    while [ "$i" -le 13 ]; do
        printf '{"event":"step_update","step_update":{"step_index":%d,"state":"ACTIVE","step_type":"tool","tool_name":"read_file"}}\n' "$i"
        printf '{"event":"step_update","step_update":{"step_index":%d,"state":"DONE","step_type":"tool","tool_name":"read_file"}}\n' "$i"
        i=$((i + 1))
    done
    # Sleep to allow cancellation
    sleep 2
    exit 0
else
    # Recovery must receive the completed observation, not discard it.
    case "$*" in *public-fixture-observed*) ;; *) exit 9;; esac
    # Second invocation (recovery): return direct synthesis with action: null
    printf '%s\n' '{"event":"result","result":{"status":"SUCCESS","response":"{\"reply\":\"Analisis komponen filter tasks telah diperiksa sebagian dari konteks yang ada.\",\"topic_title\":\"\",\"action\":null}"}}'
    exit 0
fi
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	t.Setenv("CUAN_CHAT_TOOL_BUDGET", "2")
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	var trace []string
	safe := false
	out, failure := s.executeInboxCLIWithSafety(context.WithValue(context.Background(), inboxRequestTraceKey{}, inboxRequestTrace{RequestID: "public-research-fixture"}), "Coordinator", "agy", "Quick filter perlu perbaikan?", home, workspace, func(k, v string) { trace = append(trace, k+":"+v) }, &safe)

	if failure != "" {
		t.Fatalf("expected graceful recovery without fatal failure, got failure: %q", failure)
	}
	if out == "" || !strings.Contains(out, "Analisis komponen filter tasks") {
		t.Fatalf("expected synthesized reply in output, got: %q", out)
	}
	if strings.Contains(out, "```cuan-") {
		t.Fatalf("expected action: null with no action markers, got: %q", out)
	}

	joined := strings.Join(trace, "\n")
	if !strings.Contains(joined, "runtime_tool_budget_exceeded") {
		t.Fatalf("expected runtime_tool_budget_exceeded trace, got: %s", joined)
	}
	if !strings.Contains(joined, "response_recovery") {
		t.Fatalf("expected response_recovery trace, got: %s", joined)
	}
}

func TestInboxCLIToolBudgetExceededDoesNotInventSuccessWhenRecoveryFails(t *testing.T) {
	home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
	// Both attempts fail. Keep the failure visible; no fabricated findings.
	script := `#!/bin/sh
i=1
while [ "$i" -le 13 ]; do
    printf '{"event":"step_update","step_update":{"step_index":%d,"state":"ACTIVE","step_type":"tool","tool_name":"read_file"}}\n' "$i"
    printf '{"event":"step_update","step_update":{"step_index":%d,"state":"DONE","step_type":"tool","tool_name":"read_file"}}\n' "$i"
    i=$((i + 1))
done
sleep 2
exit 0
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	t.Setenv("CUAN_CHAT_TOOL_BUDGET", "2")
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	var trace []string
	safe := false
	out, failure := s.executeInboxCLIWithSafety(context.Background(), "Coordinator", "agy", "Quick filter perlu perbaikan?", home, workspace, func(k, v string) { trace = append(trace, k+":"+v) }, &safe)

	if failure == "" || out != "" {
		t.Fatalf("failed recovery became a successful reply: out=%q failure=%q", out, failure)
	}
}

func TestInboxCLIAdaptiveBatchingCodeInspection(t *testing.T) {
	home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
	// Explicit operator overrides remain supported; large chat sweeps are no
	// longer the default. Deep investigation normally belongs in a task.
	script := `#!/bin/sh
i=1
while [ "$i" -le 50 ]; do
    printf '{"event":"step_update","step_update":{"step_index":%d,"state":"ACTIVE","step_type":"tool","tool_name":"read_file"}}\n' "$i"
    printf '{"event":"step_update","step_update":{"step_index":%d,"state":"DONE","step_type":"tool","tool_name":"read_file"}}\n' "$i"
    i=$((i + 1))
done
printf '%s\n' '{"event":"result","result":{"status":"SUCCESS","response":"{\"reply\":\"Analisis 50 berkas kode telah selesai secara menyeluruh tanpa kendala.\",\"topic_title\":\"\",\"action\":null}"}}'
exit 0
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	t.Setenv("CUAN_CHAT_TOOL_BUDGET", "50")
	t.Setenv("CUAN_CHAT_INSPECTION_BATCH_FACTOR", "5")
	t.Setenv("INBOX_TOOL_BUDGET", "")
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	var trace []string
	safe := false
	out, failure := s.executeInboxCLIWithSafety(context.Background(), "Coordinator", "agy", "Inspeksi seluruh modul kode di workspace", home, workspace, func(k, v string) { trace = append(trace, k+":"+v) }, &safe)

	if failure != "" {
		t.Fatalf("expected 50 inspection tool calls to succeed with adaptive batching, got failure: %q", failure)
	}
	if out == "" || !strings.Contains(out, "Analisis 50 berkas kode telah selesai") {
		t.Fatalf("expected successful analysis reply, got: %q", out)
	}
	joined := strings.Join(trace, "\n")
	if strings.Contains(joined, "runtime_tool_budget_exceeded") {
		t.Fatalf("did not expect tool budget exceeded during normal 50-file inspection, got trace: %s", joined)
	}
	if strings.Contains(joined, "response_recovery") {
		t.Fatalf("did not expect response recovery during normal 50-file inspection, got trace: %s", joined)
	}
}

func TestInboxChatDefaultBudgetStopsReadLoop(t *testing.T) {
	t.Setenv("CUAN_CHAT_TOOL_BUDGET", "")
	t.Setenv("INBOX_TOOL_BUDGET", "")
	t.Setenv("CUAN_CHAT_INSPECTION_BATCH_FACTOR", "")
	canceled := false
	stream := &inboxRuntimeStream{maxToolCalls: getChatToolBudget(), inspectionBatchFactor: getChatInspectionBatchFactor(), adaptiveBatching: true, cancel: func() { canceled = true }}
	for i := 0; i < 12; i++ {
		stream.Write([]byte(fmt.Sprintf(`{"event":"step_update","step_update":{"step_index":%d,"state":"DONE","step_type":"tool","tool_name":"read_file"}}`+"\n", i)))
	}
	if canceled {
		t.Fatal("context inspection stopped before configured budget")
	}
	stream.Write([]byte(`{"event":"step_update","step_update":{"step_index":13,"state":"ACTIVE","step_type":"tool","tool_name":"read_file"}}` + "\n"))
	if !canceled || !stream.toolBudgetExceeded {
		t.Fatal("default chat budget still allows read loops")
	}
}

func TestInboxBudgetRecoveryDoesNotLoseUnsafeChildEvidence(t *testing.T) {
	home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
	script := `#!/bin/sh
if [ -f "$HOME/first-call" ]; then
 printf '%s\n' '{"event":"step_update","step_update":{"step_index":1,"state":"ACTIVE","step_type":"tool","tool_name":"run_command"}}'
else
 touch "$HOME/first-call"
 printf '%s\n' '{"event":"step_update","step_update":{"step_index":1,"state":"DONE","step_type":"tool","tool_name":"read_file"}}' '{"event":"step_update","step_update":{"step_index":2,"state":"ACTIVE","step_type":"tool","tool_name":"read_file"}}'
fi
sleep 2
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	t.Setenv("CUAN_CHAT_TOOL_BUDGET", "1")
	t.Setenv("CUAN_CHAT_INSPECTION_BATCH_FACTOR", "1")
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	safe := true
	out, failure := s.executeInboxCLIWithSafety(context.Background(), "Coordinator", "agy", "Read-only question", home, workspace, nil, &safe)
	if failure == "" || out != "" || safe {
		t.Fatalf("unsafe child hidden: out=%q failure=%q safe=%v", out, failure, safe)
	}
}

func TestInboxChatDoesNotStartProviderWhenProfileRulesCannotBePrepared(t *testing.T) {
	home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
	settingsDir := filepath.Join(home, ".gemini", "antigravity-cli")
	if err := os.MkdirAll(settingsDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(`{"permissions":{"deny":"invalid"}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\ntouch \"$HOME/provider-started\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	safe := true
	out, failure := s.executeInboxCLIWithSafety(context.Background(), "Coordinator", "agy", "Read a project file", home, workspace, nil, &safe)
	if out != "" || !strings.Contains(failure, "permissions could not be prepared") || safe {
		t.Fatalf("permission failure not retained: out=%q failure=%q safe=%v", out, failure, safe)
	}
	if _, err := os.Stat(filepath.Join(home, "provider-started")); !os.IsNotExist(err) {
		t.Fatal("provider started without profile deny rules")
	}
}

func TestInboxProviderSignalIsRecordedWithoutInventingCompletion(t *testing.T) {
	home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte("#!/bin/sh\nkill -USR1 $$\n"), 0700); err != nil {
		t.Fatal(err)
	}
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	var result string
	out, failure := s.executeInboxCLI(context.Background(), "Coordinator", "agy", "Read-only fixture", home, workspace, func(kind, text string) {
		if kind == "provider_result" {
			result = text
		}
	})
	wantSignal := fmt.Sprintf("signal %d", syscall.SIGUSR1)
	if out != "" || failure == "" || !strings.Contains(result, wantSignal) {
		t.Fatalf("missing process signal: out=%q failure=%q result=%q", out, failure, result)
	}
}

func TestDeniedToolRecoveryPreservesPlanningWithoutReplayingAction(t *testing.T) {
	home, workspace, bin := t.TempDir(), t.TempDir(), t.TempDir()
	plan := inboxApprovalPlan{Title: "Two verified files", Description: "Phase 1: Deliver validated JSON | Phase 2: Deliver matching summary. No repository changes. Do not push or deploy.", ProjectID: "registered"}
	deniedReply := coordinatorJSON("REJECTED private candidate", "delegation", map[string]string{"title": "Rejected execution", "description": "Do not replay", "project_id": "registered", "authorization_message_id": "rejected"})
	denied, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]any{"status": "SUCCESS", "response": deniedReply, "denied_actions": []map[string]string{{"action": "private-command"}}}})
	fresh, _ := json.Marshal(map[string]any{"event": "result", "result": map[string]string{"status": "SUCCESS", "response": coordinatorJSON("Proposed only; not started.", "approval", plan)}})
	// Fixed fixture JSON has no shell quoting characters or user interpolation.
	script := `#!/bin/sh
printf x >> "$HOME/invocations"
if [ ! -f "$HOME/denied-once" ]; then
 touch "$HOME/denied-once"
 printf '%s\n' '{"event":"step_update","step_update":{"step_index":2,"state":"DONE","step_type":"tool","tool_name":"run_command"}}' '` + string(denied) + `'
else
 case "$2" in *'Continue only the technical analysis'*) exit 40;; esac
 case "$2" in *'SAME original target message'*) ;; *) exit 41;; esac
 case "$2" in *'REJECTED private candidate'*) exit 42;; esac
 case "$*" in *'"enum":["approval"]'*) ;; *) exit 43;; esac
 printf '%s\n' '` + string(fresh) + `'
fi
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	var trace []string
	out, failure := s.executeInboxCLI(context.Background(), "fixture", "agy", "Target message: propose two file outcomes for approval. No repository changes. Do not deploy. Do not start work.", home, workspace, func(kind, text string) { trace = append(trace, kind+":"+text) })
	parsed := parseInboxApprovalPlan(out)
	if failure != "" || parsed == nil || parsed.Description != plan.Description || parsed.ProjectID != plan.ProjectID {
		t.Fatal("fresh proposal lost", out, failure)
	}
	if strings.Contains(out, "cuan-delegate") || strings.Contains(out, "REJECTED") || strings.Contains(strings.Join(trace, "\n"), "private") {
		t.Fatal("rejected action or private detail replayed", out, trace)
	}
	calls, err := os.ReadFile(filepath.Join(home, "invocations"))
	if err != nil || string(calls) != "xx" {
		t.Fatal("unbounded recovery", string(calls), err)
	}
}
