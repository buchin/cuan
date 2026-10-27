package diagnostics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func createTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v (output: %s)", args, err, string(out))
		}
	}

	run("init", "-b", "main")
	run("config", "user.name", "TestUser")
	run("config", "user.email", "test@example.com")

	f1 := filepath.Join(dir, "README.md")
	if err := os.WriteFile(f1, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial commit")

	return dir
}

func TestValidateSafeArgs(t *testing.T) {
	// 1. Whitelisted subcommands should pass
	for _, sub := range []string{"status", "log", "diff-summary", "branches", "health", "df", "free", "tsc", "deps"} {
		if err := ValidateSafeArgs(sub, nil); err != nil {
			t.Errorf("expected subcommand %q to be allowed, got error: %v", sub, err)
		}
	}

	// 2. Non-whitelisted subcommands should be rejected
	for _, sub := range []string{"push", "commit", "checkout", "rebase", "clone", "reset", "clean", "rm", "exec"} {
		if err := ValidateSafeArgs(sub, nil); err == nil {
			t.Errorf("expected non-whitelisted subcommand %q to be rejected", sub)
		}
	}

	// 3. Shell metacharacters and control operators must be rejected
	dangerousArgs := [][]string{
		{"--path", "; rm -rf /"},
		{"--path", "| cat /etc/passwd"},
		{"--path", "& echo pwn"},
		{"--path", "> /tmp/pwn"},
		{"--path", "< /dev/null"},
		{"--path", "$(whoami)"},
		{"--path", "`id`"},
		{"--path", "dir\nrm -rf"},
	}
	for _, args := range dangerousArgs {
		if err := ValidateSafeArgs("status", args); err == nil {
			t.Errorf("expected dangerous args %v to be rejected", args)
		}
	}

	// 4. Mutation flags must be rejected
	mutationFlags := []string{
		"--exec", "-o", "--output", "--delete", "-d", "-D", "--force", "-f",
		"--hard", "--amend", "--write", "--apply",
	}
	for _, flag := range mutationFlags {
		if err := ValidateSafeArgs("status", []string{flag}); err == nil {
			t.Errorf("expected mutation flag %q to be rejected", flag)
		}
	}

	// 5. Mutation subcommands passed as flags/args must be rejected
	for _, mut := range []string{"commit", "checkout", "switch", "push", "pull", "merge", "rebase", "reset", "clean", "rm", "mv"} {
		if err := ValidateSafeArgs("status", []string{mut}); err == nil {
			t.Errorf("expected mutation subcommand %q to be rejected", mut)
		}
	}
}

func TestInspectGitStatus(t *testing.T) {
	repo := createTestGitRepo(t)
	ctx := context.Background()

	// Clean state
	res, err := InspectGitStatus(ctx, repo)
	if err != nil {
		t.Fatalf("InspectGitStatus failed: %v", err)
	}
	if !res.Clean {
		t.Errorf("expected repo to be clean, got: %+v", res)
	}
	if res.Branch != "main" {
		t.Errorf("expected branch main, got: %s", res.Branch)
	}

	// Add an untracked file
	untracked := filepath.Join(repo, "new_file.txt")
	_ = os.WriteFile(untracked, []byte("hello"), 0644)

	res, err = InspectGitStatus(ctx, repo)
	if err != nil {
		t.Fatalf("InspectGitStatus failed: %v", err)
	}
	if res.Clean {
		t.Errorf("expected repo to be dirty")
	}
	if res.UntrackedCount != 1 {
		t.Errorf("expected 1 untracked file, got: %d", res.UntrackedCount)
	}

	// Stage the file
	cmd := exec.Command("git", "add", "new_file.txt")
	cmd.Dir = repo
	_ = cmd.Run()

	res, err = InspectGitStatus(ctx, repo)
	if err != nil {
		t.Fatalf("InspectGitStatus failed: %v", err)
	}
	if res.StagedCount != 1 {
		t.Errorf("expected 1 staged file, got: %d", res.StagedCount)
	}
}

