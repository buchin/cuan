package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/git"
	"github.com/go-chi/chi/v5"
)

type githubDeviceStatus struct {
	ID              string    `json:"id"`
	State           string    `json:"state"`
	UserCode        string    `json:"user_code,omitempty"`
	VerificationURL string    `json:"verification_url,omitempty"`
	ExpiresAt       time.Time `json:"expires_at"`
	Error           string    `json:"error,omitempty"`
}
type githubDeviceLogin struct {
	mu     sync.Mutex
	owner  string
	status githubDeviceStatus
	cancel context.CancelFunc
	done   chan struct{}
}

func (j *githubDeviceLogin) snapshot() githubDeviceStatus {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.status
}
func githubDevicePending(state string) bool {
	return state == "preparing" || state == "starting" || state == "awaiting_authorization"
}

func (s *Server) existingGitHubDeviceLogin(owner string) *githubDeviceLogin {
	s.githubDeviceMu.Lock()
	defer s.githubDeviceMu.Unlock()
	for _, job := range s.githubDeviceJobs {
		status := job.snapshot()
		if job.owner == owner && githubDevicePending(status.State) && time.Now().Before(status.ExpiresAt) {
			return job
		}
	}
	return nil
}
func (s *Server) beginGitHubDeviceLogin(w http.ResponseWriter, r *http.Request, owner string) {
	s.githubDeviceMu.Lock()
	for id, job := range s.githubDeviceJobs {
		status := job.snapshot()
		if job.owner == owner && githubDevicePending(status.State) && time.Now().Before(status.ExpiresAt) {
			s.githubDeviceMu.Unlock()
			respondJSON(w, 200, map[string]any{"device_login": status})
			return
		}
		if time.Now().After(status.ExpiresAt.Add(5 * time.Minute)) {
			delete(s.githubDeviceJobs, id)
		}
	}
	if len(s.githubDeviceJobs) >= 100 {
		s.githubDeviceMu.Unlock()
		respondJSONError(w, 429, "Too many pending sign-ins. Try again later.")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	job := &githubDeviceLogin{owner: owner, status: githubDeviceStatus{ID: db.GenerateUUID(), State: "preparing", ExpiresAt: time.Now().Add(15 * time.Minute)}, cancel: cancel, done: make(chan struct{})}
	if s.githubDeviceJobs == nil {
		s.githubDeviceJobs = map[string]*githubDeviceLogin{}
	}
	s.githubDeviceJobs[job.status.ID] = job
	s.githubDeviceMu.Unlock()
	go s.runGitHubDeviceLogin(ctx, job)
	respondJSON(w, 200, map[string]any{"device_login": job.snapshot()})
}
func (s *Server) handleGitHubDeviceLogin(w http.ResponseWriter, r *http.Request) {
	owner := s.accountOwner(w, r)
	if owner == "" {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	id := chi.URLParam(r, "loginID")
	s.githubDeviceMu.Lock()
	job := s.githubDeviceJobs[id]
	s.githubDeviceMu.Unlock()
	if job == nil || job.owner != owner {
		respondJSONError(w, 404, "This sign-in has expired. Start again.")
		return
	}
	if r.Method == http.MethodPost {
		job.mu.Lock()
		if githubDevicePending(job.status.State) {
			job.status.State = "cancelled"
			job.status.UserCode = ""
			job.cancel()
		}
		job.mu.Unlock()
	}
	respondJSON(w, 200, map[string]any{"device_login": job.snapshot()})
}
func (s *Server) cancelGitHubDeviceOwner(owner string) {
	s.githubDeviceMu.Lock()
	defer s.githubDeviceMu.Unlock()
	for _, job := range s.githubDeviceJobs {
		if owner == "" || job.owner == owner {
			job.mu.Lock()
			if githubDevicePending(job.status.State) {
				job.status.State = "cancelled"
				job.status.UserCode = ""
				job.cancel()
			}
			job.mu.Unlock()
		}
	}
}

// gh must never inherit Fural's GH_TOKEN, global gh config, keyring or git
// credentials. Its temporary private profile is removed after token extraction.
func githubDeviceEnvironment(home string) []string {
	return []string{"HOME=" + home, "GH_CONFIG_DIR=" + filepath.Join(home, ".config", "gh"), "GH_BROWSER=/bin/true", "BROWSER=/bin/true", "GH_PROMPT_DISABLED=1", "GIT_TERMINAL_PROMPT=0", "PATH=/usr/local/bin:/usr/bin:/bin", "LANG=C.UTF-8", "LC_ALL=C.UTF-8", "TERM=dumb", "NO_COLOR=1"}
}
func githubDeviceCommand(ctx context.Context, binary, home string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = githubDeviceEnvironment(home)
	cmd.Dir = home
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
	cmd.WaitDelay = 2 * time.Second
	return cmd
}

var githubOneTimeCode = regexp.MustCompile(`(?i)one-time code:\s*([A-Z0-9]{4}-[A-Z0-9]{4})`)

type githubDeviceOutput struct {
	mu   sync.Mutex
	data []byte
	job  *githubDeviceLogin
}

func (out *githubDeviceOutput) Write(p []byte) (int, error) {
	out.mu.Lock()
	defer out.mu.Unlock()
	if remaining := 65536 - len(out.data); remaining > 0 {
		n := len(p)
		if n > remaining {
			n = remaining
		}
		out.data = append(out.data, p[:n]...)
	}
	match := githubOneTimeCode.FindSubmatch(out.data)
	if len(match) == 2 {
		out.job.mu.Lock()
		if githubDevicePending(out.job.status.State) {
			out.job.status.State = "awaiting_authorization"
			out.job.status.UserCode = string(match[1])
			out.job.status.VerificationURL = "https://github.com/login/device"
		}
		out.job.mu.Unlock()
	}
	return len(p), nil
}
func (s *Server) runGitHubDeviceLogin(ctx context.Context, job *githubDeviceLogin) {
	defer close(job.done)
	defer job.cancel()
	fail := func(message string) {
		job.mu.Lock()
		defer job.mu.Unlock()
		if job.status.State == "cancelled" {
			return
		}
		job.status.State = "failed"
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			job.status.State = "expired"
			message = "GitHub sign-in expired. Start again."
		}
		job.status.Error = message
		job.status.UserCode = ""
	}
	binary, err := ensureGitHubDeviceBinary(ctx)
	if err != nil {
		fail("Could not prepare GitHub sign-in. Check the installation's internet connection and try again.")
		return
	}
	root, err := os.UserHomeDir()
	if err != nil {
		fail("Could not prepare private GitHub sign-in storage.")
		return
	}
	parent := filepath.Join(root, ".local", "share", "cuan", "github-device")
	if os.MkdirAll(parent, 0700) != nil {
		fail("Could not prepare private GitHub sign-in storage.")
		return
	}
	home, err := os.MkdirTemp(parent, "login-*")
	if err != nil {
		fail("Could not prepare private GitHub sign-in storage.")
		return
	}
	defer os.RemoveAll(home)
	job.mu.Lock()
	if ctx.Err() != nil {
		job.mu.Unlock()
		fail("GitHub sign-in expired. Start again.")
		return
	}
	job.status.State = "starting"
	job.mu.Unlock()
	cmd := githubDeviceCommand(ctx, binary, home, "auth", "login", "--hostname", "github.com", "--git-protocol", "https", "--web", "--insecure-storage")
	cmd.Stdin = strings.NewReader("\n")
	output := &githubDeviceOutput{job: job}
	cmd.Stdout = output
	cmd.Stderr = output
	if cmd.Run() != nil {
		fail("GitHub sign-in was not completed. Try again.")
		return
	}
	tokenCmd := githubDeviceCommand(ctx, binary, home, "auth", "token", "--hostname", "github.com")
	tokenOutput := &boundedInboxOutput{}
	tokenCmd.Stdout = tokenOutput
	tokenCmd.Stderr = &boundedInboxOutput{}
	if tokenCmd.Run() != nil {
		fail("Could not finish GitHub sign-in. Try again.")
		return
	}
	token := strings.TrimSpace(tokenOutput.String())
	if token == "" || len(token) > 4096 {
		fail("Could not finish GitHub sign-in. Try again.")
		return
	}
	client := git.NewClient()
	client.SetAPIBaseURL(githubOAuthAPIBase)
	account, err := client.Account(ctx, token)
	if err != nil {
		fail("Could not verify your GitHub account. Try again.")
		return
	}
	path, err := githubConnectionPath(job.owner)
	if err != nil {
		fail("Could not save GitHub access. Try again.")
		return
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	if ctx.Err() != nil || job.status.State == "cancelled" {
		return
	}
	githubConnectionMu.Lock()
	err = git.SaveConnection(path, s.cfg.AuthSecret, token, account)
	githubConnectionMu.Unlock()
	if err != nil {
		job.status.State = "failed"
		job.status.Error = "Could not save GitHub access. Try again."
		return
	}
	job.status.State = "connected"
	job.status.UserCode = ""
	job.status.VerificationURL = ""
	job.status.Error = ""
}
