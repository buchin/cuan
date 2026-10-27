package deployer

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/db"
	"golang.org/x/crypto/ssh"
)

// mockSSHServer implements an in-process RFC 4253 SSH server for testing.
type mockSSHServer struct {
	listener       net.Listener
	addr           string
	port           int
	hostSigner     ssh.Signer
	authorizedKeys map[string]bool
	rejectAuth     bool
	hangHandshake  bool
	probeOutput    string
	stopChan       chan struct{}
	mu             sync.Mutex
	closed         bool
	conns          []net.Conn
}

func newMockSSHServer(t *testing.T, probeOutput string) (*mockSSHServer, []byte, string) {
	t.Helper()

	// 1. Generate host key
	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatalf("signer from host key: %v", err)
	}

	// 2. Generate valid client key
	clientPub, clientPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate client key: %v", err)
	}
	clientPrivBytes, err := x509.MarshalPKCS8PrivateKey(clientPriv)
	if err != nil {
		t.Fatalf("marshal client key: %v", err)
	}
	clientPrivPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: clientPrivBytes,
	})

	sshClientPub, err := ssh.NewPublicKey(clientPub)
	if err != nil {
		t.Fatalf("ssh public key: %v", err)
	}
	clientFingerprint := ssh.FingerprintSHA256(sshClientPub)

	// 3. Listen on local random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen mock ssh: %v", err)
	}
	tcpAddr := listener.Addr().(*net.TCPAddr)

	defaultOutput := `===CUAN_OS===
PRETTY_NAME="Ubuntu 22.04.4 LTS"
NAME="Ubuntu"
VERSION_ID="22.04"
ID=ubuntu
===CUAN_UNAME===
Linux 5.15.0-101-generic x86_64
===CUAN_CPU===
4
===CUAN_RAM===
Mem:          8192        1024        6144         100        1024        6900
===CUAN_HOST===
test-mock-server
===CUAN_END===`

	if probeOutput != "" {
		defaultOutput = probeOutput
	}

	ms := &mockSSHServer{
		listener:       listener,
		addr:           "127.0.0.1",
		port:           tcpAddr.Port,
		hostSigner:     hostSigner,
		authorizedKeys: map[string]bool{clientFingerprint: true},
		probeOutput:    defaultOutput,
		stopChan:       make(chan struct{}),
	}

	go ms.acceptLoop()

	return ms, clientPrivPEM, clientFingerprint
}

func (s *mockSSHServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}

		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			_ = conn.Close()
			return
		}
		s.conns = append(s.conns, conn)
		hang := s.hangHandshake
		s.mu.Unlock()

		if hang {
			// Don't send SSH identification banner; let client time out
			continue
		}

		go s.handleConnection(conn)
	}
}

func (s *mockSSHServer) handleConnection(netConn net.Conn) {
	defer netConn.Close()

	sshConfig := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			s.mu.Lock()
			reject := s.rejectAuth
			authorized := s.authorizedKeys[ssh.FingerprintSHA256(key)]
			s.mu.Unlock()

			if reject || !authorized {
				return nil, errors.New("permission denied")
			}
			return nil, nil
		},
	}
	sshConfig.AddHostKey(s.hostSigner)

	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, sshConfig)
	if err != nil {
		return
	}
	defer sshConn.Close()

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, err := newChannel.Accept()
		if err != nil {
			return
		}

		go func(ch ssh.Channel, reqs <-chan *ssh.Request) {
			defer ch.Close()
			for req := range reqs {
				switch req.Type {
				case "exec":
					s.mu.Lock()
					out := s.probeOutput
					s.mu.Unlock()

					_, _ = ch.Write([]byte(out))
					_ = req.Reply(true, nil)
					status := struct{ Status uint32 }{0}
					_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(&status))
					return
				default:
					_ = req.Reply(false, nil)
				}
			}
		}(channel, requests)
	}
}

func (s *mockSSHServer) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.stopChan)
	_ = s.listener.Close()
	for _, c := range s.conns {
		_ = c.Close()
	}
}

