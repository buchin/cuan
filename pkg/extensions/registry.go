// Package extensions is Cuan's v1 SDK for trusted, build-time Go extensions.
// Capability declarations are host contracts, not a sandbox for native Go code.
package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const APIVersion = 1

var ErrUnavailable = errors.New("extension unavailable")
var ErrCallback = errors.New("extension callback failed")
var ErrBusy = errors.New("extension callback capacity exhausted")
var identifier = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)+$`)
var version = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[a-zA-Z0-9.-]+)?$`)
var pointName = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,100}$`)

type Manifest struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	APIVersion   int      `json:"api_version"`
	Capabilities []string `json:"capabilities"`
	Projects     []string `json:"projects,omitempty"`
}
type Scope struct {
	OwnerID    string `json:"owner_id,omitempty"`
	ProjectID  string `json:"project_id,omitempty"`
	ResourceID string `json:"resource_id,omitempty"`
}
type Event struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Version int             `json:"version"`
	Scope   Scope           `json:"scope"`
	At      time.Time       `json:"at"`
	Data    json.RawMessage `json:"data,omitempty"`
}
type Filter func(context.Context, Scope, json.RawMessage) (json.RawMessage, error)
type Validator func(context.Context, Scope, json.RawMessage) error
type Listener func(context.Context, Event) error
type SearchItem struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
	URL     string `json:"url"`
}
type SearchRequest struct {
	Query  string
	Offset int
	Limit  int
	Scope  Scope
}
type SearchProvider func(context.Context, SearchRequest) ([]SearchItem, error)
type Message struct {
	IdempotencyKey string `json:"idempotency_key"`
	Recipient      string `json:"recipient"`
	Text           string `json:"text"`
}
type Connector func(context.Context, Scope, Message) error
type Action func(context.Context, Scope, json.RawMessage) (json.RawMessage, error)
type Plugin struct {
	Manifest   Manifest
	Filters    map[string]Filter
	Validators map[string]Validator
	Listeners  map[string]Listener
	Search     SearchProvider
	Connectors map[string]Connector
	Actions    map[string]Action
}
type Status struct {
	Manifest Manifest `json:"manifest"`
	Enabled  bool     `json:"enabled"`
}
type SearchResult struct {
	ID    string
	Label string
	Items []SearchItem
	Error error
}
type EventSink interface { // Optional durable delivery adapter; append must be idempotent by Event.ID.
	Append(context.Context, Event) error
}
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
	enabled map[string]bool
	slots   chan struct{}
	timeout time.Duration
	sink    EventSink
}

func New() *Registry {
	return &Registry{plugins: map[string]Plugin{}, enabled: map[string]bool{}, slots: make(chan struct{}, 8), timeout: 500 * time.Millisecond}
}
func copyManifest(m Manifest) Manifest {
	m.Capabilities = append([]string{}, m.Capabilities...)
	m.Projects = append([]string{}, m.Projects...)
	return m
}
func (m Manifest) has(cap string) bool {
	for _, c := range m.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}
func (m Manifest) allows(scope Scope) bool {
	if len(m.Projects) == 0 {
		return true
	}
	for _, project := range m.Projects {
		if project == scope.ProjectID {
			return true
		}
	}
	return false
}
func copyMap[T any](source map[string]T) map[string]T {
	out := map[string]T{}
	for k, v := range source {
		out[k] = v
	}
	return out
}
func (r *Registry) Register(plugin Plugin) error {
	m := copyManifest(plugin.Manifest)
	if !identifier.MatchString(m.ID) || m.Name == "" || len(m.Name) > 80 || !version.MatchString(m.Version) || m.APIVersion != APIVersion {
		return errors.New("invalid extension manifest or unsupported API version")
	}
	seen := map[string]bool{}
	for _, cap := range m.Capabilities {
		parts := strings.SplitN(cap, ":", 2)
		if len(parts) != 2 || !pointName.MatchString(parts[1]) || seen[cap] {
			return errors.New("invalid or duplicate capability")
		}
		switch parts[0] {
		case "filter", "validate", "event", "connector", "action", "markdown", "parser", "view":
		case "search":
			if parts[1] != "provide" {
				return errors.New("unsupported search capability")
			}
		default:
			return errors.New("unsupported extension capability")
		}
		seen[cap] = true
	}
	checks := []struct {
		prefix string
		keys   []string
	}{}
	keys := func(prefix string, names []string) {
		checks = append(checks, struct {
			prefix string
			keys   []string
		}{prefix, names})
	}
	for name, fn := range plugin.Filters {
		if fn == nil {
			return errors.New("nil filter")
		}
		keys("filter", []string{name})
	}
	for name, fn := range plugin.Validators {
		if fn == nil {
			return errors.New("nil validator")
		}
		keys("validate", []string{name})
	}
	for name, fn := range plugin.Listeners {
		if fn == nil {
			return errors.New("nil listener")
		}
		keys("event", []string{name})
	}
	for name, fn := range plugin.Connectors {
		if fn == nil {
			return errors.New("nil connector")
		}
		keys("connector", []string{name})
	}
	for name, fn := range plugin.Actions {
		if fn == nil {
			return errors.New("nil action")
		}
		keys("action", []string{name})
	}
	for _, check := range checks {
		for _, name := range check.keys {
			if !pointName.MatchString(name) || !m.has(check.prefix+":"+name) {
				return fmt.Errorf("undeclared capability: %s:%s", check.prefix, name)
			}
		}
	}
	if plugin.Search != nil && !m.has("search:provide") {
		return errors.New("undeclared search provider")
	}
	plugin.Manifest = m
	plugin.Filters = copyMap(plugin.Filters)
	plugin.Validators = copyMap(plugin.Validators)
	plugin.Listeners = copyMap(plugin.Listeners)
	plugin.Connectors = copyMap(plugin.Connectors)
	plugin.Actions = copyMap(plugin.Actions)
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.plugins) >= 32 {
		return errors.New("extension registration limit reached")
	}
	if _, ok := r.plugins[m.ID]; ok {
		return errors.New("extension ID already registered")
	}
	r.plugins[m.ID] = plugin
	r.enabled[m.ID] = false
	return nil
}

// Enable/Disable are bootstrap/host lifecycle operations. This registry stores no
// installation state on disk; the host must reapply its configuration on restart.
func (r *Registry) Enable(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.plugins[id]; !ok {
		return ErrUnavailable
	}
	r.enabled[id] = true
	return nil
}
func (r *Registry) Disable(id string) { r.mu.Lock(); defer r.mu.Unlock(); r.enabled[id] = false }
func (r *Registry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.enabled, id)
	delete(r.plugins, id)
}
func (r *Registry) List() []Status {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []Status{}
	for id, p := range r.plugins {
		out = append(out, Status{Manifest: copyManifest(p.Manifest), Enabled: r.enabled[id]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Manifest.ID < out[j].Manifest.ID })
	return out
}
func (r *Registry) active(scope Scope) []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := []string{}
	for id, p := range r.plugins {
		if r.enabled[id] && p.Manifest.allows(scope) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	out := []Plugin{}
	for _, id := range ids {
		out = append(out, r.plugins[id])
	}
	return out
}
func invoke[T any](r *Registry, ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	select {
	case r.slots <- struct{}{}:
	default:
		return zero, ErrBusy
	}
	bounded, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	type result struct {
		value T
		err   error
	}
	done := make(chan result, 1)
	go func() {
		defer func() { <-r.slots }()
		defer func() {
			if recover() != nil {
				done <- result{err: ErrCallback}
			}
		}()
		v, e := fn(bounded)
		if e != nil {
			e = ErrCallback
		}
		done <- result{v, e}
	}()
	select {
	case value := <-done:
		return value.value, value.err
	case <-bounded.Done():
		return zero, bounded.Err()
	}
}
func clone(data json.RawMessage) json.RawMessage { return append(json.RawMessage{}, data...) }
func validPayload(data json.RawMessage) bool     { return len(data) <= 65536 && json.Valid(data) }
func (r *Registry) Prepare(ctx context.Context, point string, scope Scope, data json.RawMessage) (json.RawMessage, error) {
	if !validPayload(data) {
		return nil, errors.New("invalid extension payload")
	}
	value := clone(data)
	plugins := r.active(scope)
	for _, p := range plugins {
		if fn := p.Filters[point]; fn != nil {
			input := clone(value)
			next, err := invoke(r, ctx, func(ctx context.Context) (json.RawMessage, error) { return fn(ctx, scope, input) })
			if err != nil {
				return nil, err
			}
			if !validPayload(next) {
				return nil, ErrCallback
			}
			value = clone(next)
		}
	}
	for _, p := range plugins {
		if fn := p.Validators[point]; fn != nil {
			input := clone(value)
			_, err := invoke(r, ctx, func(ctx context.Context) (bool, error) { return true, fn(ctx, scope, input) })
			if err != nil {
				return nil, err
			}
		}
	}
	return value, nil
}
func (r *Registry) SetEventSink(sink EventSink) { r.mu.Lock(); defer r.mu.Unlock(); r.sink = sink }

// Publish returns listener failures to the host; callers must not roll back an
// already committed operation. It never retries or reruns the original task.
// Without an EventSink delivery is synchronous best-effort, not a durable bus.
func (r *Registry) Publish(ctx context.Context, event Event) []error {
	if event.ID == "" || !pointName.MatchString(event.Type) || event.Version != 1 || (len(event.Data) > 0 && !validPayload(event.Data)) {
		return []error{errors.New("invalid event")}
	}
	event.Data = clone(event.Data)
	r.mu.RLock()
	sink := r.sink
	r.mu.RUnlock()
	failures := []error{}
	if sink != nil {
		_, err := invoke(r, ctx, func(ctx context.Context) (bool, error) {
			copy := event
			copy.Data = clone(event.Data)
			return true, sink.Append(ctx, copy)
		})
		if err != nil {
			failures = append(failures, err)
		}
	}
	for _, p := range r.active(event.Scope) {
		if fn := p.Listeners[event.Type]; fn != nil {
			copy := event
			copy.Data = clone(event.Data)
			_, err := invoke(r, ctx, func(ctx context.Context) (bool, error) { return true, fn(ctx, copy) })
			if err != nil {
				failures = append(failures, err)
			}
		}
	}
	return failures
}
func (r *Registry) Search(ctx context.Context, request SearchRequest) []SearchResult {
	if request.Limit < 1 || request.Limit > 50 || request.Offset < 0 || len(request.Query) > 800 {
		return nil
	}
	out := []SearchResult{}
	for _, p := range r.active(request.Scope) {
		if p.Search == nil {
			continue
		}
		items, err := invoke(r, ctx, func(ctx context.Context) ([]SearchItem, error) { return p.Search(ctx, request) })
		if len(items) > request.Limit {
			items = items[:request.Limit]
		}
		safe := []SearchItem{}
		seen := map[string]bool{}
		if err == nil {
			for _, item := range items {
				if item.ID == "" || seen[item.ID] || len(item.Title) > 500 || len(item.Snippet) > 2000 || !internalURL(item.URL) {
					continue
				}
				seen[item.ID] = true
				safe = append(safe, item)
			}
		}
		out = append(out, SearchResult{ID: p.Manifest.ID, Label: p.Manifest.Name, Items: safe, Error: err})
	}
	return out
}
func internalURL(value string) bool {
	return len(value) <= 2000 && !strings.ContainsAny(value, "\\\r\n") && (strings.HasPrefix(value, "/contacts?") || strings.HasPrefix(value, "/inbox/") || strings.HasPrefix(value, "/projects/") || strings.HasPrefix(value, "/tasks/"))
}
func (r *Registry) Action(ctx context.Context, id, name string, scope Scope, input json.RawMessage) (json.RawMessage, error) {
	if !validPayload(input) {
		return nil, ErrCallback
	}
	for _, p := range r.active(scope) {
		if p.Manifest.ID == id {
			if fn := p.Actions[name]; fn != nil {
				value, err := invoke(r, ctx, func(ctx context.Context) (json.RawMessage, error) { return fn(ctx, scope, clone(input)) })
				if err != nil {
					return nil, err
				}
				if !validPayload(value) {
					return nil, ErrCallback
				}
				return clone(value), nil
			}
		}
	}
	return nil, ErrUnavailable
}

// Send must be called only after the host authorizes an outbound action. A key
// must be forwarded to the provider for idempotency; the SDK never auto-retries.
func (r *Registry) Send(ctx context.Context, id, channel string, scope Scope, message Message) error {
	if message.IdempotencyKey == "" || len(message.IdempotencyKey) > 200 || message.Recipient == "" || len(message.Text) > 24000 {
		return errors.New("invalid connector message")
	}
	for _, p := range r.active(scope) {
		if p.Manifest.ID == id {
			if fn := p.Connectors[channel]; fn != nil {
				_, err := invoke(r, ctx, func(ctx context.Context) (bool, error) { return true, fn(ctx, scope, message) })
				return err
			}
		}
	}
	return ErrUnavailable
}
