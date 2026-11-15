package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/buchin/cuan/internal/executionpolicy"
)

// validateRegisteredVerificationTargets reports the first declared target that
// cannot exist in this workspace, or "" when all of them resolve. Declared
// checks are project configuration, so a stale entry is a prerequisite problem:
// detecting it before the agent starts keeps a valid attempt from being spent on
// configuration the agent cannot fix within its own scope.
func validateRegisteredVerificationTargets(root, configuredTargets string) string {
	targets, err := executionpolicy.PHPUnitTargets(configuredTargets)
	if err != nil {
		return "Registered PHPUnit targets are invalid. Project configuration must name valid checks; this attempt did not consume the task."
	}
	if len(targets) == 0 {
		return ""
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "Verification workspace unavailable; the attempt did not start."
	}
	runtime := "vendor/bin/phpunit"
	if info, err := os.Stat(filepath.Join(resolvedRoot, "vendor/bin/pest")); err == nil && info.Mode().IsRegular() {
		runtime = "vendor/bin/pest"
	}
	for _, target := range append(append([]string{}, targets...), runtime) {
		resolved, err := filepath.EvalSymlinks(filepath.Join(resolvedRoot, filepath.FromSlash(target)))
		kind := "target"
		if target == runtime {
			kind = "runtime"
		}
		if err != nil {
			if os.IsNotExist(err) {
				if kind == "runtime" {
					// The PHP dependency preparation step may still provide it.
					continue
				}
				return fmt.Sprintf("Registered PHPUnit %s %s is missing from this project; fix the project's declared checks before running this task.", kind, target)
			}
			return fmt.Sprintf("Registered PHPUnit %s %s could not be resolved; fix the project's declared checks before running this task.", kind, target)
		}
		if !strings.HasPrefix(resolved, resolvedRoot+string(os.PathSeparator)) {
			if kind == "runtime" {
				continue
			}
			return fmt.Sprintf("Registered PHPUnit %s %s resolves outside the project workspace; fix the project's declared checks before running this task.", kind, target)
		}
	}
	return ""
}

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
		kind := "target"
		if target == runtime {
			kind = "runtime"
		}
		prerequisiteFailure := func(reason string) verificationResult {
			message := fmt.Sprintf("Registered PHPUnit %s %s: %s. Project checks did not run; no test success or publication is established.", kind, target, reason)
			return verificationResult{Error: message, Summary: "Project verification prerequisite failed: " + message}
		}
		if e != nil {
			if os.IsNotExist(e) {
				return prerequisiteFailure("missing from the current private workspace")
			}
			return prerequisiteFailure("could not be resolved")
		}
		if !strings.HasPrefix(resolved, resolvedRoot+string(os.PathSeparator)) {
			return prerequisiteFailure("resolves outside the private workspace")
		}
		if target == runtime {
			if info, err := os.Stat(resolved); err != nil || !info.Mode().IsRegular() {
				return prerequisiteFailure("is not an available regular file")
			}
		}
	}
	var output verifierOutputTail
	for _, target := range targets {
		// Older PHPUnit versions silently use only the first path argument.
		// Run each registered target so every configured check actually executes.
		result := runProjectVerifierCommand(ctx, root, "php", runtime, "--colors=never", target)
		_, _ = output.Write([]byte(result.Summary + "\n"))
		if !result.Passed {
			exitCode := 1
			if idx := strings.Index(result.Summary, "(exit "); idx != -1 {
				if end := strings.Index(result.Summary[idx:], ")"); end != -1 {
					var code int
					if _, err := fmt.Sscanf(result.Summary[idx:idx+end+1], "(exit %d)", &code); err == nil && code != 0 {
						exitCode = code
					}
				}
			}
			result.Summary = fmt.Sprintf("Project verification: registered PHPUnit targets (exit %d)\n%s", exitCode, output.String())
			return result
		}
	}
	return verificationResult{Passed: true, Summary: "Project verification: registered PHPUnit targets (exit 0)\n" + output.String()}
}
