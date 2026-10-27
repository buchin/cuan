package server

import (
	"os"
	"os/exec"
	"runtime"
)

// These checks inspect installation prerequisites only. They do not execute a
// provider, access account profiles, or claim that inference/work was verified.
func onboardingRuntimePrerequisites() map[string]any {
	_, cliError := exec.LookPath("agy")
	_, commandcodeError := exec.LookPath("commandcode")
	_, sandboxError := exec.LookPath("bwrap")
	_, containerError := os.Stat("/.dockerenv")
	return map[string]any{
		"cli_available":          cliError == nil,
		"commandcode_available":  commandcodeError == nil,
		"sandbox_available":      sandboxError == nil,
		"installation_supported": runtime.GOOS == "linux" && os.IsNotExist(containerError),
		"execution_verified":     false,
	}
}
