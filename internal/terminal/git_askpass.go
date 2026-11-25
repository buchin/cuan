package terminal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EnsureGitAskpassScript writes a lightweight executable wrapper in the temp directory
// that delegates Git credential prompts to `<binaryPath> git-askpass`.
// Git requires GIT_ASKPASS to be a single executable path without arguments.
func EnsureGitAskpassScript(binaryPath string) (string, error) {
	if strings.TrimSpace(binaryPath) == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", err
		}
		binaryPath = exe
	}
	scriptPath := filepath.Join(os.TempDir(), fmt.Sprintf("cuan-git-askpass-%d.sh", os.Getuid()))
	quoted := "'" + strings.ReplaceAll(binaryPath, "'", "'\"'\"'") + "'"
	desired := "#!/bin/sh\nexec " + quoted + " git-askpass \"$@\"\n"
	if existing, err := os.ReadFile(scriptPath); err == nil && string(existing) == desired {
		_ = os.Chmod(scriptPath, 0o700)
		return scriptPath, nil
	}
	tmpPath := fmt.Sprintf("%s.%d.tmp", scriptPath, time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, []byte(desired), 0o700); err != nil {
		return "", err
	}
	if err := os.Rename(tmpPath, scriptPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return scriptPath, nil
}

// RunGitAskpassCLI implements `cuan git-askpass <prompt>` for GIT_ASKPASS in Project Terminal.
func RunGitAskpassCLI(args []string) int {
	prompt := ""
	if len(args) > 0 {
		prompt = strings.ToLower(strings.TrimSpace(args[0]))
	}
	if strings.HasPrefix(prompt, "username") {
		fmt.Println("x-access-token")
		return 0
	}
	apiURL := strings.TrimSpace(os.Getenv("CUAN_GIT_ASKPASS_URL"))
	token := strings.TrimSpace(os.Getenv("CUAN_GIT_ASKPASS_TOKEN"))
	if apiURL == "" || token == "" {
		fmt.Fprintln(os.Stderr, "cuan git-askpass: terminal git credentials are not configured")
		return 1
	}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cuan git-askpass: %v\n", err)
		return 1
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cuan git-askpass: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "cuan git-askpass: server returned status %d\n", resp.StatusCode)
		return 1
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil || strings.TrimSpace(payload.Token) == "" {
		fmt.Fprintln(os.Stderr, "cuan git-askpass: missing repository token in server response")
		return 1
	}
	fmt.Println(payload.Token)
	return 0
}
