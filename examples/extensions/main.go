// Run with: go run ./examples/extensions
// No network, credentials or application database are used.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	sdk "github.com/buchin/cuan/pkg/extensions"
)

func main() {
	ctx := context.Background()
	registry := sdk.New()
	var mu sync.Mutex
	sent := map[string]bool{}
	plugin := sdk.Plugin{
		Manifest: sdk.Manifest{ID: "example.productivity", Name: "Productivity example", Version: "1.0.0", APIVersion: sdk.APIVersion,
			Capabilities: []string{"filter:contact.before_save", "validate:contact.before_save", "event:task.completed", "search:provide", "action:echo", "connector:example"}},
		Filters: map[string]sdk.Filter{"contact.before_save": func(ctx context.Context, scope sdk.Scope, input json.RawMessage) (json.RawMessage, error) {
			var contact struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			}
			if err := json.Unmarshal(input, &contact); err != nil {
				return nil, err
			}
			contact.Name = strings.TrimSpace(contact.Name)
			return json.Marshal(contact)
		}},
		Validators: map[string]sdk.Validator{"contact.before_save": func(ctx context.Context, scope sdk.Scope, input json.RawMessage) error {
			var contact struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(input, &contact); err != nil {
				return err
			}
			if contact.Name == "" {
				return fmt.Errorf("name required")
			}
			return nil
		}},
		Listeners: map[string]sdk.Listener{"task.completed": func(ctx context.Context, event sdk.Event) error {
			fmt.Println("Received task.completed for", event.Scope.ResourceID)
			return nil
		}},
		Search: func(ctx context.Context, request sdk.SearchRequest) ([]sdk.SearchItem, error) {
			if request.Offset > 0 || !strings.Contains("example project", strings.ToLower(request.Query)) {
				return []sdk.SearchItem{}, nil
			}
			return []sdk.SearchItem{{ID: "example", Title: "Example project", URL: "/projects/example/tasks"}}, nil
		},
		Actions: map[string]sdk.Action{"echo": func(ctx context.Context, scope sdk.Scope, input json.RawMessage) (json.RawMessage, error) {
			return input, nil
		}},
		Connectors: map[string]sdk.Connector{"example": func(ctx context.Context, scope sdk.Scope, message sdk.Message) error {
			mu.Lock()
			defer mu.Unlock()
			if sent[message.IdempotencyKey] {
				return nil
			}
			sent[message.IdempotencyKey] = true
			fmt.Println("Example connector accepted one approved message")
			return nil
		}},
	}
	must := func(err error) {
		if err != nil {
			panic(err)
		}
	}
	must(registry.Register(plugin))
	fmt.Println("Registered disabled extension:", registry.List()[0].Manifest.ID)
	must(registry.Enable(plugin.Manifest.ID))
	value, err := registry.Prepare(ctx, "contact.before_save", sdk.Scope{}, json.RawMessage(`{"name":"  Ada  ","email":""}`))
	must(err)
	fmt.Println("Filtered contact:", string(value))
	found := registry.Search(ctx, sdk.SearchRequest{Query: "example", Limit: 5})
	fmt.Println("Search results:", len(found[0].Items))
	output, err := registry.Action(ctx, plugin.Manifest.ID, "echo", sdk.Scope{}, json.RawMessage(`{"approved":true}`))
	must(err)
	fmt.Println("Action result:", string(output))
	event := sdk.Event{ID: "stable-event-1", Type: "task.completed", Version: 1, Scope: sdk.Scope{ResourceID: "example-task"}}
	if failures := registry.Publish(ctx, event); len(failures) > 0 {
		panic(failures[0])
	}
	message := sdk.Message{IdempotencyKey: "approved-operation-1", Recipient: "fixture", Text: "Example"}
	must(registry.Send(ctx, plugin.Manifest.ID, "example", sdk.Scope{}, message))
	must(registry.Send(ctx, plugin.Manifest.ID, "example", sdk.Scope{}, message))
	registry.Disable(plugin.Manifest.ID)
	registry.Unregister(plugin.Manifest.ID)
	fmt.Println("Disabled and unregistered:", len(registry.List()) == 0)
}
