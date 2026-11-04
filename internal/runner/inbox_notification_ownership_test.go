package runner

import (
	"context"
	"testing"
	"time"

	"github.com/buchin/cuan/internal/task"
)

func TestInboxOwnsFailureReviewMessagesAfterRunnerCompletion(t *testing.T) {
	for _, status := range []string{task.AttemptStatusFailed, task.AttemptStatusInterrupted, task.AttemptStatusCanceled} {
		for _, source := range []string{"inbox-room", ""} {
			t.Run(status+"/"+source, func(t *testing.T) {
				ctx := context.Background()
				store := task.NewMemoryStore()
				svc := NewService(NewMemoryStore(store), store)
				item := &task.Task{ID: "notification-task", Title: "Approved work", Status: task.TaskStatusDoing, SourceRoomID: source, CreatedAt: time.Now()}
				if err := store.CreateTask(ctx, item); err != nil {
					t.Fatal(err)
				}
				attempt := &task.TaskAttempt{ID: "notification-attempt", TaskID: item.ID, RunnerID: "fixture", Status: task.AttemptStatusRunning, CreatedAt: time.Now()}
				if err := store.CreateAttempt(ctx, attempt); err != nil {
					t.Fatal(err)
				}
				messages := 0
				svc.SetChatNotifier(NewDefaultBusinessChatNotifier(func(context.Context, string, string) error { messages++; return nil }))
				bus := NewEventBus()
				svc.SetEventBus(bus)
				events := make(chan AttemptCompletedEvent, 1)
				bus.SubscribeCompleted(func(ctx context.Context, event AttemptCompletedEvent) error { events <- event; return nil })
				if _, err := svc.Complete(ctx, attempt.ID, CompleteAttemptRequest{RunnerID: "fixture", RoomID: "active-room", Status: status, ExitCode: 1, ErrorMessage: "Concrete execution failure"}); err != nil {
					t.Fatal(err)
				}
				select {
				case event := <-events:
					if event.TaskID != item.ID || event.AttemptID != attempt.ID {
						t.Fatal("wrong review event")
					}
					saved, _ := store.GetTask(ctx, item.ID)
					if saved.Status != task.TaskStatusBlocked {
						t.Fatal("review emitted before failure persisted", saved.Status)
					}
				case <-time.After(time.Second):
					t.Fatal("coordinator lost completion event")
				}
				if source != "" && messages != 0 {
					t.Fatal("runner announced an outcome before coordinator review")
				}
				if source == "" && messages != 1 {
					t.Fatal("unbound task lost its fallback notification")
				}
			})
		}
	}
}
