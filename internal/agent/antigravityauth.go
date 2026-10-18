package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	googleOAuthURLRegex = regexp.MustCompile(`https://(?:accounts\.google\.com|antigravity\.google)/[^\x00-\x20\x7f"'<>]+`)

	authSessionsMu sync.Mutex
	authSessions   = make(map[string]*AntigravityAuthSession)
)

const (
	credentialsFileName       = "antigravity-oauth-token"
	legacyCredentialsFileName = "google_credentials"
)

// AntigravityAuthSession tracks an in-flight agy interactive login process.
type AntigravityAuthSession struct {
	ID        string          `json:"session_id"`
	AuthURL   string          `json:"auth_url"`
	CreatedAt time.Time       `json:"created_at"`
	Cmd       *exec.Cmd       `json:"-"`
	Stdin     io.WriteCloser  `json:"-"`
	Done      chan struct{}   `json:"-"`
	Err       error           `json:"-"`
}

// ConfigHome returns the base configuration directory for the user.
func ConfigHome() string {
	if h, err := os.UserHomeDir(); err == nil && strings.TrimSpace(h) != "" {
		return h
	}
	if envH := strings.TrimSpace(os.Getenv("HOME")); envH != "" {
		return envH
	}
	return "/home/dev"
}

// AntigravityConfigDirPath returns ~/.gemini/antigravity-cli.
func AntigravityConfigDirPath(configHome string) string {
	if strings.TrimSpace(configHome) == "" {
		configHome = ConfigHome()
	}
	return filepath.Join(configHome, ".gemini", "antigravity-cli")
}

// AntigravityCredentialsFilePath returns the path to the OAuth token file.
func AntigravityCredentialsFilePath(configHome string) string {
	return filepath.Join(AntigravityConfigDirPath(configHome), credentialsFileName)
}

// AntigravityLegacyCredentialsFilePath returns the legacy credentials path.
func AntigravityLegacyCredentialsFilePath(configHome string) string {
	return filepath.Join(AntigravityConfigDirPath(configHome), legacyCredentialsFileName)
}

// CandidateConfigHomes returns candidate home directories where Gemini/Antigravity credentials may reside.
func CandidateConfigHomes() []string {
	var candidates []string
	seen := make(map[string]bool)
	add := func(dir string) {
		dir = strings.TrimSpace(dir)
		if dir != "" && !seen[dir] {
			seen[dir] = true
			candidates = append(candidates, dir)
		}
	}

	if envPath := os.Getenv("ANTIGRAVITY_TOKEN_PATH"); envPath != "" {
		dir := filepath.Dir(filepath.Dir(filepath.Dir(envPath)))
		add(dir)
	}
	if envDir := os.Getenv("ANTIGRAVITY_CONFIG_DIR"); envDir != "" {
		dir := filepath.Dir(filepath.Dir(envDir))
		add(dir)
	}

	add(ConfigHome())
	if h, err := os.UserHomeDir(); err == nil {
		add(h)
	}
	add(os.Getenv("HOME"))
	add("/home/dev")
	add("/home/ubuntu")
	add("/root")

	return candidates
}

// ResolveAntigravityTokenPath finds the first valid token file across all candidate directories.
func ResolveAntigravityTokenPath() (string, bool) {
	// 1. Direct environment variable check
	if envPath := strings.TrimSpace(os.Getenv("ANTIGRAVITY_TOKEN_PATH")); envPath != "" {
		if hasValidCredential(envPath) {
			return envPath, true
		}
	}

	// 2. Candidate home directories
	for _, home := range CandidateConfigHomes() {
		primary := AntigravityCredentialsFilePath(home)
		if hasValidCredential(primary) {
			return primary, true
		}
		legacy := AntigravityLegacyCredentialsFilePath(home)
		if hasValidCredential(legacy) {
			return legacy, true
		}
	}

	// Default fallback to ConfigHome
	return AntigravityCredentialsFilePath(ConfigHome()), false
}

// IsAntigravityLoggedIn checks whether agy has valid file-backed OAuth credentials across candidate locations.
func IsAntigravityLoggedIn() bool {
	_, found := ResolveAntigravityTokenPath()
	return found
}

// IsAntigravityLoggedInInDir checks credentials in a specific home directory.
func IsAntigravityLoggedInInDir(configHome string) bool {
	return hasValidCredential(AntigravityCredentialsFilePath(configHome)) ||
		hasValidCredential(AntigravityLegacyCredentialsFilePath(configHome))
}

func hasValidCredential(path string) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	trimmed := strings.TrimSpace(string(raw))
	if len(trimmed) < 16 || trimmed == "{}" || trimmed == "[]" || trimmed == "null" {
		return false
	}
	var val any
	if json.Unmarshal(raw, &val) == nil {
		return jsonHasCredentialValue(val)
	}
	return true
}

