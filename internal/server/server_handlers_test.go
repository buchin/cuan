package server

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/config"
	cuanDB "github.com/buchin/cuan/internal/db"
	"github.com/buchin/cuan/internal/deployer"
	"golang.org/x/crypto/ssh"
)

// inProcessMockSSHServer runs an RFC 4253 SSH server for testing server HTTP endpoints.
type inProcessMockSSHServer struct {
	listener       net.Listener
	addr           string
	port           int
	hostSigner     ssh.Signer
	authorizedKeys map[string]bool
	rejectAuth     bool
	probeOutput    string
	mu             sync.Mutex
	closed         bool
}

func startInProcessMockSSH(t *testing.T, output string) (*inProcessMockSSHServer, []byte, string) {
	t.Helper()

	_, hostPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostPriv)
	if err != nil {
		t.Fatalf("signer from host key: %v", err)
	}

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
8
===CUAN_RAM===
Mem:          16384        2048       12288         100        2048       14000
===CUAN_HOST===
test-byos-vps
===CUAN_END===`

	if output != "" {
		defaultOutput = output
	}

	ms := &inProcessMockSSHServer{
		listener:       listener,
		addr:           "127.0.0.1",
		port:           tcpAddr.Port,
		hostSigner:     hostSigner,
		authorizedKeys: map[string]bool{clientFingerprint: true},
		probeOutput:    defaultOutput,
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go ms.handleConn(conn)
		}
	}()

	return ms, clientPrivPEM, clientFingerprint
}

func (s *inProcessMockSSHServer) handleConn(netConn net.Conn) {
	defer netConn.Close()

	sshConfig := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			s.mu.Lock()
			reject := s.rejectAuth
			authorized := s.authorizedKeys[ssh.FingerprintSHA256(key)]
			s.mu.Unlock()

			if reject || !authorized {
				return nil, errors.New("unauthorized key")
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
			_ = newChannel.Reject(ssh.UnknownChannelType, "unsupported channel")
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

func (s *inProcessMockSSHServer) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	_ = s.listener.Close()
}

func setupTestServerForBYOS(t *testing.T) *Server {
	t.Helper()
	cfg := &config.Config{
		ServerPort: "8080",
		AuthSecret: "test-auth-secret-byos",
		AppMode:    "studio",
	}
	// Pass nil db to initialize with MemoryServerRepository
	srv := NewServer(cfg, nil)
	return srv
}

func TestServerHandlers_ListServers_EmptyAndPopulated(t *testing.T) {
	s := setupTestServerForBYOS(t)

	// 1. Initial empty list
	req := httptest.NewRequest(http.MethodGet, "/api/servers", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d", rec.Code)
	}

	var emptyResp struct {
		Servers []cuanDB.ServerDTO `json:"servers"`
		Count   int                `json:"count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&emptyResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if emptyResp.Count != 0 || len(emptyResp.Servers) != 0 {
		t.Errorf("expected 0 servers, got %d", emptyResp.Count)
	}

	// 2. Populate two servers in memory repo
	now := time.Now().UTC()
	_ = s.ServerRepository().Create(context.Background(), &cuanDB.Server{
		ID:             "srv_1",
		Name:           "Server Alpha",
		Host:           "192.168.1.10",
		Port:           22,
		Username:       "ubuntu",
		AuthType:       "key",
		EncryptedKey:   "SECRET_ENCRYPTED_KEY_AT_REST",
		KeyFingerprint: "SHA256:alpha-fingerprint",
		Status:         "pending",
		CreatedAt:      now,
		UpdatedAt:      now,
	})

	reqPop := httptest.NewRequest(http.MethodGet, "/api/servers", nil)
	recPop := httptest.NewRecorder()
	s.Router().ServeHTTP(recPop, reqPop)

	if recPop.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d", recPop.Code)
	}

	bodyStr := recPop.Body.String()
	if strings.Contains(bodyStr, "SECRET_ENCRYPTED_KEY_AT_REST") {
		t.Fatalf("SECURITY VIOLATION: EncryptedKey leaked in /api/servers response: %s", bodyStr)
	}
	if strings.Contains(bodyStr, "encrypted_key") {
		t.Fatalf("SECURITY VIOLATION: encrypted_key field leaked in /api/servers response: %s", bodyStr)
	}

	var popResp struct {
		Servers []cuanDB.ServerDTO `json:"servers"`
		Count   int                `json:"count"`
	}
	if err := json.Unmarshal([]byte(bodyStr), &popResp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if popResp.Count != 1 || len(popResp.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", popResp.Count)
	}
	if popResp.Servers[0].Name != "Server Alpha" || popResp.Servers[0].Host != "192.168.1.10" {
		t.Errorf("server data mismatch: %+v", popResp.Servers[0])
	}
}

