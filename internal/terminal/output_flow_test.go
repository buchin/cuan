package terminal

import (
	"testing"
	"time"
)

func TestOutputFlowWaitsForExactCredit(t *testing.T) {
	f := newTerminalOutputFlow()
	done := make(chan struct{})
	sent := make(chan struct{})
	result := make(chan error, 1)
	go func() { result <- f.write(done, time.Second, func() error { close(sent); return nil }) }()
	<-sent
	f.acknowledge(0)
	f.acknowledge(2)
	select {
	case <-result:
		t.Fatal("unparsed frame released by invalid credit")
	default:
	}
	f.acknowledge(1)
	f.acknowledge(1)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	err := f.write(done, time.Second, func() error { f.acknowledge(1); f.acknowledge(2); return nil })
	if err != nil {
		t.Fatal(err)
	}
}

func TestOutputFlowClosesOrTimesOutWithoutCredit(t *testing.T) {
	f := newTerminalOutputFlow()
	done := make(chan struct{})
	close(done)
	if err := f.write(done, time.Second, func() error { return nil }); err == nil {
		t.Fatal("closed connection waited for credit")
	}
	if err := newTerminalOutputFlow().write(make(chan struct{}), time.Millisecond, func() error { return nil }); err == nil {
		t.Fatal("missing credit never expired")
	}
}

func TestOutputFlow_UnblockAndTimeoutRecoveryDoesNotAccumulateStaleTokens(t *testing.T) {
	f := newTerminalOutputFlow()
	done := make(chan struct{})

	// 1. Unblock should not poison the next write with an immediate unearned ACK
	f.unblock()

	sent := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- f.write(done, 50*time.Millisecond, func() error { close(sent); return nil })
	}()
	<-sent

	// Ensure the write is actually waiting and was NOT instantly released by unblock's token
	select {
	case <-result:
		t.Fatal("write completed immediately using leftover unblock token")
	case <-time.After(10 * time.Millisecond):
	}

	// Supply valid ACK
	f.acknowledge(1)
	if err := <-result; err != nil {
		t.Fatalf("expected clean ack, got %v", err)
	}

	// 2. Timeout clears waiting and subsequent write functions normally
	err := f.write(done, 10*time.Millisecond, func() error { return nil })
	if err == nil {
		t.Fatal("expected timeout error")
	}

	// Stale ACK for old sequence is dropped safely
	f.acknowledge(2)

	// Next write works properly with fresh ACK
	go func() {
		time.Sleep(10 * time.Millisecond)
		f.acknowledge(3)
	}()
	if err := f.write(done, 100*time.Millisecond, func() error { return nil }); err != nil {
		t.Fatalf("write after timeout failed: %v", err)
	}
}
