package agent

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Account metadata and CLI profiles share a private persistent configuration root.
// Credentials stay CLI-owned; no token or filesystem path is part of the DTO.
type ProviderAccount struct {
	AllowModelFallback bool                 `json:"allow_model_fallback"`
	ModelCooldowns     map[string]time.Time `json:"model_cooldowns,omitempty"`
	AllowFailover      bool                 `json:"allow_failover"`
	QuotaRetryAt       *time.Time           `json:"quota_retry_at,omitempty"`
	AgentID            string               `json:"agent_id,omitempty"`
	ConnectionID       string               `json:"connection_id,omitempty"`
	ID                 string               `json:"id"`
	OwnerID            string               `json:"-"`
	Label              string               `json:"label"`
	Email              string               `json:"email,omitempty"`
	Provider           string               `json:"provider"`
	Legacy             bool                 `json:"legacy"`
	CredentialsPresent bool                 `json:"credentials_present"`
	State              string               `json:"state"`
	Message            string               `json:"message,omitempty"`
	LastCheckedAt      *time.Time           `json:"last_checked_at,omitempty"`
	VerifiedAt         *time.Time           `json:"verified_at,omitempty"`
	CreatedAt          time.Time            `json:"created_at"`
}

type storedProviderAccount struct {
	ProviderAccount
	Owner string `json:"owner"`
}

var accountProfileMu sync.Mutex
var accountIDPattern = regexp.MustCompile(`^acct_[a-f0-9]{32}$`)
var ErrAccountNotFound = errors.New("account not found")

func accountRoot() string { return filepath.Join(ConfigHome(), ".local", "share", "cuan", "accounts") }
func accountDirectory(id string) (string, error) {
	if !accountIDPattern.MatchString(id) {
		return "", ErrAccountNotFound
	}
	return filepath.Join(accountRoot(), id), nil
}

func CreateProviderAccount(owner, label string) (*ProviderAccount, error) {
	label = strings.TrimSpace(label)
	if owner == "" || label == "" || len(label) > 100 || strings.ContainsAny(label, "\r\n") {
		return nil, errors.New("Enter an account name of up to 100 characters")
	}
	accountProfileMu.Lock()
	defer accountProfileMu.Unlock()
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	a := &ProviderAccount{ID: "acct_" + hex.EncodeToString(nonce[:]), OwnerID: owner, Label: label, Provider: "agy", State: "login_required", CreatedAt: time.Now().UTC()}
	dir, _ := accountDirectory(a.ID)
	if err := os.MkdirAll(filepath.Join(dir, "home"), 0700); err != nil {
		return nil, err
	}
	if err := os.Chmod(accountRoot(), 0700); err != nil {
		return nil, err
	}
	if err := saveProviderAccount(a); err != nil {
		return nil, err
	}
	return a, nil
}

func saveProviderAccount(a *ProviderAccount) error {
	dir, err := accountDirectory(a.ID)
	if err != nil {
		return err
	}
	data, err := json.Marshal(storedProviderAccount{ProviderAccount: *a, Owner: a.OwnerID})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".metadata-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "account.json"))
}

func extractAccountEmail(home string) string {
	for _, file := range []string{AntigravityCredentialsFilePath(home), AntigravityLegacyCredentialsFilePath(home)} {
		raw, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var root struct {
			IDToken string `json:"id_token"`
			Email   string `json:"email"`
		}
		if json.Unmarshal(raw, &root) == nil {
			if root.Email != "" && strings.Contains(root.Email, "@") {
				return root.Email
			}
			if root.IDToken != "" {
				parts := strings.Split(root.IDToken, ".")
				if len(parts) >= 2 {
					payload, err := base64.RawURLEncoding.DecodeString(parts[1])
					if err != nil {
						payload, err = base64.URLEncoding.DecodeString(parts[1])
					}
					if err == nil {
						var claims struct {
							Email string `json:"email"`
						}
						if json.Unmarshal(payload, &claims) == nil && strings.Contains(claims.Email, "@") {
							return claims.Email
						}
					}
				}
			}
		}
	}
	return ""
}