func TestServerHandlers_CreateServer_ValidationAndEncryption(t *testing.T) {
	s := setupTestServerForBYOS(t)

	// Generate valid test key
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	privBytes, _ := x509.MarshalPKCS8PrivateKey(priv)
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}))
	sshPub, _ := ssh.NewPublicKey(pub)
	expectedFingerprint := ssh.FingerprintSHA256(sshPub)

	// 1. Validation errors
	tests := []struct {
		name       string
		payload    map[string]interface{}
		expectCode int
		errSubstr  string
	}{
		{
			name:       "Missing name",
			payload:    map[string]interface{}{"host": "10.0.0.1", "private_key": privPEM},
			expectCode: http.StatusBadRequest,
			errSubstr:  "name is required",
		},
		{
			name:       "Missing host",
			payload:    map[string]interface{}{"name": "My VPS", "private_key": privPEM},
			expectCode: http.StatusBadRequest,
			errSubstr:  "host is required",
		},
		{
			name:       "Missing private_key",
			payload:    map[string]interface{}{"name": "My VPS", "host": "10.0.0.1"},
			expectCode: http.StatusBadRequest,
			errSubstr:  "private_key is required",
		},
		{
			name:       "Invalid private_key format",
			payload:    map[string]interface{}{"name": "My VPS", "host": "10.0.0.1", "private_key": "not-a-valid-key"},
			expectCode: http.StatusBadRequest,
			errSubstr:  "invalid private key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest(http.MethodPost, "/api/servers", bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			s.Router().ServeHTTP(rec, req)

			if rec.Code != tc.expectCode {
				t.Errorf("expected %d, got %d: %s", tc.expectCode, rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), tc.errSubstr) {
				t.Errorf("expected error message to contain %q, got: %s", tc.errSubstr, rec.Body.String())
			}
		})
	}

	// 2. Successful server registration
	validPayload := map[string]interface{}{
		"name":        "Production Worker 1",
		"host":        "198.51.100.25",
		"port":        2222,
		"username":    "deployer",
		"auth_type":   "key",
		"private_key": privPEM,
	}
	b, _ := json.Marshal(validPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/servers", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got: %d: %s", rec.Code, rec.Body.String())
	}

	bodyStr := rec.Body.String()
	if strings.Contains(bodyStr, privPEM) {
		t.Fatalf("CRITICAL SECURITY LEAK: Plaintext private key returned in API response!")
	}
	if strings.Contains(bodyStr, "encrypted_key") {
		t.Fatalf("CRITICAL SECURITY LEAK: encrypted_key returned in API response!")
	}

	var createResp struct {
		Server cuanDB.ServerDTO `json:"server"`
	}
	if err := json.Unmarshal([]byte(bodyStr), &createResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	created := createResp.Server
	if created.ID == "" || !strings.HasPrefix(created.ID, "srv_") {
		t.Errorf("expected server ID with prefix srv_, got %q", created.ID)
	}
	if created.Name != "Production Worker 1" || created.Host != "198.51.100.25" {
		t.Errorf("server metadata mismatch: %+v", created)
	}
	if created.Port != 2222 || created.Username != "deployer" {
		t.Errorf("port/username mismatch: port=%d, user=%s", created.Port, created.Username)
	}
	if created.KeyFingerprint != expectedFingerprint {
		t.Errorf("fingerprint mismatch: expected %s, got %s", expectedFingerprint, created.KeyFingerprint)
	}
	if created.Status != "pending" {
		t.Errorf("expected initial status pending, got %q", created.Status)
	}

	// 3. Verify database storage has EncryptedKey at-rest
	stored, err := s.ServerRepository().GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("failed to load stored server from repository: %v", err)
	}
	if stored.EncryptedKey == "" {
		t.Fatalf("expected encrypted_key to be populated in database record")
	}
	if stored.EncryptedKey == privPEM {
		t.Fatalf("CRITICAL SECURITY LEAK: private key was stored in plaintext!")
	}

	// 4. Decrypt via KeyVault and verify original key matches
	decrypted, err := s.KeyVault().Decrypt(stored.EncryptedKey)
	if err != nil {
		t.Fatalf("failed to decrypt stored key with KeyVault: %v", err)
	}
	if strings.TrimSpace(string(decrypted)) != strings.TrimSpace(privPEM) {
		t.Fatalf("decrypted key does not match original plaintext PEM")
	}
}

