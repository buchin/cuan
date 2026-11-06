package executionpolicy

import "testing"

func TestPHPUnitTargetsRejectOptionsAndNonTestPaths(t *testing.T) {
	for _, raw := range []string{`["tests/Feature/McpTest.php"]`, `["packages/bridge/tests"]`, ``, `[]`, `["--help"]`, `["tests/../.env"]`, `["/tests/Test.php"]`, `["packages/bridge/tests-elsewhere"]`, `["app/Model.php"]`, `["tests/a\ncommand"]`, `["tests\\other.php"]`} {
		targets, err := PHPUnitTargets(raw)
		valid := raw == `["tests/Feature/McpTest.php"]` || raw == `["packages/bridge/tests"]` || raw == ``
		if (err == nil) != valid {
			t.Fatalf("unexpected configuration acceptance %q: %v %v", raw, targets, err)
		}
	}
}