func GetProviderAccount(owner, id string) (*ProviderAccount, error) {
	if id == "legacy" {
		if owner == "" {
			return nil, ErrAccountNotFound
		}
		return &ProviderAccount{ID: "legacy", OwnerID: owner, Label: "Akun server lama", Provider: "agy", Legacy: true, CredentialsPresent: IsAntigravityLoggedIn(), State: "unknown"}, nil
	}
	dir, err := accountDirectory(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "account.json"))
	if err != nil {
		return nil, ErrAccountNotFound
	}
	var stored storedProviderAccount
	if json.Unmarshal(data, &stored) != nil || stored.Owner != owner || owner == "" || stored.ID != id {
		return nil, ErrAccountNotFound
	}
	a := stored.ProviderAccount
	a.OwnerID = stored.Owner
	if a.Email == "" {
		a.Email = extractAccountEmail(filepath.Join(dir, "home"))
	}
	a.CredentialsPresent = IsAntigravityLoggedInInDir(filepath.Join(dir, "home"))
	if !a.CredentialsPresent {
		a.State = "login_required"
		a.Message = "Login Google diperlukan untuk akun ini"
		a.VerifiedAt = nil
	} else if a.State == "quota_limited" && a.QuotaRetryAt != nil && !time.Now().Before(*a.QuotaRetryAt) {
		a.State = "unknown"
		a.Message = "Usage cooldown ended. Availability has not been checked."
	} else if a.State == "verified" && a.VerifiedAt == nil {
		a.State = "unknown"
		a.Message = "Availability has not been checked."
	}
	return &a, nil
}

func ListProviderAccounts(owner string) ([]*ProviderAccount, error) {
	result := []*ProviderAccount{}
	if IsAntigravityLoggedIn() {
		a, _ := GetProviderAccount(owner, "legacy")
		if a != nil {
			result = append(result, a)
		}
	}
	entries, err := os.ReadDir(accountRoot())
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if a, err := GetProviderAccount(owner, entry.Name()); err == nil {
			result = append(result, a)
		}
	}
	return result, nil
}

func ProviderAccountHome(owner, id string) (string, error) {
	a, err := GetProviderAccount(owner, id)
	if err != nil {
		return "", err
	}
	if a.Legacy {
		return "", nil
	}
	dir, _ := accountDirectory(id)
	return filepath.Join(dir, "home"), nil
}