func TestServerHandlers_GetServer_And_DeleteServer(t *testing.T) {
	s := setupTestServerForBYOS(t)

	now := time.Now().UTC()
	srv := &cuanDB.Server{
		ID:             "srv_test_lookup",
		Name:           "Node Lookup",
		Host:           "10.0.0.5",
		Port:           22,
		Username:       "ubuntu",
		AuthType:       "key",
		EncryptedKey:   "ENCRYPTED_VALUE",
		KeyFingerprint: "SHA256:lookup-fp",
		Status:         "pending",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	_ = s.ServerRepository().Create(context.Background(), srv)

	// 1. GET existing server
	req := httptest.NewRequest(http.MethodGet, "/api/servers/srv_test_lookup", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got: %d", rec.Code)
	}
	var getResp struct {
		Server cuanDB.ServerDTO `json:"server"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&getResp); err != nil {
		t.Fatalf("decode getResp error: %v", err)
	}
	if getResp.Server.ID != "srv_test_lookup" || getResp.Server.Name != "Node Lookup" {
		t.Errorf("getResp data mismatch: %+v", getResp.Server)
	}

	// 2. GET non-existent server -> 404
	req404 := httptest.NewRequest(http.MethodGet, "/api/servers/srv_non_existent", nil)
	rec404 := httptest.NewRecorder()
	s.Router().ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing server, got: %d", rec404.Code)
	}

	// 3. DELETE existing server -> 200 OK
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/servers/srv_test_lookup", nil)
	recDel := httptest.NewRecorder()
	s.Router().ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusOK {
		t.Errorf("expected 200 OK for delete, got: %d", recDel.Code)
	}

	// 4. GET deleted server -> 404
	reqCheck := httptest.NewRequest(http.MethodGet, "/api/servers/srv_test_lookup", nil)
	recCheck := httptest.NewRecorder()
	s.Router().ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusNotFound {
		t.Errorf("expected 404 after deletion, got: %d", recCheck.Code)
	}

	// 5. DELETE non-existent server -> 404
	reqDel404 := httptest.NewRequest(http.MethodDelete, "/api/servers/srv_test_lookup", nil)
	recDel404 := httptest.NewRecorder()
	s.Router().ServeHTTP(recDel404, reqDel404)
	if recDel404.Code != http.StatusNotFound {
		t.Errorf("expected 404 for delete of missing server, got: %d", recDel404.Code)
	}
}

func TestServerHandlers_TestServer_ProbeSuccessAndDBPersistence(t *testing.T) {
	mockSSH, clientPrivPEM, _ := startInProcessMockSSH(t, "")
	defer mockSSH.Close()

	s := setupTestServerForBYOS(t)

	// Encrypt client key using server's KeyVault
	encKey, err := s.KeyVault().Encrypt(clientPrivPEM)
	if err != nil {
		t.Fatalf("encrypt test key: %v", err)
	}

	srv := &cuanDB.Server{
		ID:             "srv_test_live_probe",
		Name:           "Live Probe VPS",
		Host:           mockSSH.addr,
		Port:           mockSSH.port,
		Username:       "ubuntu",
		AuthType:       "key",
		EncryptedKey:   encKey,
		KeyFingerprint: "SHA256:live-probe-fp",
		Status:         "pending",
		OSInfo:         make(map[string]interface{}),
	}
	if err := s.ServerRepository().Create(context.Background(), srv); err != nil {
		t.Fatalf("create server record: %v", err)
	}

	// Execute instant test probe: POST /api/servers/{id}/test
	req := httptest.NewRequest(http.MethodPost, "/api/servers/srv_test_live_probe/test", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for test probe, got: %d: %s", rec.Code, rec.Body.String())
	}

	var testResp struct {
		Success bool                 `json:"success"`
		Status  string               `json:"status"`
		Specs   deployer.SystemSpecs `json:"specs"`
		Server  cuanDB.ServerDTO     `json:"server"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&testResp); err != nil {
		t.Fatalf("failed to decode test probe response: %v", err)
	}

	if !testResp.Success {
		t.Fatalf("expected test probe success=true, got false")
	}
	if testResp.Status != "online" {
		t.Errorf("expected status 'online', got %q", testResp.Status)
	}
	if testResp.Specs.OS != "Ubuntu 22.04.4 LTS" {
		t.Errorf("expected OS 'Ubuntu 22.04.4 LTS', got %q", testResp.Specs.OS)
	}
	if testResp.Specs.Cores != 8 {
		t.Errorf("expected 8 cores, got %d", testResp.Specs.Cores)
	}
	if testResp.Specs.RAMMB != 16384 {
		t.Errorf("expected 16384 RAM MB, got %d", testResp.Specs.RAMMB)
	}
	if testResp.Specs.PingMS <= 0 {
		t.Errorf("expected positive PingMS, got %d", testResp.Specs.PingMS)
	}

	// Verify persistence in database repository
	updated, err := s.ServerRepository().GetByID(context.Background(), "srv_test_live_probe")
	if err != nil {
		t.Fatalf("failed to query updated server from repo: %v", err)
	}
	if updated.Status != "online" {
		t.Errorf("expected DB status 'online', got %q", updated.Status)
	}
	if updated.LastPingAt == nil {
		t.Errorf("expected DB last_ping_at to be populated")
	}
	if updated.OSInfo["os"] != "Ubuntu 22.04.4 LTS" {
		t.Errorf("expected DB OSInfo to contain OS, got %+v", updated.OSInfo)
	}
	if fmt.Sprintf("%v", updated.OSInfo["cores"]) != "8" {
		t.Errorf("expected DB OSInfo cores=8, got %v", updated.OSInfo["cores"])
	}
}

func TestServerHandlers_TestServer_AuthFailureAndUnreachable(t *testing.T) {
	// 1. Auth failure scenario
	mockSSH, clientPrivPEM, _ := startInProcessMockSSH(t, "")
	defer mockSSH.Close()
	mockSSH.mu.Lock()
	mockSSH.rejectAuth = true // Force authentication rejection
	mockSSH.mu.Unlock()

	s := setupTestServerForBYOS(t)

	encKey, _ := s.KeyVault().Encrypt(clientPrivPEM)
	srvAuthFail := &cuanDB.Server{
		ID:           "srv_auth_fail",
		Name:         "Auth Fail VPS",
		Host:         mockSSH.addr,
		Port:         mockSSH.port,
		Username:     "ubuntu",
		AuthType:     "key",
		EncryptedKey: encKey,
		Status:       "pending",
	}
	_ = s.ServerRepository().Create(context.Background(), srvAuthFail)

	req := httptest.NewRequest(http.MethodPost, "/api/servers/srv_auth_fail/test", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with test result, got: %d", rec.Code)
	}

	var failResp struct {
		Success bool             `json:"success"`
		Status  string           `json:"status"`
		Error   string           `json:"error"`
		Server  cuanDB.ServerDTO `json:"server"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&failResp); err != nil {
		t.Fatalf("decode failResp: %v", err)
	}

	if failResp.Success {
		t.Errorf("expected success=false for auth failure")
	}
	if failResp.Status != "error" {
		t.Errorf("expected status 'error', got %q", failResp.Status)
	}
	if !strings.Contains(strings.ToLower(failResp.Error), "auth") && !strings.Contains(strings.ToLower(failResp.Error), "permission") {
		t.Errorf("expected auth error message, got: %s", failResp.Error)
	}

	// Check DB status updated to error
	inDB, _ := s.ServerRepository().GetByID(context.Background(), "srv_auth_fail")
	if inDB.Status != "error" {
		t.Errorf("expected DB status to be 'error', got %q", inDB.Status)
	}

	// 2. Host unreachable scenario (closed port)
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	closedPort := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	srvUnreachable := &cuanDB.Server{
		ID:           "srv_unreachable",
		Name:         "Unreachable VPS",
		Host:         "127.0.0.1",
		Port:         closedPort,
		Username:     "ubuntu",
		AuthType:     "key",
		EncryptedKey: encKey,
		Status:       "pending",
	}
	_ = s.ServerRepository().Create(context.Background(), srvUnreachable)

	reqUnreach := httptest.NewRequest(http.MethodPost, "/api/servers/srv_unreachable/test", nil)
	recUnreach := httptest.NewRecorder()
	s.Router().ServeHTTP(recUnreach, reqUnreach)

	if recUnreach.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with test result, got: %d", recUnreach.Code)
	}
	var unreachResp struct {
		Success bool   `json:"success"`
		Status  string `json:"status"`
		Error   string `json:"error"`
	}
	_ = json.NewDecoder(recUnreach.Body).Decode(&unreachResp)
	if unreachResp.Success {
		t.Errorf("expected success=false for unreachable server")
	}
	if unreachResp.Status != "offline" {
		t.Errorf("expected status 'offline', got %q", unreachResp.Status)
	}

	inDBUnreach, _ := s.ServerRepository().GetByID(context.Background(), "srv_unreachable")
	if inDBUnreach.Status != "offline" {
		t.Errorf("expected DB status 'offline', got %q", inDBUnreach.Status)
	}
}

func TestServerHandlers_APIV1_Compatibility(t *testing.T) {
	s := setupTestServerForBYOS(t)

	// Test GET /api/v1/servers
	req := httptest.NewRequest(http.MethodGet, "/api/v1/servers", nil)
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /api/v1/servers, got: %d", rec.Code)
	}

	var resp struct {
		Servers []cuanDB.ServerDTO `json:"servers"`
		Count   int                `json:"count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode /api/v1/servers: %v", err)
	}
	if resp.Count != 0 {
		t.Errorf("expected 0 servers, got %d", resp.Count)
	}
}