func jsonHasCredentialValue(val any) bool {
	switch typed := val.(type) {
	case map[string]any:
		for _, nested := range typed {
			if jsonHasCredentialValue(nested) {
				return true
			}
		}
	case []any:
		for _, nested := range typed {
			if jsonHasCredentialValue(nested) {
				return true
			}
		}
	case string:
		return len(strings.TrimSpace(typed)) >= 8
	}
	return false
}

// GetAntigravityAuthStatus returns current authentication status for the Antigravity harness.
func GetAntigravityAuthStatus() map[string]interface{} {
	tokenPath, loggedIn := ResolveAntigravityTokenPath()
	msg := "Sesi Antigravity server belum terotentikasi"
	if loggedIn {
		msg = "Sesi Antigravity server aktif & siap digunakan"
	}
	return map[string]interface{}{
		"status":        "success",
		"harness":       "agy",
		"authenticated": loggedIn,
		"message":       msg,
		"token_path":    tokenPath,
	}
}

// StartAntigravityAuthSession spawns an agy login process and waits for the Google OAuth URL.
func StartAntigravityAuthSession(ctx context.Context) (*AntigravityAuthSession, error) {
	// If already logged in, no need to spawn
	if IsAntigravityLoggedIn() {
		return &AntigravityAuthSession{
			ID:        "active-session",
			AuthURL:   "",
			CreatedAt: time.Now().UTC(),
		}, nil
	}

	bin, err := exec.LookPath("agy")
	if err != nil {
		candidates := []string{
			"/home/dev/.local/bin/agy",
			"/usr/local/bin/agy",
			"/usr/bin/agy",
		}
		for _, c := range candidates {
			if _, statErr := os.Stat(c); statErr == nil {
				bin = c
				break
			}
		}
		if bin == "" {
			return nil, fmt.Errorf("agy binary not found on server: %w", err)
		}
	}

	sessionID := fmt.Sprintf("auth_%d", time.Now().UnixNano())
	cmd := exec.CommandContext(ctx, bin, "--print", "login")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to open stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("failed to open stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout // combine

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("failed to start agy: %w", err)
	}

	session := &AntigravityAuthSession{
		ID:        sessionID,
		Cmd:       cmd,
		Stdin:     stdin,
		CreatedAt: time.Now().UTC(),
		Done:      make(chan struct{}),
	}

	authSessionsMu.Lock()
	authSessions[sessionID] = session
	authSessionsMu.Unlock()

	urlChan := make(chan string, 1)
	errChan := make(chan error, 1)

	go func() {
		defer close(session.Done)
		defer func() {
			authSessionsMu.Lock()
			delete(authSessions, sessionID)
			authSessionsMu.Unlock()
		}()

		scanner := bufio.NewScanner(stdout)
		foundURL := false

		for scanner.Scan() {
			line := scanner.Text()
			if !foundURL {
				if match := googleOAuthURLRegex.FindString(line); match != "" {
					foundURL = true
					urlChan <- match
				}
			}
		}

		if err := cmd.Wait(); err != nil {
			session.Err = err
		}
		if !foundURL {
			errChan <- errors.New("agy exited without emitting Google OAuth URL")
		}
	}()

	select {
	case url := <-urlChan:
		session.AuthURL = url
		return session, nil
	case err := <-errChan:
		return nil, err
	case <-time.After(15 * time.Second):
		return nil, errors.New("timed out waiting for agy to output Google OAuth URL")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// SubmitAntigravityAuthCode sends the authorization code from Google to the waiting agy process.
func SubmitAntigravityAuthCode(sessionID, code string) error {
	trimmedCode := strings.TrimSpace(code)
	if trimmedCode == "" {
		return errors.New("authorization code cannot be empty")
	}

	// Direct token payload support (if user pastes complete JSON)
	if strings.HasPrefix(trimmedCode, "{") && strings.HasSuffix(trimmedCode, "}") {
		var tokenMap map[string]any
		if err := json.Unmarshal([]byte(trimmedCode), &tokenMap); err == nil && len(tokenMap) > 0 {
			tokenPath := AntigravityCredentialsFilePath(ConfigHome())
			_ = os.MkdirAll(filepath.Dir(tokenPath), 0700)
			return os.WriteFile(tokenPath, []byte(trimmedCode), 0600)
		}
	}

	authSessionsMu.Lock()
	session, exists := authSessions[sessionID]
	authSessionsMu.Unlock()

	if !exists || session == nil || session.Stdin == nil {
		// If no active in-memory session but token already exists
		if IsAntigravityLoggedIn() {
			return nil
		}
		return errors.New("no active agy login session found for ID " + sessionID)
	}

	// Write authorization code + newline to stdin
	if _, err := session.Stdin.Write([]byte(trimmedCode + "\n")); err != nil {
		return fmt.Errorf("failed to write authorization code to agy stdin: %w", err)
	}

	// Wait up to 10 seconds for credentials to be persisted
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if IsAntigravityLoggedIn() {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}

	if IsAntigravityLoggedIn() {
		return nil
	}
	return errors.New("authorization code submitted, but credentials were not verified within 10s")
}
