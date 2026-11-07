package config

import "testing"

func TestNativeReleaseWorkerRequiresExplicitOptIn(t *testing.T) {
	for _, value := range []string{"", "false", "1", "TRUE", "true"} {
		t.Run("value="+value, func(t *testing.T) {
			t.Setenv("CUAN_RUNNER_NATIVE_RELEASE", value)
			if got := Load().RunnerNativeRelease; got != (value == "true") {
				t.Fatal("unexpected native release opt-in", value, got)
			}
		})
	}
}
