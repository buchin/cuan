package diagnostics

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/release"
)

// RunReleaseCLI is an explicit maintainer operation, not an Inbox delivery grant.
func RunReleaseCLI(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" {
		fmt.Fprintln(out, "Usage: cuan release <inspect|run> --path REPO --commit FULL_SHA --branch BRANCH [--runtime-path PATH] [--logs]\nUses tracked .cuan/release.json. Run uses the operator environment; process success does not prove deployment or accept tasks.")
		return 0
	}
	if args[0] != "inspect" && args[0] != "run" {
		fmt.Fprintln(errOut, "Use cuan release inspect or run")
		return 2
	}
	if args[0] == "run" {
		if err := taskSessionOwnerWriteError(); err != nil {
			fmt.Fprintln(errOut, err)
			return 2
		}
	}
	fs := flag.NewFlagSet("release "+args[0], flag.ContinueOnError)
	fs.SetOutput(errOut)
	path := fs.String("path", ".", "Repository directory")
	commit := fs.String("commit", "", "Exact clean source commit")
	branch := fs.String("branch", "", "Expected release branch")
	logs := fs.Bool("logs", false, "Print bounded redacted process diagnostics to stderr")
	runtimePath := fs.String("runtime-path", "\x00", "Verify required_tools exist in this explicit PATH. Empty string fails if any tools are required. Omit flag to skip runtime check (backward-compatible).")
	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(errOut, "Unexpected release arguments")
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	plan, err := release.Load(ctx, *path, *commit, *branch)
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	if args[0] == "inspect" {
		// --runtime-path flag: sentinel \x00 means flag was not supplied → skip check (backward-compatible).
		// Any other value (including "") uses that PATH explicitly via CheckRuntime.
		if *runtimePath != "\x00" {
			if err := release.CheckRuntime(plan, []string{"PATH=" + *runtimePath}); err != nil {
				fmt.Fprintln(errOut, err)
				return 1
			}
		}
		json.NewEncoder(out).Encode(plan.Description())
		return 0
	}
	result := release.Run(ctx, plan, os.Environ())
	if *logs {
		var secrets []string
		for _, item := range os.Environ() {
			parts := strings.SplitN(item, "=", 2)
			key := strings.ToUpper(parts[0])
			if len(parts) == 2 && (strings.Contains(key, "PASSWORD") || strings.Contains(key, "TOKEN") || strings.Contains(key, "SECRET") || strings.Contains(key, "KEY") || strings.Contains(key, "AUTH")) {
				secrets = append(secrets, parts[1])
			}
		}
		fmt.Fprintln(errOut, result.Diagnostic(func(text string) string { return agent.MaskSecrets(text, secrets) }))
	}
	json.NewEncoder(out).Encode(result)
	if result.Error != "" {
		if result.ExitCode > 0 && result.ExitCode < 126 {
			return result.ExitCode
		}
		return 1
	}
	return 0
}
