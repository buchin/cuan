package task

import (
	"context"
	sdk "github.com/buchin/cuan/pkg/extensions"
	"testing"
	"time"
)

func TestTaskExtensionsAfterPersistenceAndFailureIsolation(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store, nil)
	registry := sdk.New()
	events := make(chan sdk.Event, 8)
	listener := func(ctx context.Context, event sdk.Event) error {
		persisted, err := store.GetTask(ctx, event.Scope.ResourceID)
		if err != nil {
			t.Error("event before persistence", err)
		}
		if event.Type == "task.blocked" && persisted.Status != "blocked" {
			t.Error("uncommitted task status")
		}
		events <- event
		return sdk.ErrCallback
	}
	registry.Register(sdk.Plugin{Manifest: sdk.Manifest{ID: "example.tasks", Name: "Task events", Version: "1.0.0", APIVersion: 1, Capabilities: []string{"event:task.created", "event:task.blocked"}}, Listeners: map[string]sdk.Listener{"task.created": listener, "task.blocked": listener}})
	registry.Enable("example.tasks")
	service.SetExtensionRegistry(registry)
	work, err := service.CreateTask(context.Background(), "owner", CreateTaskRequest{Title: "Fixture task", Instructions: "No execution", ProjectID: ""})
	if err != nil {
		t.Fatal(err)
	}
	status := "blocked"
	work, err = service.UpdateTask(context.Background(), work.ID, "owner", UpdateTaskRequest{Status: &status})
	if err != nil || work.Status != "blocked" {
		t.Fatal("listener failure rolled back work", err)
	}
	for _, kind := range []string{"task.created", "task.blocked"} {
		select {
		case event := <-events:
			if event.Type != kind || event.ID == "" || len(event.Data) != 0 {
				t.Fatal("event envelope", event)
			}
		case <-time.After(time.Second):
			t.Fatal("missing event", kind)
		}
	}
}

func TestTaskExtensionsReleaseMutationLocksBeforeCallbacks(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store, nil)
	registry := sdk.New()
	completed := make(chan error, 1)
	err := registry.Register(sdk.Plugin{Manifest: sdk.Manifest{ID: "example.followup", Name: "Followup", Version: "1.0.0", APIVersion: 1, Capabilities: []string{"event:task.created"}}, Listeners: map[string]sdk.Listener{
		"task.created": func(ctx context.Context, event sdk.Event) error {
			title := "Updated after persistence"
			_, err := service.UpdateTask(ctx, event.Scope.ResourceID, "extension", UpdateTaskRequest{Title: &title})
			completed <- err
			return err
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	registry.Enable("example.followup")
	service.SetExtensionRegistry(registry)
	created, err := service.CreateTask(context.Background(), "owner", CreateTaskRequest{Title: "Initial", Instructions: "Fixture", ProjectID: ""})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-completed:
		if err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("listener blocked on a mutation lock")
	}
	saved, err := store.GetTask(context.Background(), created.ID)
	if err != nil || saved.Title != "Updated after persistence" {
		t.Fatal("followup lost", err)
	}
}
