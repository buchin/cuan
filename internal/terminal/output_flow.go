package terminal

import (
	"errors"
	"sync"
	"time"
)

// One bounded PTY frame is in flight for negotiated viewers. Input and ping
// processing remain independent while the browser parses that frame.
type terminalOutputFlow struct {
	mu       sync.Mutex
	sequence uint64
	waiting  uint64
	ack      chan struct{}
}

func newTerminalOutputFlow() *terminalOutputFlow {
	return &terminalOutputFlow{ack: make(chan struct{}, 1)}
}

func (f *terminalOutputFlow) acknowledge(sequence uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if sequence == 0 || sequence != f.waiting {
		return
	}
	f.waiting = 0
	f.ack <- struct{}{}
}

func (f *terminalOutputFlow) write(done <-chan struct{}, timeout time.Duration, send func() error) error {
	f.mu.Lock()
	f.sequence++
	f.waiting = f.sequence
	f.mu.Unlock()
	if err := send(); err != nil {
		return err
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-f.ack:
		return nil
	case <-done:
		return errors.New("terminal output connection closed")
	case <-timer.C:
		return errors.New("terminal output acknowledgement timed out")
	}
}
