package deployer

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/buchin/cuan/internal/crypto"
	"github.com/buchin/cuan/internal/db"
	"golang.org/x/crypto/ssh"
)

// RemoteLogStreamer manages real-time streaming of stdout/stderr logs from a remote server
// over an SSH channel into an in-memory ring buffer and persistent database store.
type RemoteLogStreamer struct {
	deploymentID string
	serverID     string
	serverRepo   db.ServerRepository
	keyVault     *crypto.KeyVault
	sshManager   SSHManager
	store        Store
	streamer     *LogStreamer

	initialBackoff time.Duration
	maxBackoff     time.Duration

	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{}
	mu      sync.RWMutex
	running bool
}

// RemoteStreamerOption configures optional parameters for RemoteLogStreamer.
type RemoteStreamerOption func(*RemoteLogStreamer)

// WithStreamerBufferSize configures the ring buffer size.
func WithStreamerBufferSize(size int) RemoteStreamerOption {
	return func(s *RemoteLogStreamer) {
		if size > 0 {
			s.streamer = NewLogStreamer(s.deploymentID, s.store, size)
		}
	}
}

// WithStreamerBackoff configures reconnect backoff timing.
func WithStreamerBackoff(initial, max time.Duration) RemoteStreamerOption {
	return func(s *RemoteLogStreamer) {
		if initial > 0 {
			s.initialBackoff = initial
		}
		if max > 0 {
			s.maxBackoff = max
		}
	}
}

// NewRemoteLogStreamer creates a new RemoteLogStreamer instance.
func NewRemoteLogStreamer(
	deploymentID string,
	serverID string,
	serverRepo db.ServerRepository,
	keyVault *crypto.KeyVault,
	sshManager SSHManager,
	store Store,
	opts ...RemoteStreamerOption,
) *RemoteLogStreamer {
	s := &RemoteLogStreamer{
		deploymentID:   deploymentID,
		serverID:       serverID,
		serverRepo:     serverRepo,
		keyVault:       keyVault,
		sshManager:     sshManager,
		store:          store,
		streamer:       NewLogStreamer(deploymentID, store, 1000),
		initialBackoff: 200 * time.Millisecond,
		maxBackoff:     3 * time.Second,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Streamer returns the underlying LogStreamer (ring buffer, subscribers, and store).
func (s *RemoteLogStreamer) Streamer() *LogStreamer {
	return s.streamer
}

// DeploymentID returns the targeted deployment ID.
func (s *RemoteLogStreamer) DeploymentID() string {
	return s.deploymentID
}

// ServerID returns the target remote server ID.
func (s *RemoteLogStreamer) ServerID() string {
	return s.serverID
}

// IsRunning reports whether the streaming loop is currently active.
func (s *RemoteLogStreamer) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Start begins asynchronous streaming of the remote log file over an SSH channel.
func (s *RemoteLogStreamer) Start(parentCtx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}

	if parentCtx == nil {
		parentCtx = context.Background()
	}

	s.ctx, s.cancel = context.WithCancel(parentCtx)
	s.done = make(chan struct{})
	s.running = true
	s.mu.Unlock()

	go s.streamLoop()
	return nil
}

// Stop cleanly cancels the streaming context, closes the SSH session, and shuts down the streamer.
func (s *RemoteLogStreamer) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	if s.cancel != nil {
		s.cancel()
	}
	doneCh := s.done
	s.mu.Unlock()

	// Wait with timeout for loop to terminate
	if doneCh != nil {
		select {
		case <-doneCh:
		case <-time.After(2 * time.Second):
		}
	}

	if s.streamer != nil {
		s.streamer.Close()
	}
}

