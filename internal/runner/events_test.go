package runner

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestEventBus_SubscribeAndPublishSync(t *testing.T) {
	bus := NewEventBus()
	received := make([]AttemptSucceededEvent, 0)
	var mu sync.Mutex

	bus.Subscribe(func(ctx context.Context, evt AttemptSucceededEvent) error {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, evt)
		return nil
	})

	evt := AttemptSucceededEvent{
		EventID:       "evt_1",
		EventType:     EventTaskAttemptSucceeded,
		TaskID:        "task_1",
		AttemptID:     "att_1",
		AttemptNumber: 1,
		Status:        "succeeded",
		Summary:       "Task completed",
		EmittedAt:     time.Now().UTC(),
	}

	err := bus.PublishSync(context.Background(), evt)
	if err != nil {
		t.Fatalf("unexpected publish error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 1 {
		t.Fatalf("expected 1 received event, got %d", len(received))
	}
	if received[0].TaskID != "task_1" || received[0].EventType != EventTaskAttemptSucceeded {
		t.Fatalf("mismatched received event: %+v", received[0])
	}
}

func TestEmitAttemptSucceeded_GlobalEventBus(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	var captured AttemptSucceededEvent
	GlobalEventBus.Subscribe(func(ctx context.Context, evt AttemptSucceededEvent) error {
		if evt.TaskID == "task_global_test" {
			captured = evt
			wg.Done()
		}
		return nil
	})

	EmitAttemptSucceeded(context.Background(), AttemptSucceededEvent{
		TaskID:        "task_global_test",
		AttemptID:     "att_global_1",
		AttemptNumber: 2,
		Status:        "succeeded",
	})

	waitCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		if captured.EventType != EventTaskAttemptSucceeded {
			t.Fatalf("expected event type %s, got %s", EventTaskAttemptSucceeded, captured.EventType)
		}
		if captured.AttemptNumber != 2 {
			t.Fatalf("expected attempt number 2, got %d", captured.AttemptNumber)
		}
		if captured.EventID == "" {
			t.Fatal("expected non-empty auto-generated EventID")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event on GlobalEventBus")
	}
}
