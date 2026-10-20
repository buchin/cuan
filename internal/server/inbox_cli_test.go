package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
case "$*" in *--dangerously-skip-permissions*) ;; *) exit 8;; esac
case "$2" in *"Read-only terminal commands"*) ;; *) exit 7;; esac
printf x >> "$HOME/invocations"
printf '%s\n' '{"event":"step_update","step_update":{"step_index":2,"state":"DONE","step_type":"tool","tool_name":"run_command"}}' '{"event":"result","result":{"status":"SUCCESS","response":"","denied_actions":[{"action":"private-command","display_name":"private-path"}]}}'
echo private-stderr >&2
`
	if err := os.WriteFile(filepath.Join(bin, "agy"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	installInboxSandboxStub(t, bin)
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	s := &Server{cfg: &config.Config{Environment: "staging"}}
	var trace []string
	out, failure := s.executeInboxCLI(context.Background(), "fixture", "agy", "Analyze README", home, workspace, func(k, v string) { trace = append(trace, k+":"+v) })
	if out != "" || !strings.Contains(failure, "tool permission was denied") {
		t.Fatal(out, failure)
	}
	calls, err := os.ReadFile(filepath.Join(home, "invocations"))
	if err != nil || string(calls) != "x" {
		t.Fatal("request replayed or not executed", err)
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
