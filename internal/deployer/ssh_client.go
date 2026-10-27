package deployer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/db"
	"golang.org/x/crypto/ssh"
)

var (
	ErrSSHConnectionTimeout = errors.New("ssh connection timeout")
	ErrSSHAuthFailed        = errors.New("ssh authentication failed")
	ErrSSHHostUnreachable   = errors.New("ssh host unreachable")
	ErrSSHClosed            = errors.New("ssh client is closed")
)

const (
	DefaultSSHTimeout         = 5 * time.Second
	DefaultSSHPort            = 22
	DefaultSSHUser            = "ubuntu"
	DefaultKeepaliveInterval  = 30 * time.Second
	ProbeScript               = `echo "===CUAN_OS==="
cat /etc/os-release 2>/dev/null || uname -a
echo "===CUAN_UNAME==="
uname -s -r -m 2>/dev/null || uname -a
echo "===CUAN_CPU==="
nproc 2>/dev/null || grep -c ^processor /proc/cpuinfo 2>/dev/null || echo 1
echo "===CUAN_RAM==="
free -m 2>/dev/null || cat /proc/meminfo 2>/dev/null
echo "===CUAN_HOST==="
hostname 2>/dev/null || echo ""
echo "===CUAN_END==="`
)

// SSHConfig holds configuration parameters for establishing an SSH connection.
type SSHConfig struct {
	Host              string
	Port              int
	Username          string
	PrivateKey        []byte
	Timeout           time.Duration
	KeepaliveInterval time.Duration
}

// SystemSpecs holds remote server hardware and OS specifications extracted by the health probe.
type SystemSpecs struct {
	OS       string `json:"os"`
	Distro   string `json:"distro,omitempty"`
	Kernel   string `json:"kernel,omitempty"`
	Arch     string `json:"arch,omitempty"`
	Cores    int    `json:"cores"`
	RAMMB    int    `json:"ram_mb"`
	Hostname string `json:"hostname,omitempty"`
	PingMS   int64  `json:"ping_ms"`
}

// ToMap converts SystemSpecs to a map[string]interface{} suitable for storage in db.Server.OSInfo.
func (s *SystemSpecs) ToMap() map[string]interface{} {
	m := map[string]interface{}{
		"os":      s.OS,
		"cores":   s.Cores,
		"ram_mb":  s.RAMMB,
		"ping_ms": s.PingMS,
	}
	if s.Distro != "" {
		m["distro"] = s.Distro
	}
	if s.Kernel != "" {
		m["kernel"] = s.Kernel
	}
	if s.Arch != "" {
		m["arch"] = s.Arch
	}
	if s.Hostname != "" {
		m["hostname"] = s.Hostname
	}
	return m
}

// SSHClient represents a managed SSH client connection with keepalive and command runner.
type SSHClient struct {
	config        SSHConfig
	netConn       net.Conn
	client        *ssh.Client
	mu            sync.Mutex
	closed        bool
	lastUsed      time.Time
	stopKeepalive chan struct{}
}

// DialSSH connects to the remote host using SSH key authentication within the 5s timeout.
func DialSSH(ctx context.Context, cfg SSHConfig) (*SSHClient, error) {
	if cfg.Port <= 0 {
		cfg.Port = DefaultSSHPort
	}
	if cfg.Username == "" {
		cfg.Username = DefaultSSHUser
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultSSHTimeout
	}

	signer, err := ssh.ParsePrivateKey(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid private key format: %v", ErrSSHAuthFailed, err)
	}

	sshConfig := &ssh.ClientConfig{
		User: cfg.Username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         cfg.Timeout,
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	d := net.Dialer{Timeout: cfg.Timeout}
	netConn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || strings.Contains(err.Error(), "timeout") {
			return nil, fmt.Errorf("%w: %v", ErrSSHConnectionTimeout, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrSSHHostUnreachable, err)
	}

	// Set deadline on raw TCP connection to enforce handshake timeout bounds
	_ = netConn.SetDeadline(time.Now().Add(cfg.Timeout))
	conn, chans, reqs, err := ssh.NewClientConn(netConn, addr, sshConfig)
	if err != nil {
		_ = netConn.Close()
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "unable to authenticate") || strings.Contains(errMsg, "authentication") {
			return nil, fmt.Errorf("%w: %v", ErrSSHAuthFailed, err)
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || strings.Contains(errMsg, "timeout") {
			return nil, fmt.Errorf("%w: %v", ErrSSHConnectionTimeout, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrSSHHostUnreachable, err)
	}
	// Reset deadline for normal persistent operations
	_ = netConn.SetDeadline(time.Time{})

	client := ssh.NewClient(conn, chans, reqs)

	sc := &SSHClient{
		config:        cfg,
		netConn:       netConn,
		client:        client,
		lastUsed:      time.Now(),
		stopKeepalive: make(chan struct{}),
	}

	if cfg.KeepaliveInterval > 0 {
		sc.startKeepalive(cfg.KeepaliveInterval)
	}

	return sc, nil
}

// Client returns the underlying *ssh.Client.
func (c *SSHClient) Client() *ssh.Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.client
}

// RunCommandWithInput executes a shell command on the remote host with optional stdin streaming.
func (c *SSHClient) RunCommandWithInput(ctx context.Context, cmd string, stdin io.Reader) (string, string, error) {
	c.mu.Lock()
	if c.closed || c.client == nil {
		c.mu.Unlock()
		return "", "", ErrSSHClosed
	}
	client := c.client
	c.lastUsed = time.Now()
	c.mu.Unlock()

	session, err := client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("create ssh session: %w", err)
	}
	defer session.Close()

	if stdin != nil {
		session.Stdin = stdin
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	done := make(chan error, 1)
	go func() {
		done <- session.Run(cmd)
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGKILL)
		_ = session.Close()
		return stdoutBuf.String(), stderrBuf.String(), ctx.Err()
	case err := <-done:
		return stdoutBuf.String(), stderrBuf.String(), err
	}
}

