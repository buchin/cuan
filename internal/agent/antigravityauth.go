package agent

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
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
	ID                 string         `json:"session_id"`
	AuthURL            string         `json:"auth_url"`
	CreatedAt          time.Time      `json:"created_at"`
	Cmd                *exec.Cmd      `json:"-"`
	Stdin              io.WriteCloser `json:"-"`
	Done               chan struct{}  `json:"-"`
	credentialBaseline map[string][32]byte
	setup              *cliSetupDriver
	Failure            chan struct{} `json:"-"`
	ProfileHome        string        `json:"-"`
	OwnerID            string        `json:"-"`
	AccountID          string        `json:"-"`
	submitMu           sync.Mutex
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
		return envPath, false
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
	_, loggedIn := ResolveAntigravityTokenPath()
	msg := "Antigravity server session is not signed in"
	if loggedIn {
		msg = "Server credentials are available. Provider connection has not been verified"
	}
	return map[string]interface{}{
		"status":              "success",
		"harness":             "agy",
		"authenticated":       false,
		"message":             msg,
		"credentials_present": loggedIn,
		"verified":            false,
	}
}

// StartAntigravityAuthSession starts the interactive headless OAuth flow.
// Its lifetime is independent of the HTTP request and is bounded to five minutes.
func StartAntigravityAuthSession(ctx context.Context) (*AntigravityAuthSession, error) {
	return startAntigravityAuthSession(ctx, "", "", "")
}

func startAntigravityAuthSession(ctx context.Context, home, ownerID, accountID string) (*AntigravityAuthSession, error) {
	if home != "" {
		if err := prepareAntigravitySettings(home); err != nil {
			return nil, errors.New("Could not prepare the AI connection settings. Existing settings were preserved; check the service's file permissions and settings format.")
		}
	}
	authSessionsMu.Lock()
	defer authSessionsMu.Unlock()
	for _, existing := range authSessions {
		select {
		case <-existing.Done:
			continue
		default:
		}
		if existing.ProfileHome == home && existing.OwnerID == ownerID && existing.AccountID == accountID && time.Since(existing.CreatedAt) < 5*time.Minute {
			return existing, nil
		}
	}
	bin, err := ResolveAGYBinaryPath()
	if err != nil {
		return nil, errors.New("Antigravity CLI unavailable on server")
	}
	nonce := make([]byte, 24)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	lifetime, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	cmd := exec.CommandContext(lifetime, bin)
	cmd.Env = AntigravityProfileEnvironment(home)
	if home != "" {
		cmd.Dir = home
		cmd.Cancel = func() error { _ = KillProcessTree(cmd.Process.Pid); return cmd.Process.Kill() }
	}
	credentialBaseline := map[string][32]byte{}
	for _, path := range []string{AntigravityCredentialsFilePath(home), AntigravityLegacyCredentialsFilePath(home)} {
		previous, e := os.ReadFile(path)
		if e == nil {
			credentialBaseline[path] = sha256.Sum256(previous)
		} else if !os.IsNotExist(e) {
			cancel()
			return nil, errors.New("Existing sign-in state could not be checked; no login process was started")
		}
	}
	terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 40, Cols: 120})
	if err != nil {
		cancel()
		return nil, errors.New("cannot start Antigravity login terminal")
	}
	session := &AntigravityAuthSession{ProfileHome: home, OwnerID: ownerID, AccountID: accountID, ID: hex.EncodeToString(nonce), Cmd: cmd, Stdin: terminal, CreatedAt: time.Now().UTC(), Done: make(chan struct{}), Failure: make(chan struct{}, 1)}
	session.credentialBaseline = credentialBaseline
	session.setup = &cliSetupDriver{writer: terminal}
	urls := make(chan string, 1)
	go func() {
		defer terminal.Close()
		var tail string
		buf := make([]byte, 4096)
		found := false
		selectedGoogle := false
		for {
			n, readErr := terminal.Read(buf)
			session.setup.feed(string(buf[:n]))
			tail += string(buf[:n])
			// Classify only explicit provider rejection; never retain/log raw CLI errors.
			lower := strings.ToLower(tail)
			if strings.Contains(lower, "invalid_grant") || strings.Contains(lower, "invalid authorization code") {
				select {
				case session.Failure <- struct{}{}:
				default:
				}
			}
			if !selectedGoogle && strings.Contains(tail, "Select login method:") && strings.Contains(tail, "1. Google OAuth") {
				selectedGoogle = true
				_, _ = terminal.Write([]byte("\r"))
			}
			if !found {
				if match := googleOAuthURLRegex.FindString(tail); validGoogleAuthURL(match) && len(tail) > strings.Index(tail, match)+len(match) {
					found = true
					urls <- match
				}
			}
			if len(tail) > 32768 {
				tail = tail[len(tail)-16384:]
			}
			if readErr != nil {
				break
			}
		}
		_ = cmd.Wait()
		close(session.Done)
		cancel()
		authSessionsMu.Lock()
		delete(authSessions, session.ID)
		authSessionsMu.Unlock()
	}()
	select {
	case url := <-urls:
		session.AuthURL = url
		if ownerID != "" && accountID != "" {
			// A new profile must let the owner choose another signed-in Google account.
			if parsed, err := neturl.Parse(url); err == nil {
				q := parsed.Query()
				q.Set("prompt", "select_account consent")
				parsed.RawQuery = q.Encode()
				session.AuthURL = parsed.String()
			}
		}
		authSessions[session.ID] = session
		return session, nil
	case <-session.Done:
		cancel()
		return nil, errors.New("Antigravity exited before an OAuth URL appeared")
	case <-time.After(15 * time.Second):
		cancel()
		return nil, errors.New("OAuth URL unavailable; login timed out before browser authorization")
	case <-ctx.Done():
		cancel()
		return nil, ctx.Err()
	}
}

