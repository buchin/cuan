package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestUnexpectedArgumentsNeverStartServer(t *testing.T) {
	if fixture := os.Getenv("CUAN_MAIN_ARGUMENT_FIXTURE"); fixture != "" {
		os.Args = append([]string{"cuan"}, strings.Split(fixture, "\n")...)
		main()
		return
	}
	for _, args := range [][]string{{"agent", "--help"}, {"--external-chat-worker", "unexpected"}, {"--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestUnexpectedArgumentsNeverStartServer$")
			cmd.Env = append(os.Environ(), "CUAN_MAIN_ARGUMENT_FIXTURE="+strings.Join(args, "\n"))
			output, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatal("argument handling entered a long-running server path", string(output))
			}
			if strings.Contains(string(output), "Checking database connectivity") || strings.Contains(string(output), "Waiting for database") {
				t.Fatal("argument handling initialized database access", string(output))
			}
			if args[0] == "--help" {
				if err != nil || len(output) == 0 {
					t.Fatal("normal help no longer exits successfully", err, string(output))
				}
				return
			}
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 2 || !strings.Contains(string(output), "cuan --help") {
				t.Fatal("invalid invocation did not return a useful usage error", err, string(output))
			}
		})
	}
}
