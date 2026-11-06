// Package publication independently observes registered production releases.
// It does not execute deployments or accept agent-supplied receipts.
package publication

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

var ErrUnavailable = errors.New("registered publication verifier is unavailable")
var ErrUnverified = errors.New("production revision or health could not be verified")
var revisionPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var releasePathPattern = regexp.MustCompile(`^/[A-Za-z0-9_./-]+$`)

type Config struct{ Host, User, Password, HostKey, DeployPath, HealthURL string }

// References resolve exclusively within the owner's saved project environment.
// No process environment, SSH agent or default developer profile is consulted.
func FromEnvironment(env map[string]string) (Config, error) {
	if env["CUAN_PUBLICATION_VERIFIER"] != "ssh-release" {
		return Config{}, ErrUnavailable
	}
	value := func(key string) string {
		key = "CUAN_PUBLICATION_" + key
		if ref := env[key+"_ENV"]; ref != "" {
			return env[ref]
		}
		return env[key]
	}
	c := Config{Host: value("SSH_HOST"), User: value("SSH_USER"), Password: value("SSH_PASSWORD"), HostKey: value("SSH_HOST_KEY"), DeployPath: value("DEPLOY_PATH"), HealthURL: value("HEALTH_URL")}
	if c.validate() != nil {
		return Config{}, ErrUnavailable
	}
	return c, nil
}

func (c Config) validate() error {
	if c.Host == "" || strings.ContainsAny(c.Host, "/@\r\n\t ") || c.User == "" || c.Password == "" || c.HostKey == "" || !releasePathPattern.MatchString(c.DeployPath) || path.Clean(c.DeployPath) != c.DeployPath || c.DeployPath == "/" {
		return ErrUnavailable
	}
	u, e := url.Parse(c.HealthURL)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return ErrUnavailable
	}
	key, _, options, rest, e := ssh.ParseAuthorizedKey([]byte(c.HostKey))
	if e != nil || key == nil || len(options) != 0 || len(bytes.TrimSpace(rest)) != 0 {
		return ErrUnavailable
	}
	return nil
}

type boundedOutput struct{ bytes.Buffer }

func (b *boundedOutput) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 4096 {
		return 0, ErrUnverified
	}
	return b.Buffer.Write(p)
}

// Verify reads an immutable Deployer-style current/releases/REVISION binding
// using a pinned host identity, then checks the registered HTTPS health target.
// Every command is fixed and read-only; no model command or result is executed.
func Verify(ctx context.Context, c Config, expected string) error {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	defer transport.CloseIdleConnections()
	return verify(ctx, c, expected, transport)
}

func verify(ctx context.Context, c Config, expected string, transport http.RoundTripper) error {
	if !revisionPattern.MatchString(expected) {
		return ErrUnavailable
	}
	observed, err := observe(ctx, c, expected, transport)
	if err != nil {
		return err
	}
	if observed != expected {
		return ErrUnverified
	}
	return nil
}

// Observe identifies the existing registered release, without attributing it to
// a task or issuing a deployment receipt. HTTPS health is checked separately.
func Observe(ctx context.Context, c Config) (string, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	defer transport.CloseIdleConnections()
	return observe(ctx, c, "", transport)
}

func observe(ctx context.Context, c Config, expected string, transport http.RoundTripper) (string, error) {
	if c.validate() != nil {
		return "", ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	address := c.Host
	if net.ParseIP(address) != nil || !strings.Contains(address, ":") {
		address = net.JoinHostPort(address, "22")
	} else if _, _, err := net.SplitHostPort(address); err != nil {
		return "", ErrUnavailable
	}
	key, _, _, _, _ := ssh.ParseAuthorizedKey([]byte(c.HostKey))
	socket, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", address)
	if err != nil {
		return "", fmt.Errorf("%w: SSH connection unavailable", ErrUnverified)
	}
	defer socket.Close()
	stop := context.AfterFunc(ctx, func() { _ = socket.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		_ = socket.SetDeadline(deadline)
	}
	algorithms := []string{key.Type()}
	if key.Type() == ssh.KeyAlgoRSA {
		algorithms = []string{ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256}
	}
	connection, channels, requests, err := ssh.NewClientConn(socket, address, &ssh.ClientConfig{User: c.User, Auth: []ssh.AuthMethod{ssh.Password(c.Password)}, HostKeyCallback: ssh.FixedHostKey(key), HostKeyAlgorithms: algorithms, Timeout: 5 * time.Second})
	if err != nil {
		return "", fmt.Errorf("%w: SSH identity or authentication rejected", ErrUnverified)
	}
	client := ssh.NewClient(connection, channels, requests)
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("%w: SSH session unavailable", ErrUnverified)
	}
	defer session.Close()
	var output boundedOutput
	session.Stdout = &output
	session.Stderr = io.Discard
	command := `set -eu; current=$(readlink -f -- '` + c.DeployPath + `/current'); case "$current" in '` + c.DeployPath + `/releases/'*) ;; *) exit 42;; esac; cat -- "$current/REVISION"`
	if session.Run(command) != nil {
		return "", fmt.Errorf("%w: immutable revision unreadable", ErrUnverified)
	}
	revision := strings.TrimSpace(output.String())
	if !revisionPattern.MatchString(revision) || (expected != "" && revision != expected) {
		return "", fmt.Errorf("%w: immutable revision mismatch or unreadable", ErrUnverified)
	}
	health := &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.HealthURL, nil)
	if err != nil {
		return "", ErrUnverified
	}
	resp, err := health.Do(req)
	if err != nil {
		return "", ErrUnverified
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: health response is not healthy", ErrUnverified)
	}
	return revision, nil
}