// streamLoop runs the tailing and auto-reconnect cycle.
func (s *RemoteLogStreamer) streamLoop() {
	defer close(s.done)

	firstRun := true
	backoff := s.initialBackoff

	for {
		s.mu.RLock()
		ctx := s.ctx
		s.mu.RUnlock()

		if ctx == nil || ctx.Err() != nil {
			return
		}

		// 1. Fetch remote server configuration
		srv, err := s.serverRepo.GetByID(ctx, s.serverID)
		if err != nil {
			if s.waitBackoff(ctx, &backoff) {
				return
			}
			continue
		}

		// 2. Decrypt SSH credentials
		decryptedKey, err := s.keyVault.Decrypt(srv.EncryptedKey)
		if err != nil {
			if s.waitBackoff(ctx, &backoff) {
				return
			}
			continue
		}

		// 3. Obtain or dial SSH client from pool
		client, err := s.sshManager.GetClient(ctx, srv, decryptedKey)
		if err != nil {
			if s.waitBackoff(ctx, &backoff) {
				return
			}
			continue
		}

		sshClient := client.Client()
		if sshClient == nil || client.IsClosed() {
			if s.waitBackoff(ctx, &backoff) {
				return
			}
			continue
		}

		// 4. Create dedicated SSH session for streaming
		session, err := sshClient.NewSession()
		if err != nil {
			// Evict connection if session creation failed
			_ = s.sshManager.CloseServer(s.serverID)
			if s.waitBackoff(ctx, &backoff) {
				return
			}
			continue
		}

		stdoutPipe, err := session.StdoutPipe()
		if err != nil {
			_ = session.Close()
			if s.waitBackoff(ctx, &backoff) {
				return
			}
			continue
		}

		stderrPipe, err := session.StderrPipe()
		if err != nil {
			_ = session.Close()
			if s.waitBackoff(ctx, &backoff) {
				return
			}
			continue
		}

		// On first run, tail all lines from beginning; on reconnect, tail new lines only
		remoteLogPath := fmt.Sprintf("~/cuan-apps/%s/app.log", s.deploymentID)
		tailCmd := fmt.Sprintf("touch %s && tail -n +1 -f %s", remoteLogPath, remoteLogPath)
		if !firstRun {
			tailCmd = fmt.Sprintf("touch %s && tail -n 0 -f %s", remoteLogPath, remoteLogPath)
		}

		if err := session.Start(tailCmd); err != nil {
			_ = session.Close()
			if s.waitBackoff(ctx, &backoff) {
				return
			}
			continue
		}

		// Successfully connected and command spawned
		firstRun = false
		backoff = s.initialBackoff

		// 5. Read stream lines concurrently
		combinedReader := io.MultiReader(stdoutPipe, stderrPipe)
		scanner := bufio.NewScanner(combinedReader)
		scanner.Buffer(make([]byte, 1024), 64*1024)

		scannerDone := make(chan struct{})
		go func() {
			defer close(scannerDone)
			for scanner.Scan() {
				line := scanner.Text()
				if line == "" {
					continue
				}

				entry := LogEntry{
					DeploymentID: s.deploymentID,
					Stream:       StreamStdout,
					Message:      line,
					Timestamp:    time.Now().UTC(),
				}
				s.streamer.Append(ctx, entry)
			}
		}()

		// 6. Wait for context cancellation or stream termination
		select {
		case <-ctx.Done():
			_ = session.Signal(ssh.SIGTERM)
			_ = session.Close()
			<-scannerDone
			return

		case <-scannerDone:
			_ = session.Close()
			// Stream ended (e.g. network disconnect or remote process termination).
			// Attempt auto-reconnect after backoff delay.
			if s.waitBackoff(ctx, &backoff) {
				return
			}
		}
	}
}

func (s *RemoteLogStreamer) waitBackoff(ctx context.Context, backoff *time.Duration) bool {
	select {
	case <-ctx.Done():
		return true
	case <-time.After(*backoff):
		*backoff *= 2
		if *backoff > s.maxBackoff {
			*backoff = s.maxBackoff
		}
		return false
	}
}

// FetchSnapshot performs a one-shot read of the remote log file using an SSH command,
// injecting missing lines into the in-memory ring buffer and persistent store.
func (s *RemoteLogStreamer) FetchSnapshot(ctx context.Context, lines int) ([]LogEntry, error) {
	if lines <= 0 {
		lines = 100
	}

	srv, err := s.serverRepo.GetByID(ctx, s.serverID)
	if err != nil {
		return nil, fmt.Errorf("lookup server %s: %w", s.serverID, err)
	}

	decryptedKey, err := s.keyVault.Decrypt(srv.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt server ssh key: %w", err)
	}

	client, err := s.sshManager.GetClient(ctx, srv, decryptedKey)
	if err != nil {
		return nil, fmt.Errorf("connect to remote host: %w", err)
	}

	remoteLogPath := fmt.Sprintf("~/cuan-apps/%s/app.log", s.deploymentID)
	cmd := fmt.Sprintf("if [ -f %s ]; then tail -n %d %s; fi", remoteLogPath, lines, remoteLogPath)

	stdout, stderr, err := client.RunCommand(ctx, cmd)
	if err != nil {
		return nil, fmt.Errorf("read remote log file: %w (stderr: %s)", err, stderr)
	}

	var entries []LogEntry
	rawLines := strings.Split(strings.TrimSpace(stdout), "\n")
	for _, l := range rawLines {
		line := strings.TrimSpace(l)
		if line == "" {
			continue
		}
		entry := LogEntry{
			DeploymentID: s.deploymentID,
			Stream:       StreamStdout,
			Message:      line,
			Timestamp:    time.Now().UTC(),
		}
		s.streamer.Append(ctx, entry)
		entries = append(entries, entry)
	}

	return entries, nil
}
