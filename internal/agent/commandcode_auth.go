package agent

import (
	"errors"
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

func StartCommandCodeAuth(owner string) *CommandCodeAuthSession {
	commandCodeAuthMu.Lock()
	defer commandCodeAuthMu.Unlock()
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
	c := *s
	return &c
}
func GetCommandCodeAuth(id, owner string) (*CommandCodeAuthSession, error) {
	commandCodeAuthMu.Lock()
	defer commandCodeAuthMu.Unlock()
	s := commandCodeAuth[id]
	if s == nil || s.OwnerID != owner {
		return nil, errors.New("Sign-in session is unavailable")
	}
	if time.Now().UTC().After(s.ExpiresAt) && s.Status == "pending" {
		s.Status = "expired"
		s.UpdatedAt = time.Now().UTC()
	}
	c := *s
	return &c, nil
}
func CompleteCommandCodeAuth(id, owner string, authenticated, verified bool, message string) (*CommandCodeAuthSession, error) {
	commandCodeAuthMu.Lock()
	defer commandCodeAuthMu.Unlock()
	s := commandCodeAuth[id]
	if s == nil || s.OwnerID != owner {
		return nil, errors.New("Sign-in session is unavailable")
	}
	if time.Now().UTC().After(s.ExpiresAt) {
		s.Status = "expired"
		s.UpdatedAt = time.Now().UTC()
		return nil, errors.New("Sign-in session expired")
	}
	s.Authenticated = authenticated
	s.Verified = verified
	s.Status = "authenticated"
	s.Message = message
	s.UpdatedAt = time.Now().UTC()
	c := *s
	return &c, nil
}
func CancelCommandCodeAuth(id, owner string) error {
	commandCodeAuthMu.Lock()
	defer commandCodeAuthMu.Unlock()
	s := commandCodeAuth[id]
	if s == nil || s.OwnerID != owner {
		return errors.New("Sign-in session is unavailable")
	}
	if s.Status == "cancelled" {
		return nil
	}
	s.Status = "cancelled"
	s.UpdatedAt = time.Now().UTC()
	return nil
}