func TestInspectGitLog(t *testing.T) {
	repo := createTestGitRepo(t)
	ctx := context.Background()

	res, err := InspectGitLog(ctx, repo, 5)
	if err != nil {
		t.Fatalf("InspectGitLog failed: %v", err)
	}
	if res.Count < 1 {
		t.Errorf("expected at least 1 commit, got: %d", res.Count)
	}
	if res.Commits[0].Message != "initial commit" {
		t.Errorf("expected commit message 'initial commit', got: %s", res.Commits[0].Message)
	}
}

func TestInspectGitDiffSummary(t *testing.T) {
	repo := createTestGitRepo(t)
	ctx := context.Background()

	// Clean state
	res, err := InspectGitDiffSummary(ctx, repo)
	if err != nil {
		t.Fatalf("InspectGitDiffSummary failed: %v", err)
	}
	if res.ChangedFiles != 0 {
		t.Errorf("expected 0 changed files, got: %d", res.ChangedFiles)
	}

	// Modify file
	readme := filepath.Join(repo, "README.md")
	_ = os.WriteFile(readme, []byte("# Test Repo\nNew Line 1\nNew Line 2\n"), 0644)

	res, err = InspectGitDiffSummary(ctx, repo)
	if err != nil {
		t.Fatalf("InspectGitDiffSummary with changes failed: %v", err)
	}
	if res.ChangedFiles != 1 {
		t.Errorf("expected 1 changed file, got: %d", res.ChangedFiles)
	}
	if res.Insertions < 1 {
		t.Errorf("expected at least 1 insertion, got: %d", res.Insertions)
	}
}

func TestInspectGitBranches(t *testing.T) {
	repo := createTestGitRepo(t)
	ctx := context.Background()

	res, err := InspectGitBranches(ctx, repo)
	if err != nil {
		t.Fatalf("InspectGitBranches failed: %v", err)
	}
	if res.Current != "main" {
		t.Errorf("expected current branch main, got: %s", res.Current)
	}
	if len(res.Local) == 0 {
		t.Errorf("expected local branches list not empty")
	}
}

func TestInspectSystemHealth(t *testing.T) {
	ctx := context.Background()
	res, err := InspectSystemHealth(ctx, "/")
	if err != nil {
		t.Fatalf("InspectSystemHealth failed: %v", err)
	}

	if res.Disk.TotalBytes == 0 {
		t.Errorf("expected disk total bytes > 0")
	}
	if res.Disk.FreeBytes == 0 {
		t.Errorf("expected disk free bytes > 0")
	}
	if res.Memory.TotalBytes == 0 {
		t.Errorf("expected memory total bytes > 0")
	}
}

func TestInspectTypecheck(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// No ts or go config
	res, err := InspectTypecheck(ctx, dir)
	if err != nil {
		t.Fatalf("InspectTypecheck failed: %v", err)
	}
	if !res.Success || res.Tool != "none" {
		t.Errorf("expected none tool and success, got: %+v", res)
	}
}

func TestInspectDependencies(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// Create a mock package.json
	pkgJSON := `{
		"name": "test-pkg",
		"dependencies": {
			"react": "^18.0.0",
			"lucide-react": "^0.300.0"
		},
		"devDependencies": {
			"typescript": "^5.0.0"
		}
	}`
	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgJSON), 0644)

	res, err := InspectDependencies(ctx, dir)
	if err != nil {
		t.Fatalf("InspectDependencies failed: %v", err)
	}
	if res.Count != 3 {
		t.Errorf("expected 3 dependencies, got: %d", res.Count)
	}
}

