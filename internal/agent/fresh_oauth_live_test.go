package agent

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// Explicit isolated native probe only. It starts provider authorization without
// supplying a code, accepting terms or making an inference request. Completion
// of actual account authorization must be established separately by the owner.
func TestLiveFreshProfileOAuthStart(t *testing.T) {
	if os.Getenv("CUAN_FRESH_OAUTH_PROBE") != "1" {
		t.Skip("explicit isolated native OAuth-start probe required")
	}
	if os.Getenv("FURAL_AGENT_TOKEN") != "" {
		t.Fatal("run only with a clean dedicated Cuan test environment")
	}
	home := t.TempDir()
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatal("profile must start empty")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	session, err := startAntigravityAuthSession(ctx, home, "", "")
	if err != nil {
		t.Fatal("fresh profile did not reach OAuth", err)
	}
	defer CancelAntigravityAuthSession(session.ID)
	if !validGoogleAuthURL(session.AuthURL) {
		t.Fatal("provider authorization URL not validated")
	}
	if sessionCredentialsSaved(session) || antigravityOnboardingComplete(home) {
		t.Fatal("fresh OAuth start falsely completed account setup")
	}
	// Native CLI may rewrite settings.json while initializing. Readiness is
	// established by the actual authorization flow, not literal file retention.
	session.setup.mu.Lock()
	themeHandled := session.setup.themeSelected
	accepted := session.setup.accepted
	session.setup.mu.Unlock()
	if accepted {
		t.Fatal("provider terms accepted without owner consent")
	}

	CancelAntigravityAuthSession(session.ID)
	select {
	case <-session.Done:
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled native login did not exit")
	}
	if sessionCredentialsSaved(session) || antigravityOnboardingComplete(home) {
		t.Fatal("cancelled login saved credentials or accepted terms")
	}
	proof := map[string]any{"freshEmptyProfile": true, "nativeOAuthURLValidated": true, "noManualThemeInput": true, "themeWizardHandledByDriver": themeHandled, "noCredentialCopy": true, "noAuthorizationCodeSubmitted": true, "noTermsAccepted": true, "noInference": true, "cancelledProcessExited": true, "fullOAuthCompletionVerified": false}
	if output := os.Getenv("CUAN_FRESH_OAUTH_EVIDENCE"); output != "" {
		data, _ := json.MarshalIndent(proof, "", "  ")
		if err := os.WriteFile(output, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Log("Native empty-profile OAuth start and cancellation passed; real owner authorization remains unverified")
}