// SubmitAntigravityAuthCode passes a code only to a live OAuth terminal.
// Raw credential JSON is never accepted as an authorization code.
func SubmitAntigravityAuthCode(sessionID, code string) error {
	return submitAntigravityAuthCode(sessionID, code, "", "")
}

func submitAntigravityAuthCode(sessionID, code, ownerID, accountID string, acceptTerms ...bool) error {
	code = strings.TrimSpace(code)
	if code == "" || strings.ContainsAny(code, "\r\n{}") || len(code) > 4096 {
		return errors.New("invalid authorization code")
	}
	authSessionsMu.Lock()
	session := authSessions[sessionID]
	authSessionsMu.Unlock()
	if session == nil || session.OwnerID != ownerID || session.AccountID != accountID {
		return errors.New("login session expired or cancelled; start login again")
	}
	session.submitMu.Lock()
	defer session.submitMu.Unlock()
	if time.Since(session.CreatedAt) >= 5*time.Minute {
		CancelAntigravityAuthSession(sessionID)
		return errors.New("login session expired; start login again")
	}
	select {
	case <-session.Done:
		return errors.New("login ended; start login again")
	default:
	}
	if len(acceptTerms) > 0 && acceptTerms[0] && session.setup != nil {
		session.setup.accept()
	}
	if !sessionCredentialsSaved(session) {
		if _, err := session.Stdin.Write([]byte(code + "\r")); err != nil {
			return errors.New("login terminal closed; start login again")
		}
	}
	deadline := time.NewTimer(20 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			if sessionCredentialsSaved(session) && (session.AccountID == "" || antigravityOnboardingComplete(session.ProfileHome)) {
				CancelAntigravityAuthSession(sessionID)
				return nil
			}
		case <-session.Done:
			if sessionCredentialsSaved(session) && (session.AccountID == "" || antigravityOnboardingComplete(session.ProfileHome)) {
				return nil
			}
			if sessionCredentialsSaved(session) {
				return errors.New("Google sign-in was saved, but AI setup is incomplete. Finish the provider setup before using this account.")
			}
			return errors.New("authorization failed; restart login")
		case <-session.Failure:
			CancelAntigravityAuthSession(sessionID)
			return errors.New("Google rejected the authorization code. Restart sign-in to get a new code.")
		case <-deadline.C:
			return errors.New("AI setup is not complete. Your sign-in may be saved; finish the provider setup before using this account.")
		}
	}
}

