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

// Task instructions are stored and replayed to workers, so environment blocks
// that reached a conversation must never keep their credential values.
func TestMaskEnvAndSSHKeyValues(t *testing.T) {
	cases := []struct{ name, input, secret string }{
		{"password", "EXAMPLE_SERVICE_PASSWORD=fixture-only-secret", "fixture-only-secret"},
		{"auth secret", "CUAN_AUTH_SECRET=fixture-auth-secret-value", "fixture-auth-secret-value"},
		{"client secret", "CUAN_GITHUB_CLIENT_SECRET=fixture-client-secret-value", "fixture-client-secret-value"},
		{"ssh host key", "CUAN_PUBLICATION_SSH_HOST_KEY=ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFixtureKeyMaterial root@fixture", "AAAAC3NzaC1lZDI1NTE5AAAAIFixtureKeyMaterial"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MaskSecrets(tc.input, nil)
			if strings.Contains(got, tc.secret) {
				t.Fatalf("secret survived masking: %s", got)
			}
			if !strings.Contains(got, "[REDACTED]") {
				t.Fatalf("no redaction marker: %s", got)
			}
		})
	}
}

// Non-secret operational settings stay readable so saved instructions remain
// diagnosable after masking.
func TestMaskKeepsNonSecretSettings(t *testing.T) {
	for _, input := range []string{
		"EXAMPLE_SERVICE_PUBLIC_IP=203.0.113.10",
		"CUAN_PUBLICATION_DEPLOY_PATH=/opt/cuan",
		"CUAN_PUBLICATION_VERIFIER=ssh-release",
	} {
		if got := MaskSecrets(input, nil); got != input {
			t.Fatalf("non-secret setting was altered: %s -> %s", input, got)
		}
	}
}