// DeleteProviderAccount removes a provider account and its stored credentials.
func DeleteProviderAccount(owner, id string) error {
	if id == "" || id == "legacy" {
		return errors.New("cannot delete legacy server account")
	}
	dir, err := accountDirectory(id)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

// ResolveHealthyAccountProfile finds an available verified or non-quota-limited provider account
// from the stored accounts directory and returns its home directory and account ID.
func ResolveHealthyAccountProfile(owner string) (string, *ProviderAccount, error) {
	if owner == "" {
		owner = "owner_admin"
	}
	accounts, err := ListProviderAccounts(owner)
	if err != nil || len(accounts) == 0 {
		return "", nil, err
	}

	// 1. First priority: verified account with credentials present
	for _, a := range accounts {
		if a.Legacy || !a.CredentialsPresent {
			continue
		}
		if a.State == "verified" {
			home, err := ProviderAccountHome(owner, a.ID)
			if err == nil && home != "" && IsAntigravityLoggedInInDir(home) {
				return home, a, nil
			}
		}
	}

	// 2. Second priority: any account not quota_limited and not unavailable
	for _, a := range accounts {
		if a.Legacy || !a.CredentialsPresent {
			continue
		}
		if a.State != "quota_limited" && a.State != "unavailable" && a.State != "login_required" {
			home, err := ProviderAccountHome(owner, a.ID)
			if err == nil && home != "" && IsAntigravityLoggedInInDir(home) {
				return home, a, nil
			}
		}
	}

	return "", nil, errors.New("no healthy provider account available")
}

// A filtered child environment avoids inherited provider credentials, config paths,
// cloud secrets, keyrings and fallback to another user's account. Never os.Setenv HOME.
func AntigravityProfileEnvironment(home string) []string {
	env := []string{}
	hasPath := false
	for _, key := range []string{"PATH", "LANG", "LC_ALL", "SSL_CERT_FILE", "SSL_CERT_DIR", "LD_LIBRARY_PATH", "TMPDIR"} {
		if value, ok := os.LookupEnv(key); ok {
			if key == "PATH" {
				hasPath = true
				paths := filepath.SplitList(value)
				pathSet := make(map[string]bool)
				for _, p := range paths {
					pathSet[p] = true
				}
				candidates := []string{}
				if home != "" {
					candidates = append(candidates, filepath.Join(home, ".local", "bin"))
				}
				candidates = append(candidates, "/usr/local/bin", "/opt/cuan/bin", "/usr/bin", "/bin")
				for _, cand := range candidates {
					if !pathSet[cand] {
						paths = append([]string{cand}, paths...)
						pathSet[cand] = true
					}
				}
				value = strings.Join(paths, string(filepath.ListSeparator))
			}
			env = append(env, key+"="+value)
		}
	}
	if !hasPath {
		defaultPaths := []string{}
		if home != "" {
			defaultPaths = append(defaultPaths, filepath.Join(home, ".local", "bin"))
		}
		defaultPaths = append(defaultPaths, "/usr/local/bin", "/opt/cuan/bin", "/usr/bin", "/bin")
		env = append(env, "PATH="+strings.Join(defaultPaths, string(filepath.ListSeparator)))
	}
	legacy := home == ""
	if legacy {
		home = ConfigHome()
	}
	env = append(env, "HOME="+home, "XDG_CONFIG_HOME="+filepath.Join(home, ".config"), "XDG_DATA_HOME="+filepath.Join(home, ".local", "share"), "XDG_CACHE_HOME="+filepath.Join(home, ".cache"), "GEMINI_CLI_HOME="+home, "ANTIGRAVITY_CONFIG_DIR="+AntigravityConfigDirPath(home), "ANTIGRAVITY_TOKEN_PATH="+AntigravityCredentialsFilePath(home), "SSH_CONNECTION=127.0.0.1 22 127.0.0.1 22", "SSH_CLIENT=127.0.0.1 22 22", "SSH_TTY=/dev/pts/0", "TERM=xterm-256color", "CI=", "DBUS_SESSION_BUS_ADDRESS=", "GNOME_KEYRING_CONTROL=")
	if legacy {
		// Retain explicitly configured paths only for the pre-existing service account.
		for _, key := range []string{"ANTIGRAVITY_TOKEN_PATH", "ANTIGRAVITY_CONFIG_DIR", "GEMINI_CLI_HOME"} {
			if value := os.Getenv(key); value != "" {
				env = append(env, key+"="+value)
			}
		}
	}
	return env
}

func StartProviderAccountAuth(ctx context.Context, owner, id string) (*AntigravityAuthSession, error) {
	home, err := ProviderAccountHome(owner, id)
	if err != nil {
		return nil, err
	}
	// Existing credentials are never overwritten by a fresh login request.
	if id == "legacy" || IsAntigravityLoggedInInDir(home) {
		return nil, errors.New("This account already has credentials. Add a new account for a different sign-in.")
	}
	return startAntigravityAuthSession(ctx, home, owner, id)
}

func SubmitProviderAccountAuth(owner, id, session, code string, acceptTerms ...bool) error {
	if _, err := GetProviderAccount(owner, id); err != nil {
		return err
	}
	return submitAntigravityAuthCode(session, code, owner, id, acceptTerms...)
}
func CancelProviderAccountAuth(owner, id, sessionID string) error {
	authSessionsMu.Lock()
	s := authSessions[sessionID]
	ok := s != nil && s.OwnerID == owner && s.AccountID == id
	authSessionsMu.Unlock()
	if !ok {
		return errors.New("Sign-in session is unavailable")
	}
	CancelAntigravityAuthSession(sessionID)
	return nil
}

func VerifyProviderAccount(ctx context.Context, owner, id string) (*ProviderAccount, error) {
	a, err := GetProviderAccount(owner, id)
	if err != nil {
		return nil, err
	}
	home, err := ProviderAccountHome(owner, id)
	if err != nil {
		return nil, err
	}
	probeStarted := time.Now().UTC()
	model := ""
	// Verify the same provider-default path used by a new Auto conversation.
	probeErr := VerifyAntigravityProfile(ctx, home, model)

	now := time.Now().UTC()
	a.LastCheckedAt = &now
	a.CredentialsPresent = IsAntigravityLoggedInInDir(home)
	if a.Email == "" {
		a.Email = extractAccountEmail(home)
	}
	if a.Legacy {
		a.CredentialsPresent = IsAntigravityLoggedIn()
	}
	a.State = "verified"
	a.Message = "Provider connection verified"
	if model != "" {
		a.Message += " using " + model
	}
	a.VerifiedAt = &now
	if probeErr != nil {
		a.VerifiedAt = nil
		a.State = "unavailable"
		a.Message = probeErr.Error()
		if !a.CredentialsPresent {
			a.State = "login_required"
		} else if strings.Contains(strings.ToLower(a.Message), "quota") {
			a.State = "quota_limited"
		}
	} else if a.CredentialsPresent {
		_ = prepareAntigravitySettings(home)
		_ = EnsureAntigravityOnboarding(home)
		if !a.Legacy && !antigravityOnboardingComplete(home) {
			a.VerifiedAt = nil
			a.State = "setup_incomplete"
			a.Message = "Google sign-in is saved, but provider setup is incomplete. Finish provider setup before using this account."
		}
	}
	if !a.Legacy {
		accountProfileMu.Lock()
		current, readErr := GetProviderAccount(owner, id)
		if readErr != nil {
			accountProfileMu.Unlock()
			return nil, readErr
		}
		// A probe must not overwrite a concurrent preference edit or newer runtime cooldown.
		a.AllowFailover = current.AllowFailover
		a.AllowModelFallback = current.AllowModelFallback
		a.ModelCooldowns = current.ModelCooldowns
		a.AgentID = current.AgentID
		a.ConnectionID = current.ConnectionID
		if current.LastCheckedAt != nil && current.LastCheckedAt.After(probeStarted) {
			accountProfileMu.Unlock()
			return current, nil
		}
		a.QuotaRetryAt = nil
		if a.State == "quota_limited" {
			dur, _, ok := ParseProviderQuotaResetDuration(a.Message)
			if !ok || dur <= 0 {
				dur = 5 * time.Minute
			}
			retry := now.Add(dur)
			a.QuotaRetryAt = &retry
		}
		err = saveProviderAccount(a)
		accountProfileMu.Unlock()
		if err != nil {
			return nil, err
		}
	}
	return a, nil
}

// Legacy API aliases cannot cancel an account-scoped OAuth session.
func CancelLegacyAntigravityAuthSession(id string) bool {
	authSessionsMu.Lock()
	s := authSessions[id]
	scoped := s != nil && (s.OwnerID != "" || s.AccountID != "")
	authSessionsMu.Unlock()
	if scoped {
		return false
	}
	CancelAntigravityAuthSession(id)
	return true
}

// BindProviderAccount makes the private CLI profile exclusive to one agent/connection.
func BindProviderAccount(owner, id, agentID, connectionID string) error {
	accountProfileMu.Lock()
	defer accountProfileMu.Unlock()
	a, err := GetProviderAccount(owner, id)
	if err != nil {
		return err
	}
	if a.Legacy || !a.CredentialsPresent {
		return errors.New("Complete agent sign-in first")
	}
	if agentID == "" || !strings.HasPrefix(connectionID, "conn_profile_") {
		return errors.New("Invalid agent connection")
	}
	if a.ConnectionID != "" && (a.ConnectionID != connectionID || a.AgentID != agentID) {
		return errors.New("This connection is already used by another agent")
	}
	a.AgentID = agentID
	a.ConnectionID = connectionID
	return saveProviderAccount(a)
}
func UnbindProviderAccount(owner, id, agentID, connectionID string) {
	accountProfileMu.Lock()
	defer accountProfileMu.Unlock()
	a, err := GetProviderAccount(owner, id)
	if err != nil || a.AgentID != agentID || a.ConnectionID != connectionID {
		return
	}
	a.AgentID = ""
	a.ConnectionID = ""
	_ = saveProviderAccount(a)
}
func ProviderAccountForConnection(owner, agentID, connectionID string) (*ProviderAccount, error) {
	list, err := ListProviderAccounts(owner)
	if err != nil {
		return nil, err
	}
	for _, a := range list {
		if !a.Legacy && a.ConnectionID == connectionID && a.AgentID == agentID {
			return a, nil
		}
	}
	return nil, ErrAccountNotFound
}

// Backup participation is an explicit owner choice. Primary account binding never changes.
func SetProviderAccountFailover(owner, id string, enabled bool) (*ProviderAccount, error) {
	accountProfileMu.Lock()
	defer accountProfileMu.Unlock()
	a, err := GetProviderAccount(owner, id)
	if err != nil {
		return nil, err
	}
	if a.Legacy || (enabled && (!a.CredentialsPresent || a.AgentID == "" || a.ConnectionID == "")) {
		return nil, errors.New("Only connected private accounts can be used as backups")
	}
	a.AllowFailover = enabled
	if err := saveProviderAccount(a); err != nil {
		return nil, err
	}
	return a, nil
}

// Runtime observations update metadata only; credentials and primary binding are preserved.
func RecordProviderAccountQuotaWithDuration(owner, id string, dur time.Duration, resetStr string) error {
	accountProfileMu.Lock()
	defer accountProfileMu.Unlock()
	a, err := GetProviderAccount(owner, id)
	if err != nil {
		return err
	}
	if a.Legacy {
		return nil
	}
	now := time.Now().UTC()
	if dur <= 0 {
		dur = 5 * time.Minute
	}
	retry := now.Add(dur)
	a.State = "quota_limited"
	a.VerifiedAt = nil
	a.LastCheckedAt = &now
	a.QuotaRetryAt = &retry
	if resetStr != "" {
		a.Message = fmt.Sprintf("Provider usage limit reached. Resets in %s.", resetStr)
	} else {
		a.Message = "Provider usage limit reached. This account is cooling down before another attempt."
	}
	return saveProviderAccount(a)
}

func RecordProviderAccountQuota(owner, id string) error {
	return RecordProviderAccountQuotaWithDuration(owner, id, 5*time.Minute, "")
}

func SetProviderAccountModelFallback(owner, id string, enabled bool) (*ProviderAccount, error) {
	accountProfileMu.Lock()
	defer accountProfileMu.Unlock()
	a, err := GetProviderAccount(owner, id)
	if err != nil {
		return nil, err
	}
	if a.Legacy {
		return nil, errors.New("Private account required")
	}
	a.AllowModelFallback = enabled
	if err := saveProviderAccount(a); err != nil {
		return nil, err
	}
	return a, nil
}
func RecordProviderModelQuota(owner, id, model string) error {
	if !IsTextModel(model) {
		return RecordProviderAccountQuota(owner, id)
	}
	accountProfileMu.Lock()
	defer accountProfileMu.Unlock()
	a, err := GetProviderAccount(owner, id)
	if err != nil {
		return err
	}
	if a.Legacy {
		return nil
	}
	if a.ModelCooldowns == nil {
		a.ModelCooldowns = map[string]time.Time{}
	}
	for key, until := range a.ModelCooldowns {
		if !time.Now().Before(until) {
			delete(a.ModelCooldowns, key)
		}
	}
	a.ModelCooldowns[model] = time.Now().Add(5 * time.Minute)
	return saveProviderAccount(a)
}

// A location failure applies to the selected route, not every model on an
// account. The empty key represents Auto/provider default. This is backoff,
// never a claim that provider eligibility will change when the timer expires.
func RecordProviderModelRestriction(owner, id, model string) error {
	accountProfileMu.Lock()
	defer accountProfileMu.Unlock()
	a, err := GetProviderAccount(owner, id)
	if err != nil {
		return err
	}
	if a.Legacy {
		return nil
	}
	if a.ModelCooldowns == nil {
		a.ModelCooldowns = map[string]time.Time{}
	}
	for key, until := range a.ModelCooldowns {
		if !time.Now().Before(until) {
			delete(a.ModelCooldowns, key)
		}
	}
	a.ModelCooldowns[model] = time.Now().Add(5 * time.Minute)
	return saveProviderAccount(a)
}