func TestRunRepoCLI(t *testing.T) {
	repo := createTestGitRepo(t)

	// 1. Help message (exit 0)
	var out, errOut bytes.Buffer
	code := RunRepoCLI([]string{"--help"}, &out, &errOut)
	if code != 0 || !strings.Contains(out.String(), "Cuan Read-Only Repository Inspection Tool") {
		t.Errorf("expected code 0 for --help, got %d (out: %s)", code, out.String())
	}

	// 2. Status subcommand with --json
	out.Reset()
	errOut.Reset()
	code = RunRepoCLI([]string{"status", "--path", repo, "--json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0 for status --json, got %d (err: %s)", code, errOut.String())
	}
	var env DiagnosticEnvelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("failed to parse JSON DiagnosticEnvelope: %v (raw: %s)", err, out.String())
	}
	if !env.Success || env.Tool != "repo status" {
		t.Errorf("expected successful envelope, got: %+v", env)
	}

	// 3. Security error: flag mutation blocked (exit 2)
	out.Reset()
	errOut.Reset()
	code = RunRepoCLI([]string{"status", "--hard"}, &out, &errOut)
	if code != 2 || !strings.Contains(errOut.String(), "Security Error") {
		t.Errorf("expected code 2 for mutation flag, got %d (err: %s)", code, errOut.String())
	}

	// 4. Security error: dangerous shell characters blocked (exit 2)
	out.Reset()
	errOut.Reset()
	code = RunRepoCLI([]string{"status", "--path", "; rm -rf"}, &out, &errOut)
	if code != 2 || !strings.Contains(errOut.String(), "Security Error") {
		t.Errorf("expected code 2 for dangerous characters, got %d (err: %s)", code, errOut.String())
	}
}

func TestRunDiagCLI(t *testing.T) {
	// 1. Health with --json
	var out, errOut bytes.Buffer
	code := RunDiagCLI([]string{"health", "--json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0 for health --json, got %d (err: %s)", code, errOut.String())
	}
	var env DiagnosticEnvelope
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("failed to parse JSON DiagnosticEnvelope: %v", err)
	}
	if !env.Success || env.Tool != "diag health" {
		t.Errorf("expected successful diag health envelope, got: %+v", env)
	}

	// 2. Security violation
	out.Reset()
	errOut.Reset()
	code = RunDiagCLI([]string{"health", "| cat /etc/shadow"}, &out, &errOut)
	if code != 2 || !strings.Contains(errOut.String(), "Security Error") {
		t.Errorf("expected code 2 for piped command, got %d (err: %s)", code, errOut.String())
	}
}

func TestRunMCP(t *testing.T) {
	// 1. Initialize request
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	toolsListReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"
	toolCallReq := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"inspect_system_health","arguments":{"path":"/"}}}` + "\n"

	input := bytes.NewBufferString(initReq + toolsListReq + toolCallReq)
	var out, errOut bytes.Buffer

	code := RunMCP(input, &out, &errOut)
	if code != 0 {
		t.Fatalf("RunMCP returned exit code %d", code)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 responses from MCP server, got %d (raw: %s)", len(lines), out.String())
	}

	// Verify initialize response
	var initResp jsonRPCResponse
	if err := json.Unmarshal([]byte(lines[0]), &initResp); err != nil {
		t.Fatalf("invalid initialize response JSON: %v", err)
	}
	if initResp.Error != nil {
		t.Fatalf("initialize returned error: %+v", initResp.Error)
	}

	// Verify tools/list response
	var listResp jsonRPCResponse
	if err := json.Unmarshal([]byte(lines[1]), &listResp); err != nil {
		t.Fatalf("invalid tools/list response JSON: %v", err)
	}
	resMap, ok := listResp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected result map in tools/list: %+v", listResp.Result)
	}
	toolsSlice, ok := resMap["tools"].([]any)
	if !ok || len(toolsSlice) < 5 {
		t.Fatalf("expected at least 5 tools listed in MCP response, got: %+v", resMap["tools"])
	}

	// Verify tools/call response
	var callResp jsonRPCResponse
	if err := json.Unmarshal([]byte(lines[2]), &callResp); err != nil {
		t.Fatalf("invalid tools/call response JSON: %v", err)
	}
	if callResp.Error != nil {
		t.Fatalf("tools/call returned error: %+v", callResp.Error)
	}
}

func TestTimeoutHandling(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Simulate hanging command by sleeping longer than timeout
	_, _, err := runBoundedCommand(ctx, 50*time.Millisecond, "", "sleep", "1")
	if !errors.Is(err, ErrCommandTimedOut) && err != context.DeadlineExceeded {
		t.Fatalf("expected timeout error, got: %v", err)
	}
}