// An old credential file is not evidence that this authorization code succeeded.
func sessionCredentialsSaved(session *AntigravityAuthSession) bool {
	home := session.ProfileHome
	if home == "" {
		home = ConfigHome()
	}
	for _, path := range []string{AntigravityCredentialsFilePath(home), AntigravityLegacyCredentialsFilePath(home)} {
		if session.credentialBaseline != nil {
			data, err := os.ReadFile(path)
			previous, existed := session.credentialBaseline[path]
			if err == nil && (!existed || sha256.Sum256(data) != previous) && hasValidCredential(path) {
				return true
			}
			continue
		}
		info, err := os.Stat(path)
		if err == nil && !info.ModTime().Before(session.CreatedAt) && hasValidCredential(path) {
			return true
		}
	}
	return false
}

func CancelAntigravityAuthSession(id string) {
	authSessionsMu.Lock()
	session := authSessions[id]
	delete(authSessions, id)
	authSessionsMu.Unlock()
	if session != nil {
		if session.Cmd != nil && session.Cmd.Process != nil {
			if session.ProfileHome != "" {
				_ = KillProcessTree(session.Cmd.Process.Pid)
			}
			_ = session.Cmd.Process.Kill()
		}
		if session.Stdin != nil {
			_ = session.Stdin.Close()
		}
	}
}

// VerifyAntigravityConnection distinguishes stored credentials from a working provider.
func VerifyAntigravityConnection(ctx context.Context) error {
	return VerifyAntigravityProfile(ctx, "")
}

var verifyProfileMu sync.Mutex

func VerifyAntigravityProfile(ctx context.Context, home string, models ...string) error {
	var loggedIn bool
	if home == "" {
		loggedIn = IsAntigravityLoggedIn()
	} else {
		loggedIn = IsAntigravityLoggedInInDir(home)
	}
	if !loggedIn {
		return errors.New("Google login required")
	}

	verifyProfileMu.Lock()
	defer verifyProfileMu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "agy", "--print", "Reply with exactly CUAN_AUTH_OK. Do not use tools.", "--print-timeout", "30s")
	if len(models) > 0 && IsTextModel(models[0]) {
		cmd.Args = append(cmd.Args, "--model", models[0])
	}
	cmd.Env = AntigravityProfileEnvironment(home)
	if home != "" {
		cmd.Dir = home
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil || !strings.Contains(string(out), "CUAN_AUTH_OK") {
		combined := strings.TrimSpace(stderr.String() + "\n" + string(out))
		classified := antigravityProbeError(combined)
		if errors.Is(ctx.Err(), context.DeadlineExceeded) && !strings.Contains(strings.ToLower(classified.Error()), "quota") {
			return fmt.Errorf("Provider readiness check timed out; saved sign-in is retained: %w", context.DeadlineExceeded)
		}
		return classified
	}
	return nil
}

// Never return raw CLI output: it can contain account or authorization material.
func antigravityProbeError(output string) error {
	lower := strings.ToLower(output)
	if strings.Contains(lower, "429") || strings.Contains(lower, "resource_exhausted") || strings.Contains(lower, "quota") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "too many requests") || strings.Contains(lower, "limit reached") {
		if dur, resetStr, ok := ParseProviderQuotaResetDuration(output); ok && dur > 0 {
			return fmt.Errorf("Antigravity provider quota is limited (Resets in %s). Credentials are saved. Wait for the quota to reset and verify again. Signing in to Google again does not increase quota.", resetStr)
		}
		return errors.New("Antigravity provider quota is limited. Credentials are saved. Wait for the quota to reset and verify again. Signing in to Google again does not increase quota.")
	}
	return errors.New("Could not verify the Antigravity provider connection. Credentials are saved. Verify again. A provider outage does not require signing in again.")
}

func validGoogleAuthURL(raw string) bool {
	parsed, err := neturl.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host != "accounts.google.com" {
		return false
	}
	q := parsed.Query()
	return (parsed.Path == "/o/oauth2/v2/auth" || parsed.Path == "/o/oauth2/auth") && q.Get("client_id") != "" && q.Get("redirect_uri") != "" && q.Get("response_type") == "code"
}
