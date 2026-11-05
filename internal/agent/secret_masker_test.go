package agent

import (
	"strings"
	"testing"
)

func TestMaskLabeledCredentials(t *testing.T) {
	for _, label := range []string{"- Kredensial root/SSH:", "Credentials:", "**Root password:**", "kata sandi="} {
		input := "Inspect logs\n" + label + " fixture-only-value\n- Report findings"
		got := MaskSecrets(input, nil)
		if strings.Contains(got, "fixture-only-value") || !strings.Contains(got, "Report findings") || !strings.Contains(got, "Inspect logs") {
			t.Fatal("unsafe credential masking")
		}
	}
	ordinary := "Confirm whether credentials are configured; do not ask for passwords."
	if got := MaskSecrets(ordinary, nil); got != ordinary {
		t.Fatal("ordinary requirement was removed")
	}
}
