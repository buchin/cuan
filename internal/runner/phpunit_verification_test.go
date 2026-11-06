package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/task"
)

func TestRegisteredPHPUnitRunsOnlySavedTargetsAndPreservesFailure(t *testing.T) {
	if _, err := exec.LookPath("php"); err != nil {
		t.Fatal("PHP runtime required for native verifier contract")
	}
	for _, scenario := range []string{"pass", "fail", "missing target", "escaped target", "invalid targets"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			for _, dir := range []string{"vendor/bin", "tests/Feature"} {
				if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			target := "tests/Feature/ContractTest.php"
			if err := os.WriteFile(filepath.Join(root, target), []byte("fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			script := `<?php
if (array_slice($argv,1) !== ['--colors=never','tests/Feature/ContractTest.php']) { exit(9); }
echo "Registered target executed\n";
exit(` + map[bool]string{true: "7", false: "0"}[scenario == "fail"] + `);`
			if err := os.WriteFile(filepath.Join(root, "vendor/bin/phpunit"), []byte(script), 0600); err != nil {
				t.Fatal(err)
			}
			env := `["tests/Feature/ContractTest.php"]`
			switch scenario {
			case "missing target":
				_ = os.Remove(filepath.Join(root, target))
			case "escaped target":
				_ = os.Remove(filepath.Join(root, target))
				outside := filepath.Join(t.TempDir(), "external")
				_ = os.WriteFile(outside, []byte("external"), 0600)
				_ = os.Symlink(outside, filepath.Join(root, target))
			case "invalid targets":
				env = `["--configuration=outside"]`
			}
			got := runRegisteredProjectVerification(context.Background(), root, env)
			if got.Passed != (scenario == "pass") {
				t.Fatal("incorrect native check", got)
			}
			if task.PassingProjectCheckSummary(got.Summary) != (scenario == "pass") {
				t.Fatal("failed or missing check credited", got)
			}
			if (scenario == "pass" || scenario == "fail") && !strings.Contains(got.Summary, "Registered target executed") {
				t.Fatal("runtime did not execute target", got)
			}
		})
	}
}

func TestRegisteredPHPUnitDoesNotSkipSecondTarget(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"vendor/bin", "tests/Feature"} {
		_ = os.MkdirAll(filepath.Join(root, dir), 0700)
	}
	for _, name := range []string{"First", "Second"} {
		_ = os.WriteFile(filepath.Join(root, "tests/Feature", name+"Test.php"), []byte("fixture"), 0600)
	}
	script := `<?php echo $argv[2]." executed\n"; exit(str_contains($argv[2], 'Second') ? 7 : 0);`
	_ = os.WriteFile(filepath.Join(root, "vendor/bin/phpunit"), []byte(script), 0600)
	got := runRegisteredProjectVerification(context.Background(), root, `["tests/Feature/FirstTest.php","tests/Feature/SecondTest.php"]`)
	if got.Passed || task.PassingProjectCheckSummary(got.Summary) || !strings.Contains(got.Summary, "SecondTest.php executed") || !strings.Contains(got.Summary, "FirstTest.php executed") {
		t.Fatal("second test target skipped", got)
	}
}
