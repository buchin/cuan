package diagnostics

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func diagnosticPassed(result any) bool {
	if check, ok := result.(*TypecheckResult); ok {
		return check.Success
	}
	return true
}
func diagnosticExitCode(result any) int {
	if !diagnosticPassed(result) {
		return 1
	}
	return 0
}

// RunRepoCLI executes git repository inspection subcommands.
func RunRepoCLI(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		printRepoHelp(out)
		return 0
	}

	subcommand := strings.ToLower(strings.TrimSpace(args[0]))
	extraArgs := args[1:]

	if subcommand == "push" {
		return runRepoPush(extraArgs, out, errOut)
	}

	if err := ValidateSafeArgs(subcommand, extraArgs); err != nil {
		fmt.Fprintf(errOut, "Security Error: %v\n", err)
		return 2
	}

	fs := flag.NewFlagSet("repo "+subcommand, flag.ContinueOnError)
	fs.SetOutput(errOut)
	pathFlag := fs.String("path", "", "Target repository directory (default: current working directory)")
	jsonFlag := fs.Bool("json", false, "Output results in structured JSON format")
	limitFlag := fs.Int("limit", DefaultLogLimit, "Number of commits to display (for log subcommand)")

	if err := fs.Parse(extraArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	targetDir := *pathFlag
	if targetDir == "" {
		targetDir, _ = os.Getwd()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	start := time.Now()
	var result any
	var execErr error

	switch subcommand {
	case "status":
		result, execErr = InspectGitStatus(ctx, targetDir)
	case "log":
		result, execErr = InspectGitLog(ctx, targetDir, *limitFlag)
	case "diff-summary":
		result, execErr = InspectGitDiffSummary(ctx, targetDir)
	case "branches":
		result, execErr = InspectGitBranches(ctx, targetDir)
	case "df":
		result, execErr = InspectSystemHealth(ctx, targetDir)
	case "free":
		result, execErr = InspectSystemHealth(ctx, targetDir)
	case "tsc", "typecheck":
		result, execErr = InspectTypecheck(ctx, targetDir)
	case "health":
		result, execErr = InspectSystemHealth(ctx, targetDir)
	case "deps":
		result, execErr = InspectDependencies(ctx, targetDir)
	default:
		fmt.Fprintf(errOut, "Unknown repo subcommand: %s (use 'cuan repo --help')\n", subcommand)
		return 2
	}

	durationMs := time.Since(start).Milliseconds()

	if execErr != nil {
		if *jsonFlag {
			env := DiagnosticEnvelope{
				Tool:       "repo " + subcommand,
				Success:    false,
				DurationMs: durationMs,
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
				Error:      execErr.Error(),
			}
			enc, _ := json.MarshalIndent(env, "", "  ")
			fmt.Fprintln(out, string(enc))
		} else {
			fmt.Fprintf(errOut, "Inspection Error [%s]: %v\n", subcommand, execErr)
		}
		return 1
	}

	if *jsonFlag {
		env := DiagnosticEnvelope{
			Tool:       "repo " + subcommand,
			Success:    diagnosticPassed(result),
			DurationMs: durationMs,
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			Data:       result,
		}
		enc, err := json.MarshalIndent(env, "", "  ")
		if err != nil {
			fmt.Fprintf(errOut, "JSON Marshal Error: %v\n", err)
			return 1
		}
		fmt.Fprintln(out, string(enc))
		return diagnosticExitCode(result)
	}

	// Human-readable format
	printHumanResult(out, subcommand, result)
	return diagnosticExitCode(result)
}

// RunDiagCLI executes system health and static diagnostic subcommands.
func RunDiagCLI(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		printDiagHelp(out)
		return 0
	}

	subcommand := strings.ToLower(strings.TrimSpace(args[0]))
	extraArgs := args[1:]

	if subcommand == "push" {
		return runRepoPush(extraArgs, out, errOut)
	}

	if err := ValidateSafeArgs(subcommand, extraArgs); err != nil {
		fmt.Fprintf(errOut, "Security Error: %v\n", err)
		return 2
	}

	fs := flag.NewFlagSet("diag "+subcommand, flag.ContinueOnError)
	fs.SetOutput(errOut)
	pathFlag := fs.String("path", "", "Target directory or path (default: root / or cwd)")
	jsonFlag := fs.Bool("json", false, "Output results in structured JSON format")
	limitFlag := fs.Int("limit", DefaultLogLimit, "Number of items (if applicable)")

	if err := fs.Parse(extraArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	targetDir := *pathFlag
	if targetDir == "" {
		targetDir, _ = os.Getwd()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	start := time.Now()
	var result any
	var execErr error

	switch subcommand {
	case "health":
		result, execErr = InspectSystemHealth(ctx, targetDir)
	case "df":
		result, execErr = InspectSystemHealth(ctx, targetDir)
	case "free":
		result, execErr = InspectSystemHealth(ctx, targetDir)
	case "tsc", "typecheck":
		result, execErr = InspectTypecheck(ctx, targetDir)
	case "deps":
		result, execErr = InspectDependencies(ctx, targetDir)
	case "status":
		result, execErr = InspectGitStatus(ctx, targetDir)
	case "log":
		result, execErr = InspectGitLog(ctx, targetDir, *limitFlag)
	case "diff-summary":
		result, execErr = InspectGitDiffSummary(ctx, targetDir)
	case "branches":
		result, execErr = InspectGitBranches(ctx, targetDir)
	default:
		fmt.Fprintf(errOut, "Unknown diag subcommand: %s (use 'cuan diag --help')\n", subcommand)
		return 2
	}

	durationMs := time.Since(start).Milliseconds()

	if execErr != nil {
		if *jsonFlag {
			env := DiagnosticEnvelope{
				Tool:       "diag " + subcommand,
				Success:    false,
				DurationMs: durationMs,
				Timestamp:  time.Now().UTC().Format(time.RFC3339),
				Error:      execErr.Error(),
			}
			enc, _ := json.MarshalIndent(env, "", "  ")
			fmt.Fprintln(out, string(enc))
		} else {
			fmt.Fprintf(errOut, "Diagnostic Error [%s]: %v\n", subcommand, execErr)
		}
		return 1
	}

	if *jsonFlag {
		env := DiagnosticEnvelope{
			Tool:       "diag " + subcommand,
			Success:    diagnosticPassed(result),
			DurationMs: durationMs,
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			Data:       result,
		}
		enc, err := json.MarshalIndent(env, "", "  ")
		if err != nil {
			fmt.Fprintf(errOut, "JSON Marshal Error: %v\n", err)
			return 1
		}
		fmt.Fprintln(out, string(enc))
		return diagnosticExitCode(result)
	}

	printHumanResult(out, subcommand, result)
	return diagnosticExitCode(result)
}

func printRepoHelp(out io.Writer) {
	fmt.Fprintln(out, `Cuan Read-Only Repository Inspection Tool

Usage:
  cuan repo <subcommand> [flags]

Subcommands:
  status         Inspect current working tree, staged/unstaged changes, and branch
  log            Inspect commit history (bounded to N recent commits)
  diff-summary   Inspect summary of changes (--stat) without raw diff mutation
  branches       Inspect local and remote branch tracking
  df             Inspect filesystem disk capacity and usage
  free           Inspect host RAM memory availability
  tsc            Run static typecheck (tsc --noEmit or go vet)
  health         Inspect full host health (disk, memory, load, uptime)
  push           Push current (or specified) branch via Cuan server

Flags:
  --path <dir>      Target directory (default: current working directory)
  --allow-default   Allow pushing to the project's default branch (for 'push')
  --json            Output structured JSON response
  --limit <n>       Number of commits to display for 'log' (default: 10, max: 100)
  --help            Show this help message

Inspection commands are strictly read-only; 'push' delegates to the Cuan server with branch protection.`)
}

func runRepoPush(args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("repo push", flag.ContinueOnError)
	fs.SetOutput(errOut)
	projectFlag := fs.String("project", "", "Project ID (default: matched from --path)")
	pathFlag := fs.String("path", "", "Target repository directory (default: current working directory)")
	branchFlag := fs.String("branch", "", "Branch to push (default: current checked-out branch)")
	allowDefaultFlag := fs.Bool("allow-default", false, "Allow pushing to the project's default branch")
	jsonFlag := fs.Bool("json", false, "Output structured JSON response")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		fmt.Fprintln(errOut, "Configure the Cuan connection first (cuan config set ...).")
		return 1
	}

	targetDir := strings.TrimSpace(*pathFlag)
	if targetDir == "" {
		targetDir, _ = os.Getwd()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	projectID := strings.TrimSpace(*projectFlag)
	if projectID == "" {
		projects, err := client.ListProjects(ctx, 100)
		if err != nil {
			fmt.Fprintf(errOut, "Failed to resolve project: %v\n", err)
			return 1
		}
		cleanTarget := filepath.Clean(targetDir)
		if resolved, err := filepath.EvalSymlinks(cleanTarget); err == nil {
			cleanTarget = resolved
		}
		for _, p := range projects {
			lp, _ := p["local_path"].(string)
			if strings.TrimSpace(lp) == "" {
				continue
			}
			cleanLocal := filepath.Clean(lp)
			if resolved, err := filepath.EvalSymlinks(cleanLocal); err == nil {
				cleanLocal = resolved
			}
			if cleanTarget == cleanLocal || strings.HasPrefix(cleanTarget, cleanLocal+string(filepath.Separator)) {
				if id, ok := p["id"].(string); ok && id != "" {
					projectID = id
					break
				}
			}
		}
		if projectID == "" {
			fmt.Fprintf(errOut, "No Cuan project matched directory %s (pass --project <id>)\n", targetDir)
			return 1
		}
	}

	branch := strings.TrimSpace(*branchFlag)
	if branch == "" {
		if branches, err := InspectGitBranches(ctx, targetDir); err == nil && strings.TrimSpace(branches.Current) != "" {
			branch = strings.TrimSpace(branches.Current)
		}
	}

	res, err := client.PushProjectRepo(ctx, projectID, branch, *allowDefaultFlag)
	if err != nil {
		if *jsonFlag {
			enc, _ := json.MarshalIndent(map[string]any{
				"pushed":     false,
				"project_id": projectID,
				"branch":     branch,
				"error":      err.Error(),
			}, "", "  ")
			fmt.Fprintln(out, string(enc))
		} else {
			fmt.Fprintf(errOut, "Push failed: %v\n", err)
		}
		return 1
	}

	if *jsonFlag {
		enc, _ := json.MarshalIndent(res, "", "  ")
		fmt.Fprintln(out, string(enc))
		return 0
	}
	pushedBranch, _ := res["branch"].(string)
	commit, _ := res["commit"].(string)
	fmt.Fprintf(out, "Pushed %s (%s) for project %s\n", pushedBranch, commit, projectID)
	return 0
}

func printDiagHelp(out io.Writer) {
	fmt.Fprintln(out, `Cuan System Health & Diagnostics Tool

Usage:
  cuan diag <subcommand> [flags]

Subcommands:
  health         Inspect full host health (disk, memory, load, uptime)
  df             Inspect disk usage and capacity
  free           Inspect memory usage and availability
  tsc            Inspect static typecheck (tsc --noEmit or go vet)
  deps           Inspect project dependencies
  status         Inspect git working status
  log            Inspect git commit history

Flags:
  --path <dir>   Target directory or mount path (default: current directory or /)
  --json         Output structured JSON response
  --help         Show this help message`)
}

func printHumanResult(out io.Writer, sub string, result any) {
	switch v := result.(type) {
	case *GitStatusResult:
		cleanStatus := "CLEAN"
		if !v.Clean {
			cleanStatus = "DIRTY"
		}
		fmt.Fprintf(out, "Branch: %s (Upstream: %s) [%s]\n", v.Branch, v.Upstream, cleanStatus)
		if v.Ahead > 0 || v.Behind > 0 {
			fmt.Fprintf(out, "Ahead: %d, Behind: %d\n", v.Ahead, v.Behind)
		}
		fmt.Fprintf(out, "Changes: %d staged, %d unstaged, %d untracked\n", v.StagedCount, v.UnstagedCount, v.UntrackedCount)
		if len(v.StagedFiles) > 0 {
			fmt.Fprintf(out, "Staged files:\n")
			for _, f := range v.StagedFiles {
				fmt.Fprintf(out, "  + %s\n", f)
			}
		}
		if len(v.UnstagedFiles) > 0 {
			fmt.Fprintf(out, "Unstaged files:\n")
			for _, f := range v.UnstagedFiles {
				fmt.Fprintf(out, "  * %s\n", f)
			}
		}
		if len(v.UntrackedFiles) > 0 {
			fmt.Fprintf(out, "Untracked files:\n")
			for _, f := range v.UntrackedFiles {
				fmt.Fprintf(out, "  ? %s\n", f)
			}
		}
	case *GitLogResult:
		fmt.Fprintf(out, "Recent Commits (%d entries):\n", v.Count)
		for _, c := range v.Commits {
			fmt.Fprintf(out, "  %s  %-20s  %s  %s\n", c.SHA, c.Author, c.Date, c.Message)
		}
	case *GitDiffResult:
		if v.Summary != "" {
			fmt.Fprintf(out, "Diff Summary: %s\n", v.Summary)
		} else {
			fmt.Fprintf(out, "Diff Summary: Clean (no uncommitted or staged diffs)\n")
		}
		if v.Raw != "" && v.Summary == "" {
			fmt.Fprint(out, v.Raw)
		}
	case *GitBranchesResult:
		fmt.Fprintf(out, "Current branch: %s\n", v.Current)
		fmt.Fprintf(out, "Local branches (%d):\n", len(v.Local))
		for _, b := range v.Local {
			prefix := "  "
			if b == v.Current {
				prefix = "* "
			}
			fmt.Fprintf(out, "%s%s\n", prefix, b)
		}
		if len(v.Remote) > 0 {
			fmt.Fprintf(out, "Remote branches (%d):\n", len(v.Remote))
			for _, b := range v.Remote {
				fmt.Fprintf(out, "  %s\n", b)
			}
		}
	case *SystemHealthResult:
		fmt.Fprintf(out, "System Health [%s]:\n", v.Disk.Status)
		fmt.Fprintf(out, "- Disk: %.1f%% used (%d MB free of %d MB, path: %s)\n",
			v.Disk.UsedPercent, v.Disk.FreeBytes/(1024*1024), v.Disk.TotalBytes/(1024*1024), v.Disk.Path)
		fmt.Fprintf(out, "- Memory: %.1f%% used (%d MB available of %d MB)\n",
			v.Memory.UsedPercent, v.Memory.AvailableMB, v.Memory.TotalMB)
		fmt.Fprintf(out, "- Load Average: %.2f, %.2f, %.2f (Uptime: %ds)\n",
			v.LoadAverages[0], v.LoadAverages[1], v.LoadAverages[2], v.UptimeSeconds)
	case *TypecheckResult:
		status := "PASSED"
		if !v.Success {
			status = "FAILED"
		}
		if v.Status == TypecheckNotRun {
			status = "NOT RUN"
		} else if v.Status == TypecheckTimedOut {
			status = "TIMED OUT"
		} else if v.Status == TypecheckCanceled {
			status = "CANCELED"
		}
		fmt.Fprintf(out, "Static Typecheck (%s): %s (%d issues, %d ms)\n", v.Tool, status, v.IssueCount, v.DurationMs)
		if v.Reason != "" {
			fmt.Fprintln(out, v.Reason)
		}
		for _, issue := range v.Issues {
			fmt.Fprintf(out, "  ! %s\n", issue)
		}
	case *DepsResult:
		fmt.Fprintf(out, "Dependencies (%s, %d packages):\n", v.Tool, v.Count)
		for _, p := range v.Packages {
			fmt.Fprintf(out, "  • %s\n", p)
		}
	default:
		enc, _ := json.MarshalIndent(result, "", "  ")
		fmt.Fprintln(out, string(enc))
	}
}