func TestSSHConnector_Success(t *testing.T) {
	server, privKey, _ := newMockSSHServer(t, "")
	defer server.Close()

	cfg := SSHConfig{
		Host:              server.addr,
		Port:              server.port,
		Username:          "ubuntu",
		PrivateKey:        privKey,
		Timeout:           2 * time.Second,
		KeepaliveInterval: 1 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	client, err := DialSSH(ctx, cfg)
	if err != nil {
		t.Fatalf("DialSSH failed: %v", err)
	}
	defer client.Close()

	if client.IsClosed() {
		t.Errorf("expected client to be open")
	}

	// 1. Test ping
	pingDuration, err := client.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
	if pingDuration <= 0 {
		t.Errorf("expected positive ping duration, got %v", pingDuration)
	}

	// 2. Test probe & fingerprinting
	specs, err := client.Probe(ctx)
	if err != nil {
		t.Fatalf("Probe failed: %v", err)
	}

	if specs.OS != "Ubuntu 22.04.4 LTS" {
		t.Errorf("expected OS 'Ubuntu 22.04.4 LTS', got %q", specs.OS)
	}
	if specs.Distro != "ubuntu" {
		t.Errorf("expected Distro 'ubuntu', got %q", specs.Distro)
	}
	if specs.Cores != 4 {
		t.Errorf("expected 4 CPU cores, got %d", specs.Cores)
	}
	if specs.RAMMB != 8192 {
		t.Errorf("expected 8192 RAM MB, got %d", specs.RAMMB)
	}
	if specs.Arch != "x86_64" {
		t.Errorf("expected x86_64 arch, got %q", specs.Arch)
	}
	if specs.Hostname != "test-mock-server" {
		t.Errorf("expected hostname 'test-mock-server', got %q", specs.Hostname)
	}
	if specs.PingMS <= 0 {
		t.Errorf("expected positive PingMS, got %d", specs.PingMS)
	}

	toMap := specs.ToMap()
	if toMap["os"] != "Ubuntu 22.04.4 LTS" || toMap["cores"] != 4 || toMap["ram_mb"] != 8192 {
		t.Errorf("specs.ToMap() mismatch: %+v", toMap)
	}
}

func TestSSHConnector_AuthFailure(t *testing.T) {
	server, _, _ := newMockSSHServer(t, "")
	defer server.Close()

	// 1. Other valid private key not in server's authorizedKeys
	_, otherPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate other key: %v", err)
	}
	otherPrivBytes, _ := x509.MarshalPKCS8PrivateKey(otherPriv)
	otherPrivPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: otherPrivBytes,
	})

	cfg := SSHConfig{
		Host:       server.addr,
		Port:       server.port,
		Username:   "ubuntu",
		PrivateKey: otherPrivPEM,
		Timeout:    1 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := DialSSH(ctx, cfg)
	if err == nil {
		if client != nil {
			_ = client.Close()
		}
		t.Fatalf("expected auth error, got nil")
	}
	if !errors.Is(err, ErrSSHAuthFailed) {
		t.Errorf("expected ErrSSHAuthFailed, got: %v", err)
	}

	// 2. Corrupted private key
	cfg.PrivateKey = []byte("corrupted-not-a-pem-key")
	_, err = DialSSH(ctx, cfg)
	if err == nil {
		t.Fatalf("expected error on corrupt key, got nil")
	}
	if !errors.Is(err, ErrSSHAuthFailed) {
		t.Errorf("expected ErrSSHAuthFailed for corrupt key, got: %v", err)
	}
}

func TestSSHConnector_HostUnreachableAndTimeout(t *testing.T) {
	// 1. Host unreachable: connect to closed local port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close() // Immediately close so port is unreachable

	// Valid key
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	privBytes, _ := x509.MarshalPKCS8PrivateKey(priv)
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	cfg := SSHConfig{
		Host:       "127.0.0.1",
		Port:       port,
		Username:   "ubuntu",
		PrivateKey: privPEM,
		Timeout:    500 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err = DialSSH(ctx, cfg)
	if err == nil {
		t.Fatalf("expected unreachable error, got nil")
	}
	if !errors.Is(err, ErrSSHHostUnreachable) {
		t.Errorf("expected ErrSSHHostUnreachable, got: %v", err)
	}

	// 2. Hanging connection (times out within 5s or context timeout)
	server, privKey, _ := newMockSSHServer(t, "")
	defer server.Close()
	server.mu.Lock()
	server.hangHandshake = true
	server.mu.Unlock()

	cfgHanging := SSHConfig{
		Host:       server.addr,
		Port:       server.port,
		Username:   "ubuntu",
		PrivateKey: privKey,
		Timeout:    100 * time.Millisecond,
	}

	start := time.Now()
	ctxShort, cancelShort := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancelShort()

	_, err = DialSSH(ctxShort, cfgHanging)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("expected timeout error on hanging server, got nil")
	}
	if !errors.Is(err, ErrSSHConnectionTimeout) && !errors.Is(err, ErrSSHHostUnreachable) {
		t.Errorf("expected timeout/unreachable error, got: %v", err)
	}
	if elapsed > 2*time.Second {
		t.Errorf("timeout took too long: %v", elapsed)
	}
}

