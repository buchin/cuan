package agent

import (
	"os"
	"testing"
)

func TestIsAntigravityLoggedIn(t *testing.T) {
	tempHome := t.TempDir()

	// Initially not logged in
	if IsAntigravityLoggedInInDir(tempHome) {
		t.Fatal("expected IsAntigravityLoggedIn to return false for empty dir")
	}

	// Create fake credentials
	credDir := AntigravityConfigDirPath(tempHome)
	_ = os.MkdirAll(credDir, 0700)
	credFile := AntigravityCredentialsFilePath(tempHome)

	fakeToken := `{"auth_method":"oauth","id_token":"mock-id-token-123456","token":"mock-access-token-123456"}`
	if err := os.WriteFile(credFile, []byte(fakeToken), 0600); err != nil {
		t.Fatalf("failed to write fake token: %v", err)
	}

	if !IsAntigravityLoggedInInDir(tempHome) {
		t.Fatal("expected IsAntigravityLoggedIn to return true after writing token")
	}

	// Check with status helper
	status := GetAntigravityAuthStatus()
	if status["harness"] != "agy" {
		t.Fatalf("expected harness agy, got %v", status["harness"])
	}
}