// RunCommand executes a shell command on the remote host, capturing stdout and stderr.
func (c *SSHClient) RunCommand(ctx context.Context, cmd string) (string, string, error) {
	return c.RunCommandWithInput(ctx, cmd, nil)
}

// Ping measures round-trip latency to the remote host.
func (c *SSHClient) Ping(ctx context.Context) (time.Duration, error) {
	start := time.Now()
	_, _, err := c.RunCommand(ctx, "echo 1")
	if err != nil {
		return 0, err
	}
	return time.Since(start), nil
}

// Probe executes remote system probe commands and parses the hardware/OS specifications.
func (c *SSHClient) Probe(ctx context.Context) (*SystemSpecs, error) {
	start := time.Now()
	stdout, _, err := c.RunCommand(ctx, ProbeScript)
	if err != nil {
		return nil, fmt.Errorf("probe command execution failed: %w", err)
	}
	pingDuration := time.Since(start)
	pingMS := pingDuration.Milliseconds()
	if pingMS <= 0 {
		pingMS = 1
	}

	specs := ParseProbeOutput(stdout, pingMS)
	return specs, nil
}

// IsClosed checks whether the SSH client connection is closed.
func (c *SSHClient) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

// Close closes the SSH connection and stops background keepalive goroutines.
func (c *SSHClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	close(c.stopKeepalive)

	var err error
	if c.client != nil {
		err = c.client.Close()
	}
	if c.netConn != nil {
		_ = c.netConn.Close()
	}
	return err
}

func (c *SSHClient) startKeepalive(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-c.stopKeepalive:
				return
			case <-ticker.C:
				c.mu.Lock()
				closed := c.closed
				client := c.client
				c.mu.Unlock()
				if closed || client == nil {
					return
				}
				_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
				if err != nil {
					_ = c.Close()
					return
				}
			}
		}
	}()
}

// ParseProbeOutput parses the raw multi-section output of the probe script into SystemSpecs.
func ParseProbeOutput(raw string, pingMS int64) *SystemSpecs {
	if pingMS <= 0 {
		pingMS = 1
	}

	osSec := extractSection(raw, "===CUAN_OS===", "===CUAN_UNAME===")
	unameSec := extractSection(raw, "===CUAN_UNAME===", "===CUAN_CPU===")
	cpuSec := extractSection(raw, "===CUAN_CPU===", "===CUAN_RAM===")
	ramSec := extractSection(raw, "===CUAN_RAM===", "===CUAN_HOST===")
	hostSec := extractSection(raw, "===CUAN_HOST===", "===CUAN_END===")

	// If delimiters were absent (e.g. single command output), fallback gracefully
	if osSec == "" && unameSec == "" && cpuSec == "" && ramSec == "" {
		osSec = raw
		unameSec = raw
		cpuSec = raw
		ramSec = raw
		hostSec = ""
	}

	osName, distro := parseOSSection(osSec)
	kernel, arch := parseUnameSection(unameSec)
	cores := parseCPUSection(cpuSec)
	ramMB := parseRAMSection(ramSec)
	hostname := parseHostSection(hostSec)

	return &SystemSpecs{
		OS:       osName,
		Distro:   distro,
		Kernel:   kernel,
		Arch:     arch,
		Cores:    cores,
		RAMMB:    ramMB,
		Hostname: hostname,
		PingMS:   pingMS,
	}
}

func extractSection(content, startMarker, endMarker string) string {
	startIdx := strings.Index(content, startMarker)
	if startIdx == -1 {
		return ""
	}
	startIdx += len(startMarker)
	rest := content[startIdx:]
	if endMarker != "" {
		endIdx := strings.Index(rest, endMarker)
		if endIdx != -1 {
			return strings.TrimSpace(rest[:endIdx])
		}
	}
	return strings.TrimSpace(rest)
}

