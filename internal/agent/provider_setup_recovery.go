package agent

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/creack/pty"
)

// FinishProviderAccountSetup resumes the provider's own wizard. It never writes
// a completion flag, replaces credentials, sends an AI prompt or opts into sharing.
func FinishProviderAccountSetup(ctx context.Context, owner, id string, acceptTerms bool) error {
	if !acceptTerms {
		return errors.New("Accept the provider terms to finish setup")
	}
	home, err := ProviderAccountHome(owner, id)
	if err != nil || home == "" || !IsAntigravityLoggedInInDir(home) {
		return errors.New("Saved Google sign-in is required to finish setup")
	}
	if antigravityOnboardingComplete(home) {
		return nil
	}
	release, err := TryLeaseProviderAccount(owner, id)
	if err != nil {
		return err
	}
	defer release()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// Reuse a live login rather than opening a second wizard on the same profile.
	authSessionsMu.Lock()
	var live *AntigravityAuthSession
	for _, s := range authSessions {
		if s.OwnerID != owner || s.AccountID != id || s.ProfileHome != home {
			continue
		}
		select {
		case <-s.Done:
			continue
		default:
			live = s
		}
	}
	authSessionsMu.Unlock()
	var done <-chan struct{}
	if live != nil {
		live.submitMu.Lock()
		defer live.submitMu.Unlock()
		if terminal, ok := live.Stdin.(*os.File); ok {
			if err := pty.Setsize(terminal, &pty.Winsize{Rows: 40, Cols: 120}); err != nil {
				return err
			}
		}
		live.setup.accept()
		done = live.Done
	} else {
		bin, err := exec.LookPath("agy")
		if err != nil {
			return errors.New("AI command is unavailable")
		}
		if err := prepareAntigravitySettings(home); err != nil {
			return err
		}
		cmd := exec.CommandContext(ctx, bin)
		cmd.Dir = home
		cmd.Env = AntigravityProfileEnvironment(home)
		cmd.Cancel = func() error { _ = KillProcessTree(cmd.Process.Pid); return cmd.Process.Kill() }
		terminal, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 40, Cols: 120})
		if err != nil {
			return err
		}
		defer terminal.Close()
		defer func() { _ = KillProcessTree(cmd.Process.Pid); _ = cmd.Process.Kill() }()
		driver := &cliSetupDriver{writer: terminal}
		driver.accept()
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := terminal.Read(buf)
				if n > 0 {
					driver.feed(string(buf[:n]))
				}
				if err != nil {
					return
				}
			}
		}()
		exited := make(chan struct{})
		done = exited
		go func() { _ = cmd.Wait(); close(exited) }()
	}
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		if antigravityOnboardingComplete(home) {
			if live != nil {
				CancelAntigravityAuthSession(live.ID)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return errors.New("Google sign-in is saved, but provider setup has not finished. Try Finish setup again")
		case <-done:
			if antigravityOnboardingComplete(home) {
				return nil
			}
			return io.ErrUnexpectedEOF
		case <-tick.C:
		}
	}
}
