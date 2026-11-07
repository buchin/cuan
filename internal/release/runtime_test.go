package release

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseRuntimeRequirementsUseOnlySelectedPathWithoutExecutingTools(t *testing.T) {
	for _, scenario := range []string{"available", "ambient only", "relative path", "nonexecutable", "missing PATH", "nil environment"} {
		t.Run(scenario, func(t *testing.T) {
			tools := t.TempDir()
			tool := filepath.Join(tools, "fixture-tool")
			toolMarker := filepath.Join(t.TempDir(), "tool-ran")
			mode := os.FileMode(0700)
			if scenario == "nonexecutable" {
				mode = 0600
			}
			if err := os.WriteFile(tool, []byte("#!/bin/sh\nprintf unexpected > \"$TOOL_MARKER\"\n"), mode); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", tools+":"+os.Getenv("PATH"))
			manifest := `{"version":1,"command":["release.sh"],"timeout_seconds":5,"required_tools":["fixture-tool"]}`
			root, commit := fixture(t, "printf release > \"$MARKER\"\n", manifest)
			plan, err := Load(context.Background(), root, commit, "dev")
			if err != nil {
				t.Fatal(err)
			}
			descriptor := plan.Description()
			if len(descriptor.RequiredTools) != 1 {
				t.Fatal("requirements not retained")
			}
			descriptor.RequiredTools[0] = "changed"
			marker := filepath.Join(t.TempDir(), "release-ran")
			environment := []string{"PATH=" + tools, "MARKER=" + marker, "TOOL_MARKER=" + toolMarker}
			switch scenario {
			case "ambient only":
				environment[0] = "PATH=" + t.TempDir()
			case "relative path":
				environment[0] = "PATH=relative"
			case "missing PATH":
				environment = environment[1:]
			case "nil environment":
				environment = nil
			}
			checkErr := CheckRuntime(plan, environment)
			if (checkErr == nil) != (scenario == "available") {
				t.Fatal("runtime borrowed ambient tool or rejected selected tool", checkErr)
			}
			result := Run(context.Background(), plan, environment)
			if scenario == "available" {
				if result.ExitCode != 0 || !result.SourceUnchanged {
					t.Fatal("available native runtime did not execute release", result)
				}
			} else {
				if result.ExitCode != -1 {
					t.Fatal("missing runtime started process")
				}
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatal("release executed without required tools")
				}
			}
			if _, err := os.Stat(toolMarker); !os.IsNotExist(err) {
				t.Fatal("runtime availability inspection executed a tool")
			}
		})
	}
}

func TestReleaseRejectsMalformedRuntimeRequirements(t *testing.T) {
	cases := [][]string{{"../tool"}, {"/usr/bin/tool"}, {"tool arg"}, {""}, {"tool", "tool"}}
	many := make([]string, 33)
	for i := range many {
		many[i] = "tool"
	}
	cases = append(cases, many)
	for _, requirements := range cases {
		raw, _ := json.Marshal(map[string]any{"version": 1, "command": []string{"release.sh"}, "timeout_seconds": 5, "required_tools": requirements})
		root, commit := fixture(t, "exit 0\n", string(raw))
		if _, err := Load(context.Background(), root, commit, "dev"); err == nil {
			t.Fatal("malformed tool requirement accepted", requirements)
		}
	}
}
