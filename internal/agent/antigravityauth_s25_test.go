package agent

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestS25RejectCredentialInjection(t *testing.T) {
	if err := SubmitAntigravityAuthCode("missing", `{"token":"arbitrary-token-value"}`); err == nil {
		t.Fatal("credential JSON must not be accepted")
	}
	if err := SubmitAntigravityAuthCode("missing", "invalid-code"); err == nil {
		t.Fatal("unknown session must fail")
	}
	if err := SubmitAntigravityAuthCode("missing", "one\ntwo"); err == nil {
		t.Fatal("terminal input injection must fail")
	}
}

func TestS25ExpiredSession(t *testing.T) {
	id := "expired-test"
	authSessionsMu.Lock()
	authSessions[id] = &AntigravityAuthSession{ID: id, CreatedAt: time.Now().Add(-6 * time.Minute)}
	authSessionsMu.Unlock()
	if err := SubmitAntigravityAuthCode(id, "code"); err == nil {
		t.Fatal("expired session accepted")
	}
}

func TestS25RealOAuthURL(t *testing.T) {
	if os.Getenv("CUAN_VERIFY_REAL_OAUTH") != "1" {
		t.Skip("explicit real CLI check only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	session, err := StartAntigravityAuthSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer CancelAntigravityAuthSession(session.ID)
	if !validGoogleAuthURL(session.AuthURL) {
		t.Fatal("official provider URL not found")
	}
	t.Log("Official Google OAuth URL captured from real interactive agy; URL and session withheld. Account owner must consent and provide authorization code.")
}

type s25EnterWriter struct {
	input []byte
	done  chan struct{}
}

func (w *s25EnterWriter) Write(input []byte) (int, error) {
	w.input = append(w.input, input...)
	close(w.done)
	return len(input), nil
}
func (w *s25EnterWriter) Close() error { return nil }

func TestS25AuthorizationCodeUsesTerminalEnter(t *testing.T) {
	id := "pty-enter-regression"
	done := make(chan struct{})
	writer := &s25EnterWriter{done: done}
	session := &AntigravityAuthSession{ID: id, CreatedAt: time.Now(), Done: done, Stdin: writer}
	authSessionsMu.Lock()
	authSessions[id] = session
	authSessionsMu.Unlock()
	defer CancelAntigravityAuthSession(id)
	_ = SubmitAntigravityAuthCode(id, "diagnostic-code")
	if string(writer.input) != "diagnostic-code\r" {
		t.Fatal("OAuth PTY must receive the Enter key, not a newline")
	}
}
