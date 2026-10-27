package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
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
	AuthURL       string    `json:"auth_url,omitempty"`
	cmd           *exec.Cmd
	terminal      *os.File
	callbackPort  int
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
	loadCommandCodeAuthLocked()
	now := time.Now().UTC()
	for _, s := range commandCodeAuth {
		if s.OwnerID == owner && s.Status == "pending" && now.Before(s.ExpiresAt) {
			c := *s
			commandCodeAuthMu.Unlock()
			return &c
		}
	}
	id := generateHexID("ccauth")
	s := &CommandCodeAuthSession{ID: id, OwnerID: owner, Status: "pending", CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(5 * time.Minute), Message: "Complete CommandCode sign-in in your browser."}
	commandCodeAuth[id] = s
	persistCommandCodeAuthLocked()
	commandCodeAuthMu.Unlock()
	// CommandCode's official login flow requires a TTY. Start it server-side
	// and expose the resulting browser URL to the Cuan UI.
	cmd := exec.Command("commandcode", "login")
	term, err := pty.Start(cmd)
	if err == nil {
		s.cmd, s.terminal = cmd, term
		go readCommandCodeLogin(s)
		for i := 0; i < 20; i++ {
			time.Sleep(100 * time.Millisecond)
			commandCodeAuthMu.Lock()
			ready := s.AuthURL != ""
			commandCodeAuthMu.Unlock()
			if ready {
				break
			}
		}
	} else {
		s.Message = fmt.Sprintf("Could not start CommandCode login: %v", err)
	}
	commandCodeAuthMu.Lock()
	persistCommandCodeAuthLocked()
	c := *s
	commandCodeAuthMu.Unlock()
	return &c
}

var commandCodeAuthURL = regexp.MustCompile(`https://commandcode\.ai/studio/auth/cli\?[^\s\x1b]+`)
var commandCodeCallback = regexp.MustCompile(`callback=http%3A%2F%2F127\.0\.0\.1%3A([0-9]+)%2Fcallback`)

func readCommandCodeLogin(s *CommandCodeAuthSession) {
	if s.terminal == nil {
		return
	}
	buf := make([]byte, 4096)
	var output strings.Builder
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for {
		read := make(chan int, 1)
		var n int
		go func() { n, _ = s.terminal.Read(buf); read <- n }()
		select {
		case <-deadline.C:
			return
		case n = <-read:
			if n <= 0 {
				return
			}
			output.Write(buf[:n])
			if m := commandCodeAuthURL.FindString(output.String()); m != "" {
				commandCodeAuthMu.Lock()
				s.AuthURL = strings.ReplaceAll(m, "\x1b", "")
				if p := commandCodeCallback.FindStringSubmatch(s.AuthURL); len(p) == 2 {
					s.callbackPort, _ = strconv.Atoi(p[1])
				}
				s.Message = "Open the CommandCode sign-in link to continue."
				s.UpdatedAt = time.Now().UTC()
				persistCommandCodeAuthLocked()
				commandCodeAuthMu.Unlock()
				return
			}
		}
	}
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

// CommandCodeAuthURL rewrites the CLI's localhost callback through Cuan so a
// browser on the user's device can complete a login running on the server.
func CommandCodeAuthURL(id, raw, publicBase string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("callback", strings.TrimRight(publicBase, "/")+"/api/v1/agents/commandcode/callback?session_id="+url.QueryEscape(id))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func ForwardCommandCodeCallback(id string, values url.Values) error {
	commandCodeAuthMu.Lock()
	loadCommandCodeAuthLocked()
	s := commandCodeAuth[id]
	port := 0
	if s != nil {
		port = s.callbackPort
	}
	commandCodeAuthMu.Unlock()
	if port == 0 {
		return errors.New("CommandCode login session is not ready")
	}
	values.Del("session_id")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Get("http://127.0.0.1:" + strconv.Itoa(port) + "/callback?" + values.Encode())
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
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
