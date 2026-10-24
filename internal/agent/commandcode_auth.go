package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type CommandCodeAuthSession struct {
	ID            string    `json:"id"`
	OwnerID       string    `json:"-"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	Authenticated bool      `json:"authenticated"`
	Verified      bool      `json:"verified"`
	Message       string    `json:"message,omitempty"`
}

var commandCodeAuthMu sync.Mutex
var commandCodeAuth = map[string]*CommandCodeAuthSession{}

func commandCodeAuthPath() string {
	return filepath.Join(ConfigHome(), ".local", "share", "cuan", "commandcode-auth.json")
}
func loadCommandCodeAuthLocked() {
	if len(commandCodeAuth) > 0 {
		return
	}
	b, e := os.ReadFile(commandCodeAuthPath())
	if e != nil {
		return
	}
	var all map[string]*CommandCodeAuthSession
	if json.Unmarshal(b, &all) == nil {
		commandCodeAuth = all
	}
}
func persistCommandCodeAuthLocked() {
	p := commandCodeAuthPath()
	_ = os.MkdirAll(filepath.Dir(p), 0700)
	b, _ := json.Marshal(commandCodeAuth)
	tmp := p + ".tmp"
	if os.WriteFile(tmp, b, 0600) == nil {
		_ = os.Rename(tmp, p)
	}
}
func StartCommandCodeAuth(owner string) *CommandCodeAuthSession {
	commandCodeAuthMu.Lock()
	defer commandCodeAuthMu.Unlock()
	loadCommandCodeAuthLocked()
	now := time.Now().UTC()
	for _, s := range commandCodeAuth {
		if s.OwnerID == owner && s.Status == "pending" && now.Before(s.ExpiresAt) {
			c := *s
			return &c
		}
	}
	id := generateHexID("ccauth")
	s := &CommandCodeAuthSession{ID: id, OwnerID: owner, Status: "pending", CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(5 * time.Minute), Message: "Sign in with your CommandCode subscription, then retry verification."}
	commandCodeAuth[id] = s
	persistCommandCodeAuthLocked()
	c := *s
	return &c
}
func GetCommandCodeAuth(id, owner string) (*CommandCodeAuthSession, error) {
	commandCodeAuthMu.Lock()
	defer commandCodeAuthMu.Unlock()
	loadCommandCodeAuthLocked()
	s := commandCodeAuth[id]
	if s == nil || s.OwnerID != owner {
		return nil, errors.New("Sign-in session is unavailable")
	}
	if time.Now().UTC().After(s.ExpiresAt) && s.Status == "pending" {
		s.Status = "expired"
		s.UpdatedAt = time.Now().UTC()
		persistCommandCodeAuthLocked()
	}
	c := *s
	return &c, nil
}
func CompleteCommandCodeAuth(id, owner string, authenticated, verified bool, message string) (*CommandCodeAuthSession, error) {
	commandCodeAuthMu.Lock()
	defer commandCodeAuthMu.Unlock()
	loadCommandCodeAuthLocked()
	s := commandCodeAuth[id]
	if s == nil || s.OwnerID != owner {
		return nil, errors.New("Sign-in session is unavailable")
	}
	if s.Status == "cancelled" || s.Status == "expired" {
		return nil, errors.New("Sign-in session is no longer active")
	}
	if time.Now().UTC().After(s.ExpiresAt) {
		s.Status = "expired"
		s.UpdatedAt = time.Now().UTC()
		persistCommandCodeAuthLocked()
		return nil, errors.New("Sign-in session expired")
	}
	s.Authenticated = authenticated
	s.Verified = verified
	s.Status = "authenticated"
	s.Message = message
	s.UpdatedAt = time.Now().UTC()
	persistCommandCodeAuthLocked()
	c := *s
	return &c, nil
}
func CancelCommandCodeAuth(id, owner string) error {
	commandCodeAuthMu.Lock()
	defer commandCodeAuthMu.Unlock()
	loadCommandCodeAuthLocked()
	s := commandCodeAuth[id]
	if s == nil || s.OwnerID != owner {
		return errors.New("Sign-in session is unavailable")
	}
	if s.Status == "cancelled" {
		return nil
	}
	if s.Status == "authenticated" {
		return nil
	}
	s.Status = "cancelled"
	s.UpdatedAt = time.Now().UTC()
	persistCommandCodeAuthLocked()
	return nil
}
