package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/executionpolicy"
)

func TestLiveInspectionChecksMeasureScopeAfterFailingVerifier(t *testing.T) {
	for _, scenario := range []string{"pass", "assertion fails", "assertion mutates source"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			ctx := context.Background()
			run := func(args ...string) {
				t.Helper()
				command := exec.Command("git", append([]string{"-C", root}, args...)...)
				command.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@localhost", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@localhost")
				if out, err := command.CombinedOutput(); err != nil {
					t.Fatal(err, string(out))
				}
			}
			run("init", "--quiet")
			os.Mkdir(filepath.Join(root, "scripts"), 0700)
			os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("original"), 0600)
			script := "#!/bin/sh\nprintf 'actual native check output\\n'\n"
			if scenario == "assertion mutates source" {
				script += "printf 'changed by failed verifier' > tracked.txt\n"
			}
			if scenario != "pass" {
				script += "exit 1\n"
			}
			os.WriteFile(filepath.Join(root, "scripts/test.sh"), []byte(script), 0700)
			run("add", ".")
			run("commit", "--quiet", "-m", "fixture")
			baseline, err := repositoryFingerprint(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			result, evidence := verifyLiveInspectionChecks(ctx, root, "", executionpolicy.PublicationPolicy{ReadOnlyRepository: true}, baseline)
			if result.Passed != (scenario == "pass") || !strings.Contains(result.Summary, "actual native check output") {
				t.Fatal("native check result lost", result)
			}
			if evidence == nil || evidence.Unchanged() != (scenario != "assertion mutates source") {
				t.Fatal("post-check scope was not observed", evidence)
			}
			content, _ := os.ReadFile(filepath.Join(root, "tracked.txt"))
			if scenario == "assertion mutates source" && string(content) != "changed by failed verifier" {
				t.Fatal("failed source was repaired or reset")
			}
		})
	}
}

func TestLiveInspectionChecksRejectWritableScopeBeforeRunningVerifier(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "scripts"), 0700)
	os.WriteFile(filepath.Join(root, "scripts/test.sh"), []byte("#!/bin/sh\ntouch verifier-ran\n"), 0700)
	result, evidence := verifyLiveInspectionChecks(context.Background(), root, "", executionpolicy.PublicationPolicy{}, strings.Repeat("a", 64))
	if result.Passed || evidence != nil {
		t.Fatal("writable inspection accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "verifier-ran")); !os.IsNotExist(err) {
		t.Fatal("verifier ran before scope validation")
	}
}
