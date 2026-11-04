package diagnostics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func typecheckFixture(t *testing.T, script string) (string, string) {
	t.Helper()
	root := t.TempDir()
	web := filepath.Join(root, "web")
	if err := os.MkdirAll(filepath.Join(web, "node_modules", ".bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "tsconfig.json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "node_modules", ".bin", "tsc"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return root, web
}
func TestTypecheckOutcomeAcrossEngineCLIAndMCP(t *testing.T) {
	for _, scenario := range []string{"passed", "failed", "not_run", "unsupported", "incomplete"} {
		t.Run(scenario, func(t *testing.T) {
			script := "#!/bin/sh\nexit 0\n"
			if scenario == "failed" {
				script = "#!/bin/sh\necho 'index.ts(1,1): error TS2322: fixture type error' >&2\nexit 2\n"
			}
			if scenario == "not_run" {
				script = "#!/missing-cuan-fixture-interpreter\n"
			}
			if scenario == "incomplete" {
				script = "#!/bin/sh\nsleep 10 &\nexit 0\n"
			}
			root, checked := typecheckFixture(t, script)
			want := scenario
			if scenario == "incomplete" {
				want = "failed"
			}
			if scenario == "unsupported" {
				root = t.TempDir()
				checked = root
				want = "not_run"
			}
			result, err := InspectTypecheck(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != want || result.Success != (want == "passed") || result.Directory != checked {
				t.Fatalf("incorrect check outcome: %+v", result)
			}
			if want != "passed" && result.Reason == "" {
				t.Fatal("non-passing result lacks an explanation")
			}
			if scenario == "failed" && result.IssueCount != 1 {
				t.Fatal("type error output lost", result)
			}
			for name, run := range map[string]func([]string, *bytes.Buffer, *bytes.Buffer) int{
				"repo": func(args []string, out, errOut *bytes.Buffer) int { return RunRepoCLI(args, out, errOut) },
				"diag": func(args []string, out, errOut *bytes.Buffer) int { return RunDiagCLI(args, out, errOut) },
			} {
				var out, errOut bytes.Buffer
				code := run([]string{"typecheck", "--path", root, "--json"}, &out, &errOut)
				var envelope DiagnosticEnvelope
				if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
					t.Fatal(err, out.String(), errOut.String())
				}
				if envelope.Success != (want == "passed") || (code == 0) != (want == "passed") {
					t.Fatalf("%s CLI claimed an incorrect success: code=%d %+v", name, code, envelope)
				}
				out.Reset()
				errOut.Reset()
				code = run([]string{"typecheck", "--path", root}, &out, &errOut)
				if (code == 0) != (want == "passed") || (want != "passed" && strings.Contains(out.String(), "PASSED")) {
					t.Fatal("human CLI misrepresented verification", out.String())
				}
				if want == "not_run" && !strings.Contains(out.String(), "NOT RUN") {
					t.Fatal("unsupported check shown as passed/failed instead of not run", out.String())
				}
			}
			request, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "inspect_typecheck", "arguments": map[string]string{"path": root}}})
			var out, errOut bytes.Buffer
			if RunMCP(bytes.NewReader(append(request, '\n')), &out, &errOut) != 0 {
				t.Fatal(errOut.String())
			}
			var response struct {
				Result struct {
					IsError bool `json:"isError"`
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
				} `json:"result"`
			}
			if err := json.Unmarshal(out.Bytes(), &response); err != nil {
				t.Fatal(err, out.String())
			}
			if response.Result.IsError != (want != "passed") || len(response.Result.Content) != 1 {
				t.Fatal("MCP did not expose the non-passing outcome", out.String())
			}
			var mcpResult TypecheckResult
			if err := json.Unmarshal([]byte(response.Result.Content[0].Text), &mcpResult); err != nil || mcpResult.Status != want {
				t.Fatal("MCP lost structured typecheck status", err, out.String())
			}
		})
	}
}

func TestTypecheckTimeoutDoesNotHangOnInheritedOutputPipe(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	script := "#!/bin/sh\necho $$ > '" + pidFile + "'\nsleep 10 &\nwait\n"
	root, _ := typecheckFixture(t, script)
	defer func() {
		data, _ := os.ReadFile(pidFile)
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if pid > 0 {
			_ = syscall.Kill(-pid, syscall.SIGKILL)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	done := make(chan *TypecheckResult, 1)
	go func() { result, _ := InspectTypecheck(ctx, root); done <- result }()
	select {
	case result := <-done:
		if result == nil || result.Success || result.Status != "timed_out" || result.Reason == "" {
			t.Fatalf("timeout masqueraded as verification: %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("tool exceeded timeout because a child kept stdout open")
	}
}

func TestDiagnosticOutputHelper(t *testing.T) {
	if os.Getenv("CUAN_DIAGNOSTIC_OUTPUT_FIXTURE") != "1" {
		return
	}
	_, err := os.Stdout.Write(bytes.Repeat([]byte("x"), 2*1024*1024))
	if err != nil {
		os.Exit(23)
	}
	os.Exit(0)
}
func TestBoundedDiagnosticCommandDrainsOutputAfterRetentionLimit(t *testing.T) {
	t.Setenv("CUAN_DIAGNOSTIC_OUTPUT_FIXTURE", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	output, truncated, err := runBoundedCommand(context.Background(), 2*time.Second, t.TempDir(), executable, "-test.run=^TestDiagnosticOutputHelper$")
	if err != nil || !truncated || len(output) != MaxOutputBytes {
		t.Fatalf("output cap blocked or failed a healthy command: bytes=%d truncated=%t err=%v", len(output), truncated, err)
	}
}
func TestBoundedDiagnosticCommandReturnsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := runBoundedCommand(ctx, time.Second, t.TempDir(), "/bin/sh", "-c", "exit 0")
	if !errors.Is(err, context.Canceled) {
		t.Fatal("lost caller cancellation", err)
	}
}

func TestTypecheckCanceledBeforeExecutionNeverClaimsPass(t *testing.T) {
	root, _ := typecheckFixture(t, "#!/bin/sh\nexit 0\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := InspectTypecheck(ctx, root)
	if err != nil || result == nil || result.Success || result.Status != "canceled" {
		t.Fatalf("cancellation lost: %+v %v", result, err)
	}
}

func TestGoTypecheckReportsActualExecutionFailure(t *testing.T) {
	root := t.TempDir()
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nif [ \"$1\" != vet ] || [ \"$2\" != ./... ]; then exit 23; fi\necho 'fixture vet diagnostic' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	result, err := InspectTypecheck(context.Background(), root)
	if err != nil || result.Success || result.Status != "failed" || result.Tool != "go vet ./..." || result.Directory != root || result.IssueCount != 1 {
		t.Fatalf("Go checker outcome lost: %+v %v", result, err)
	}
}