func parseOSSection(content string) (osName string, distro string) {
	lines := strings.Split(content, "\n")
	var prettyName, name, versionID, version string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			prettyName = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"'`)
		} else if strings.HasPrefix(line, "ID=") {
			distro = strings.Trim(strings.TrimPrefix(line, "ID="), `"'`)
		} else if strings.HasPrefix(line, "NAME=") {
			name = strings.Trim(strings.TrimPrefix(line, "NAME="), `"'`)
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			versionID = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), `"'`)
		} else if strings.HasPrefix(line, "VERSION=") {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION="), `"'`)
		}
	}
	if prettyName != "" {
		osName = prettyName
	} else if name != "" && version != "" {
		osName = fmt.Sprintf("%s %s", name, version)
	} else if name != "" && versionID != "" {
		osName = fmt.Sprintf("%s %s", name, versionID)
	} else if name != "" {
		osName = name
	} else if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
		osName = strings.TrimSpace(lines[0])
	} else {
		osName = "Linux"
	}
	return osName, distro
}

func parseUnameSection(content string) (kernel string, arch string) {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			kernel = fields[1]
			arch = fields[len(fields)-1]
			return kernel, arch
		} else if len(fields) >= 1 {
			arch = fields[len(fields)-1]
			return "", arch
		}
	}
	return "", ""
}

func parseCPUSection(content string) int {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if n, err := strconv.Atoi(line); err == nil && n > 0 {
			return n
		}
	}
	return 1
}

func parseRAMSection(content string) int {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "mem:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if mb, err := strconv.Atoi(fields[1]); err == nil && mb > 0 {
					return mb
				}
			}
		}
		if strings.HasPrefix(lower, "memtotal:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if kb, err := strconv.Atoi(fields[1]); err == nil && kb > 0 {
					return kb / 1024
				}
			}
		}
	}
	return 0
}

func parseHostSection(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

// SSHManager defines the interface for managing pooled SSH connections and probing remote servers.
type SSHManager interface {
	GetClient(ctx context.Context, srv *db.Server, decryptedKey []byte) (*SSHClient, error)
	Probe(ctx context.Context, srv *db.Server, decryptedKey []byte) (*SystemSpecs, error)
	CloseServer(serverID string) error
	CloseAll() error
}

// ClientPool manages pooled SSH connections per server ID.
type ClientPool struct {
	mu      sync.RWMutex
	clients map[string]*SSHClient
	timeout time.Duration
}

// NewClientPool creates a new SSH connection pool with configurable timeout.
func NewClientPool(timeout ...time.Duration) *ClientPool {
	t := DefaultSSHTimeout
	if len(timeout) > 0 && timeout[0] > 0 {
		t = timeout[0]
	}
	return &ClientPool{
		clients: make(map[string]*SSHClient),
		timeout: t,
	}
}

// GetClient retrieves an active SSH client connection or dials a new one if not yet pooled or closed.
func (p *ClientPool) GetClient(ctx context.Context, srv *db.Server, decryptedKey []byte) (*SSHClient, error) {
	p.mu.RLock()
	if c, ok := p.clients[srv.ID]; ok && !c.IsClosed() {
		p.mu.RUnlock()
		return c, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check under write lock
	if c, ok := p.clients[srv.ID]; ok && !c.IsClosed() {
		return c, nil
	}

	cfg := SSHConfig{
		Host:              srv.Host,
		Port:              srv.Port,
		Username:          srv.Username,
		PrivateKey:        decryptedKey,
		Timeout:           p.timeout,
		KeepaliveInterval: DefaultKeepaliveInterval,
	}

	client, err := DialSSH(ctx, cfg)
	if err != nil {
		return nil, err
	}

	p.clients[srv.ID] = client
	return client, nil
}

// Probe dials or reuses the SSH connection for srv, runs health probe commands, and extracts system specifications.
func (p *ClientPool) Probe(ctx context.Context, srv *db.Server, decryptedKey []byte) (*SystemSpecs, error) {
	client, err := p.GetClient(ctx, srv, decryptedKey)
	if err != nil {
		return nil, err
	}
	return client.Probe(ctx)
}

// CloseServer terminates the active SSH client for the given server ID.
func (p *ClientPool) CloseServer(serverID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if client, ok := p.clients[serverID]; ok {
		delete(p.clients, serverID)
		return client.Close()
	}
	return nil
}

// CloseAll closes all cached SSH connections in the pool.
func (p *ClientPool) CloseAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var firstErr error
	for id, client := range p.clients {
		if err := client.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(p.clients, id)
	}
	return firstErr
}
