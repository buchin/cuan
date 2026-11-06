package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/buchin/cuan/internal/executionpolicy"
)

func runRegisteredProjectVerification(ctx context.Context, root, configuredTargets string) verificationResult {
	targets, err := executionpolicy.PHPUnitTargets(configuredTargets)
	if err != nil {
		return verificationResult{Error: "Registered PHPUnit targets are invalid"}
	}
	if len(targets) == 0 {
		return runProjectVerification(ctx, root)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return verificationResult{Error: "Verification workspace unavailable"}
	}
	runtime := "vendor/bin/phpunit"
	if info, err := os.Stat(filepath.Join(resolvedRoot, "vendor/bin/pest")); err == nil && info.Mode().IsRegular() {
		runtime = "vendor/bin/pest"
	}
	for _, target := range append(append([]string{}, targets...), runtime) {
		resolved, e := filepath.EvalSymlinks(filepath.Join(resolvedRoot, filepath.FromSlash(target)))
		if e != nil || !strings.HasPrefix(resolved, resolvedRoot+string(os.PathSeparator)) {
			return verificationResult{Error: "Registered PHPUnit target or runtime is unavailable outside the private workspace"}
		}
	}
	var output verifierOutputTail
	for _, target := range targets {
		// Older PHPUnit versions silently use only the first path argument.
		// Run each registered target so every configured check actually executes.
		result := runProjectVerifierCommand(ctx, root, "php", runtime, "--colors=never", target)
		_, _ = output.Write([]byte(result.Summary + "\n"))
		if !result.Passed {
			result.Summary = output.String()
			return result
		}
	}
	return verificationResult{Passed: true, Summary: "Project verification: registered PHPUnit targets (exit 0)\n" + output.String()}
}
