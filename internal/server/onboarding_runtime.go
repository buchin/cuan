package server

import (
	"os"
	"os/exec"
	"runtime"
)

// These checks inspect installation prerequisites only. They do not execute a
// provider, access account profiles, or claim that inference/work was verified.
func onboardingRuntimePrerequisites() map[string]any {
	cliAvailable := false
	if _, err := exec.LookPath("agy"); err == nil {
		cliAvailable = true
	} else if !isIsolatedTestEnv() {
		if cand := findCandidateCLI("agy"); cand != "" {
			cliAvailable = true
		}
	}

	commandcodeAvailable := false
	if _, err := exec.LookPath("commandcode"); err == nil {
		commandcodeAvailable = true
	} else if !isIsolatedTestEnv() {
		if cand := findCandidateCLI("commandcode"); cand != "" {
			commandcodeAvailable = true
		}
	}

	sandboxAvailable := false
	if _, err := exec.LookPath("bwrap"); err == nil {
		sandboxAvailable = true
	} else if !isIsolatedTestEnv() {
		if cand := findCandidateSandbox(); cand != "" {
			sandboxAvailable = true
		}
	}

	_, containerError := os.Stat("/.dockerenv")
	return map[string]any{
		"cli_available":          cliAvailable,
		"commandcode_available":  commandcodeAvailable,
		"sandbox_available":      sandboxAvailable,
		"installation_supported": runtime.GOOS == "linux" && os.IsNotExist(containerError),
		"execution_verified":     false,
	}
}
