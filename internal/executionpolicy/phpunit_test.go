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

func TestPHPUnitTargetsFlexibleFormat(t *testing.T) {
	cases := []struct {
		raw      string
		expected []string
	}{
		{`[tests/Unit/EmailAddressHeuristicValidatorTest.php]`, []string{"tests/Unit/EmailAddressHeuristicValidatorTest.php"}},
		{`tests/Feature/SmokeTest.php`, []string{"tests/Feature/SmokeTest.php"}},
		{`['tests/Unit/A.php', 'tests/Feature/B.php']`, []string{"tests/Unit/A.php", "tests/Feature/B.php"}},
		{`tests/Unit/A.php, tests/Feature/B.php`, []string{"tests/Unit/A.php", "tests/Feature/B.php"}},
	}
	for _, tc := range cases {
		targets, err := PHPUnitTargets(tc.raw)
		if err != nil {
			t.Fatalf("expected valid targets for %q, got error: %v", tc.raw, err)
		}
		if len(targets) != len(tc.expected) {
			t.Fatalf("expected %d targets for %q, got %d", len(tc.expected), tc.raw, len(targets))
		}
		for i, v := range targets {
			if v != tc.expected[i] {
				t.Fatalf("target mismatch at %d: expected %s, got %s", i, tc.expected[i], v)
			}
		}
	}
}