func TestSSHClientPool_Lifecycle(t *testing.T) {
	server, privKey, _ := newMockSSHServer(t, "")
	defer server.Close()

	pool := NewClientPool(2 * time.Second)
	defer pool.CloseAll()

	srvRecord := &db.Server{
		ID:       "srv_test_pool_1",
		Name:     "Pool Test Server",
		Host:     server.addr,
		Port:     server.port,
		Username: "ubuntu",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 1. First probe creates and caches connection
	specs1, err := pool.Probe(ctx, srvRecord, privKey)
	if err != nil {
		t.Fatalf("pool.Probe 1 failed: %v", err)
	}
	if specs1.OS != "Ubuntu 22.04.4 LTS" {
		t.Errorf("unexpected OS: %s", specs1.OS)
	}

	// 2. Second get retrieves cached connection
	client, err := pool.GetClient(ctx, srvRecord, privKey)
	if err != nil {
		t.Fatalf("pool.GetClient failed: %v", err)
	}
	if client.IsClosed() {
		t.Errorf("expected cached client to be active")
	}

	// 3. CloseServer terminates and evicts
	if err := pool.CloseServer(srvRecord.ID); err != nil {
		t.Fatalf("CloseServer failed: %v", err)
	}
	if !client.IsClosed() {
		t.Errorf("expected evicted client to be closed")
	}

	// 4. Probe after close establishes fresh connection
	specs2, err := pool.Probe(ctx, srvRecord, privKey)
	if err != nil {
		t.Fatalf("pool.Probe 2 failed: %v", err)
	}
	if specs2.Cores != 4 {
		t.Errorf("unexpected cores: %d", specs2.Cores)
	}
}

func TestParseProbeOutput_Variations(t *testing.T) {
	// 1. Alpine Linux with MemTotal kb
	alpineRaw := `===CUAN_OS===
NAME="Alpine Linux"
ID=alpine
VERSION_ID=3.19.1
PRETTY_NAME="Alpine Linux v3.19"
===CUAN_UNAME===
Linux 6.6.14-0-virt x86_64
===CUAN_CPU===
2
===CUAN_RAM===
MemTotal:        4048296 kB
MemFree:         3120144 kB
===CUAN_HOST===
alpine-node
===CUAN_END===`

	specs := ParseProbeOutput(alpineRaw, 15)
	if specs.OS != "Alpine Linux v3.19" {
		t.Errorf("expected Alpine Linux v3.19, got %q", specs.OS)
	}
	if specs.Distro != "alpine" {
		t.Errorf("expected alpine distro, got %q", specs.Distro)
	}
	if specs.Cores != 2 {
		t.Errorf("expected 2 cores, got %d", specs.Cores)
	}
	if specs.RAMMB != 3953 { // 4048296 / 1024
		t.Errorf("expected ~3953 RAM MB, got %d", specs.RAMMB)
	}
	if specs.Arch != "x86_64" {
		t.Errorf("expected x86_64, got %q", specs.Arch)
	}
	if specs.Hostname != "alpine-node" {
		t.Errorf("expected alpine-node hostname, got %q", specs.Hostname)
	}
	if specs.PingMS != 15 {
		t.Errorf("expected 15 ping_ms, got %d", specs.PingMS)
	}

	// 2. Debian without PRETTY_NAME quotes
	debianRaw := `===CUAN_OS===
NAME=Debian GNU/Linux
VERSION_ID=12
VERSION=12 (bookworm)
ID=debian
===CUAN_UNAME===
Linux 6.1.0-18-amd64 x86_64
===CUAN_CPU===
8
===CUAN_RAM===
              total        used        free      shared  buff/cache   available
Mem:          16000        1500       12000         200        2500       14300
===CUAN_HOST===
deb-srv
===CUAN_END===`

	specsDebian := ParseProbeOutput(debianRaw, 5)
	if !strings.Contains(specsDebian.OS, "Debian") {
		t.Errorf("expected Debian OS, got %q", specsDebian.OS)
	}
	if specsDebian.Distro != "debian" {
		t.Errorf("expected debian distro, got %q", specsDebian.Distro)
	}
	if specsDebian.Cores != 8 {
		t.Errorf("expected 8 cores, got %d", specsDebian.Cores)
	}
	if specsDebian.RAMMB != 16000 {
		t.Errorf("expected 16000 RAM MB, got %d", specsDebian.RAMMB)
	}

	// 3. Raw output fallback without delimiters
	fallbackRaw := `Linux generic-host 5.4.0 x86_64`
	specsFallback := ParseProbeOutput(fallbackRaw, 1)
	if specsFallback.OS == "" {
		t.Errorf("expected non-empty fallback OS")
	}
	if specsFallback.Cores < 1 {
		t.Errorf("expected at least 1 core default")
	}
}
