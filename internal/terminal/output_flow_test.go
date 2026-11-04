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
