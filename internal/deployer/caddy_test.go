package deployer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestCaddyDomainFormatting verifies subdomain formatting and fallback domain behaviors.
func TestCaddyDomainFormatting(t *testing.T) {
	tests := []struct {
		name       string
		baseDomain string
		slug       string
		want       string
	}{
		{
			name:       "Standard production subdomain",
			baseDomain: "cuan.dojo.cc",
			slug:       "my-cool-app",
			want:       "my-cool-app.cuan.dojo.cc",
		},
		{
			name:       "Localhost development subdomain",
			baseDomain: "cuan.localhost",
			slug:       "test-service",
			want:       "test-service.cuan.localhost",
		},
		{
			name:       "Uppercase and trimmed slug handling",
			baseDomain: "CUAN.DOJO.CC",
			slug:       "  StoreFront-V2  ",
			want:       "storefront-v2.cuan.dojo.cc",
		},
		{
			name:       "Empty slug defaults to app",
			baseDomain: "cuan.dojo.cc",
			slug:       "",
			want:       "app.cuan.dojo.cc",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultCaddyConfig()
			cfg.BaseDomain = tc.baseDomain
			router := NewCaddyRouter(cfg)

			got := router.FormatDomain(tc.slug)
			if got != tc.want {
				t.Errorf("FormatDomain(%q) = %q, want %q", tc.slug, got, tc.want)
			}
		})
	}
}

// TestCaddyMockAdminAPI verifies route registration, error route registration,
// route status, deletion, and idempotency using a mock Caddy Admin HTTP server.
func TestCaddyMockAdminAPI(t *testing.T) {
	var mu sync.Mutex
	routesStore := make(map[string]map[string]interface{})
	errorsStore := make(map[string]map[string]interface{})

	// Mock Caddy Admin API server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		path := r.URL.Path

		// Liveness probe
		if path == "/config/" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"admin": map[string]interface{}{}})
			return
		}

		// Server info
		if path == "/config/apps/http/servers/srv0" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"listen": []string{":443"}})
			return
		}

		// Check errors block
		if path == "/config/apps/http/servers/srv0/errors" {
			if r.Method == http.MethodGet {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"routes": []interface{}{}})
				return
			}
			if r.Method == http.MethodPut {
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		// Post route to /config/apps/http/servers/srv0/routes
		if path == "/config/apps/http/servers/srv0/routes" && r.Method == http.MethodPost {
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			id, _ := body["@id"].(string)
			routesStore[id] = body
			w.WriteHeader(http.StatusOK)
			return
		}

		// Post error route to /config/apps/http/servers/srv0/errors/routes
		if path == "/config/apps/http/servers/srv0/errors/routes" && r.Method == http.MethodPost {
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			id, _ := body["@id"].(string)
			errorsStore[id] = body
			w.WriteHeader(http.StatusOK)
			return
		}

		// Query by ID: /id/{id}
		if strings.HasPrefix(path, "/id/") {
			id := strings.TrimPrefix(path, "/id/")
			if r.Method == http.MethodGet {
				if item, exists := routesStore[id]; exists {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(item)
					return
				}
				if item, exists := errorsStore[id]; exists {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(item)
					return
				}
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if r.Method == http.MethodDelete {
				delete(routesStore, id)
				delete(errorsStore, id)
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	ctx := context.Background()
	cfg := CaddyConfig{
		AdminURL:     mockServer.URL,
		BaseDomain:   "cuan.dojo.cc",
		ServerName:   "srv0",
		HTTPClient:   mockServer.Client(),
		FallbackHTML: "<html><body>App Starting</body></html>",
	}
	router := NewCaddyRouter(cfg)

	// 1. Verify router can reach Caddy Admin API
	if !router.IsAdminReachable(ctx) {
		t.Fatalf("expected admin to be reachable at %s", mockServer.URL)
	}

	// 2. Register dynamic route
	slug := "demo-store"
	port := 9002
	err := router.RegisterRoute(ctx, slug, port)
	if err != nil {
		t.Fatalf("RegisterRoute failed: %v", err)
	}

	// 3. Verify route registered in mock Caddy
	mu.Lock()
	routeID := fmt.Sprintf("cuan-route-%s", slug)
	routeObj, exists := routesStore[routeID]
	mu.Unlock()

	if !exists {
		t.Fatalf("expected route ID %s to exist in Caddy", routeID)
	}

	// Verify route details
	matches, _ := routeObj["match"].([]interface{})
	if len(matches) == 0 {
		t.Fatalf("expected route to have match rules")
	}
	firstMatch := matches[0].(map[string]interface{})
	hosts, _ := firstMatch["host"].([]interface{})
	if len(hosts) == 0 || hosts[0] != "demo-store.cuan.dojo.cc" {
		t.Fatalf("expected host demo-store.cuan.dojo.cc, got %v", hosts)
	}

	// Verify reverse proxy upstream & headers
	handles, _ := routeObj["handle"].([]interface{})
	if len(handles) == 0 {
		t.Fatalf("expected route to have handle actions")
	}
	firstHandle := handles[0].(map[string]interface{})
	if firstHandle["handler"] != "reverse_proxy" {
		t.Fatalf("expected handler reverse_proxy, got %v", firstHandle["handler"])
	}
	upstreams, _ := firstHandle["upstreams"].([]interface{})
	if len(upstreams) == 0 {
		t.Fatalf("expected upstream to be set")
	}
	upstream0 := upstreams[0].(map[string]interface{})
	if upstream0["dial"] != "127.0.0.1:9002" {
		t.Fatalf("expected upstream dial 127.0.0.1:9002, got %v", upstream0["dial"])
	}

	// Verify error route (graceful fallback)
	mu.Lock()
	errID := fmt.Sprintf("cuan-error-%s", slug)
	_, errExists := errorsStore[errID]
	mu.Unlock()
	if !errExists {
		t.Fatalf("expected error route %s to exist in Caddy", errID)
	}

	// 4. Verify route status via GetRouteStatus
	status, err := router.GetRouteStatus(ctx, slug)
	if err != nil {
		t.Fatalf("GetRouteStatus error: %v", err)
	}
	if !status {
		t.Fatalf("expected route status to be active (true)")
	}

	// 5. Test idempotency: re-registering should update cleanly without conflict
	err = router.RegisterRoute(ctx, slug, 9005)
	if err != nil {
		t.Fatalf("idempotent RegisterRoute failed: %v", err)
	}
	mu.Lock()
	updRoute := routesStore[routeID]
	mu.Unlock()
	updHandles := updRoute["handle"].([]interface{})[0].(map[string]interface{})
	updDial := updHandles["upstreams"].([]interface{})[0].(map[string]interface{})["dial"]
	if updDial != "127.0.0.1:9005" {
		t.Fatalf("expected updated upstream dial 127.0.0.1:9005, got %v", updDial)
	}

	// 6. Test UnregisterRoute cleanly removes the route
	err = router.UnregisterRoute(ctx, slug)
	if err != nil {
		t.Fatalf("UnregisterRoute failed: %v", err)
	}

	statusAfterDelete, err := router.GetRouteStatus(ctx, slug)
	if err != nil {
		t.Fatalf("GetRouteStatus after delete error: %v", err)
	}
	if statusAfterDelete {
		t.Fatalf("expected route status to be false after unregister")
	}

	mu.Lock()
	_, stillExists := routesStore[routeID]
	_, stillErrExists := errorsStore[errID]
	mu.Unlock()
	if stillExists || stillErrExists {
		t.Fatalf("expected routes to be deleted from Caddy, stillExists=%v stillErrExists=%v", stillExists, stillErrExists)
	}
}

// TestCaddySnippetGenerationAndLifecycle verifies that Caddyfile snippets can be generated,
// written to disk, and cleaned up correctly.
func TestCaddySnippetGenerationAndLifecycle(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := CaddyConfig{
		AdminURL:     "http://localhost:2019",
		BaseDomain:   "cuan.dojo.cc",
		SnippetsDir:  tmpDir,
		FallbackHTML: "<html><body>App Booting</body></html>",
	}
	router := NewCaddyRouter(cfg)

	slug := "blog-v1"
	port := 9040

	// 1. Generate snippet
	snippet := router.GenerateSnippet(slug, port)
	if !strings.Contains(snippet, "blog-v1.cuan.dojo.cc {") {
		t.Errorf("expected snippet to contain domain header, got:\n%s", snippet)
	}
	if !strings.Contains(snippet, "reverse_proxy 127.0.0.1:9040") {
		t.Errorf("expected snippet to contain reverse proxy directive, got:\n%s", snippet)
	}
	if !strings.Contains(snippet, "handle_errors {") {
		t.Errorf("expected snippet to contain handle_errors block, got:\n%s", snippet)
	}

	// 2. Write snippet file
	err := router.WriteSnippet(slug, port)
	if err != nil {
		t.Fatalf("WriteSnippet failed: %v", err)
	}

	snippetFile := filepath.Join(tmpDir, "blog-v1.caddy")
	content, err := os.ReadFile(snippetFile)
	if err != nil {
		t.Fatalf("failed to read written snippet: %v", err)
	}
	if !strings.Contains(string(content), "127.0.0.1:9040") {
		t.Errorf("snippet content does not match expected output")
	}

	// 3. Remove snippet file
	err = router.RemoveSnippet(slug)
	if err != nil {
		t.Fatalf("RemoveSnippet failed: %v", err)
	}
	if _, err := os.Stat(snippetFile); !os.IsNotExist(err) {
		t.Errorf("expected snippet file to be deleted")
	}
}

// TestCaddyReconcileRoutes verifies that running deployments can be reconciled
// into Caddy routes upon system restart.
func TestCaddyReconcileRoutes(t *testing.T) {
	ctx := context.Background()
	cfg := DefaultCaddyConfig()
	router := NewCaddyRouter(cfg)

	deps := []*Deployment{
		{
			ID:           "dep_1",
			ProjectName:  "App One",
			Slug:         "app-one",
			Status:       StatusRunning,
			InternalPort: 9010,
		},
		{
			ID:           "dep_2",
			ProjectName:  "App Two",
			Slug:         "app-two",
			Status:       StatusStopped, // should not be registered
			InternalPort: 9011,
		},
		{
			ID:           "dep_3",
			ProjectName:  "App Three",
			Slug:         "app-three",
			Status:       StatusRunning,
			InternalPort: 9012,
		},
	}

	err := router.Reconcile(ctx, deps)
	if err != nil {
		t.Fatalf("Reconcile error: %v", err)
	}

	// App one and three should have active routes in cache
	st1, _ := router.GetRouteStatus(ctx, "app-one")
	st2, _ := router.GetRouteStatus(ctx, "app-two")
	st3, _ := router.GetRouteStatus(ctx, "app-three")

	if !st1 {
		t.Errorf("expected app-one route to be active")
	}
	if st2 {
		t.Errorf("expected stopped app-two route to NOT be active")
	}
	if !st3 {
		t.Errorf("expected app-three route to be active")
	}
}

// TestLocalDriverWithCaddyRouter verifies that LocalDriver interacts seamlessly
// with the Router during Deploy, Start, Stop, and Delete operations.
func TestLocalDriverWithCaddyRouter(t *testing.T) {
	tmpDir := t.TempDir()
	storage, err := NewStorageEngine(filepath.Join(tmpDir, "deployments"))
	if err != nil {
		t.Fatalf("NewStorageEngine error: %v", err)
	}
	store := NewMemoryStore()
	portPool := NewPortPool()

	cfg := CaddyConfig{
		BaseDomain: "cuan.dojo.cc",
	}
	router := NewCaddyRouter(cfg)
	driver := NewLocalDriverWithRouter(storage, store, portPool, router)

	ctx := context.Background()

	// 1. Deploy project: verify domain is automatically allocated
	zipData, zipSize := buildZip(t, map[string]string{
		"index.html": "<h1>Deploy Test</h1>",
	})
	res, err := driver.Deploy(ctx, DeploymentRequest{
		ProjectName: "Virtual Host App",
		Slug:        "vh-app",
		ZipReader:   bytes.NewReader(zipData),
		ZipSize:     zipSize,
	})
	if err != nil {
		t.Fatalf("Deploy error: %v", err)
	}

	if res.Deployment.Domain != "vh-app.cuan.dojo.cc" {
		t.Errorf("expected domain vh-app.cuan.dojo.cc, got %q", res.Deployment.Domain)
	}

	// 2. Start deployment: verify dynamic route is registered
	err = driver.Start(ctx, res.Deployment.ID)
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	routeActive, err := router.GetRouteStatus(ctx, "vh-app")
	if err != nil {
		t.Fatalf("GetRouteStatus error: %v", err)
	}
	if !routeActive {
		t.Errorf("expected dynamic route to be active after driver.Start")
	}

	// 3. Stop deployment: verify dynamic route is revoked cleanly
	err = driver.Stop(ctx, res.Deployment.ID)
	if err != nil {
		t.Fatalf("Stop error: %v", err)
	}

	routeActiveAfterStop, err := router.GetRouteStatus(ctx, "vh-app")
	if err != nil {
		t.Fatalf("GetRouteStatus error: %v", err)
	}
	if routeActiveAfterStop {
		t.Errorf("expected dynamic route to be revoked after driver.Stop")
	}

	// 4. Delete deployment: verify resources cleaned
	err = driver.Delete(ctx, res.Deployment.ID)
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	deletedDep, err := store.GetDeployment(ctx, res.Deployment.ID)
	if err == nil && deletedDep != nil {
		t.Errorf("expected deployment to be deleted from store")
	}
}

// TestRealCaddyIntegration runs against a real Caddy binary on test ports if available.
// It verifies:
// 1. Real Caddy Admin API route injection without service restart.
// 2. Graceful 503 fallback handling when the backend process is not responding.
// 3. Reverse proxy forwarding (HTTP 200) when backend process is running.
// 4. Clean unregistration of route without service restart.
func TestRealCaddyIntegration(t *testing.T) {
	caddyPath := filepath.Join(os.Getenv("HOME"), "go", "bin", "caddy")
	if _, err := os.Stat(caddyPath); err != nil {
		t.Skip("Caddy binary not found at", caddyPath)
	}

	adminPort := 22019
	listenPort := 28443

	tmpCfg := filepath.Join(t.TempDir(), "caddy_test.json")
	initialConfig := fmt.Sprintf(`{
		"admin": {
			"listen": "127.0.0.1:%d"
		},
		"apps": {
			"http": {
				"servers": {
					"srv0": {
						"listen": [":%d"],
						"automatic_https": {
							"disable": true
						},
						"routes": []
					}
				}
			}
		}
	}`, adminPort, listenPort)

	if err := os.WriteFile(tmpCfg, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("failed to write test caddy config: %v", err)
	}

	// Start test Caddy in background without blocking on daemon pipe
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("failed to open devnull: %v", err)
	}
	defer devNull.Close()

	cmd := exec.Command(caddyPath, "run", "--config", tmpCfg)
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start test Caddy: %v", err)
	}
	defer func() {
		stopCmd := exec.Command(caddyPath, "stop", "--address", fmt.Sprintf("127.0.0.1:%d", adminPort))
		_ = stopCmd.Run()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()

	ctx := context.Background()
	adminURL := fmt.Sprintf("http://127.0.0.1:%d", adminPort)
	cfg := CaddyConfig{
		AdminURL:     adminURL,
		BaseDomain:   "cuan.localhost",
		ServerName:   "srv0",
		FallbackHTML: DefaultFallbackHTML,
	}
	router := NewCaddyRouter(cfg)

	// Wait for Caddy admin to become reachable (up to 3s)
	ready := false
	for i := 0; i < 30; i++ {
		if router.IsAdminReachable(ctx) {
			ready = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		t.Fatalf("test Caddy admin at %s not reachable after startup", adminURL)
	}

	// Pick a test backend port that has no listener yet
	backendPort := 29099
	slug := "test-graceful"

	// 1. Dynamic route registration WITHOUT restarting Caddy
	err = router.RegisterRoute(ctx, slug, backendPort)
	if err != nil {
		t.Fatalf("RegisterRoute failed on real Caddy: %v", err)
	}

	respConfig, _ := http.Get(adminURL + "/config/")
	if respConfig != nil {
		cBytes, _ := io.ReadAll(respConfig.Body)
		t.Logf("CURRENT CADDY CONFIG:\n%s", string(cBytes))
		respConfig.Body.Close()
	}

	// Verify route is registered in Caddy
	routeActive, err := router.GetRouteStatus(ctx, slug)
	if err != nil {
		t.Fatalf("GetRouteStatus error: %v", err)
	}
	if !routeActive {
		t.Fatalf("expected route %s to be registered in real Caddy", slug)
	}

	// 2. Query Caddy via HTTP: backend is DOWN -> must return graceful 503 fallback
	targetHost := fmt.Sprintf("%s.cuan.localhost", slug)
	caddyClient := &http.Client{Timeout: 3 * time.Second}

	reqFallback, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/", listenPort), nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	reqFallback.Host = targetHost

	respFallback, err := caddyClient.Do(reqFallback)
	if err != nil {
		t.Fatalf("query Caddy fallback failed: %v", err)
	}
	defer respFallback.Body.Close()

	if respFallback.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 Service Unavailable when backend is down, got %d", respFallback.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(respFallback.Body)
	bodyStr := string(bodyBytes)
	if !strings.Contains(bodyStr, "Aplikasi Sedang Dimuat") {
		t.Errorf("expected fallback body to contain friendly Indonesian explanation, got:\n%s", bodyStr)
	}

	// 3. Start a dummy backend HTTP server on backendPort
	backendListener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", backendPort))
	if err != nil {
		t.Fatalf("failed to bind backend test port: %v", err)
	}
	defer backendListener.Close()

	backendServer := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify reverse proxy forwarded headers
			xfp := r.Header.Get("X-Forwarded-Proto")
			xreal := r.Header.Get("X-Real-IP")
			w.Header().Set("X-Backend-Handled", "true")
			w.Header().Set("X-Echo-XFP", xfp)
			w.Header().Set("X-Echo-XReal", xreal)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("Hello from dynamic backend!"))
		}),
	}
	go func() {
		_ = backendServer.Serve(backendListener)
	}()

	// Query Caddy via HTTP: backend is UP -> must return 200 OK with backend response
	reqUp, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/", listenPort), nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	reqUp.Host = targetHost

	respUp, err := caddyClient.Do(reqUp)
	if err != nil {
		t.Fatalf("query Caddy upstream failed: %v", err)
	}
	defer respUp.Body.Close()

	if respUp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK when backend is up, got %d", respUp.StatusCode)
	}
	upBody, _ := io.ReadAll(respUp.Body)
	if !strings.Contains(string(upBody), "Hello from dynamic backend!") {
		t.Errorf("expected proxied backend body, got %q", string(upBody))
	}

	// 4. Dynamic route revocation WITHOUT restarting Caddy
	err = router.UnregisterRoute(ctx, slug)
	if err != nil {
		t.Fatalf("UnregisterRoute failed on real Caddy: %v", err)
	}

	routeActiveAfter, err := router.GetRouteStatus(ctx, slug)
	if err != nil {
		t.Fatalf("GetRouteStatus after unregister error: %v", err)
	}
	if routeActiveAfter {
		t.Errorf("expected route to be removed from real Caddy")
	}

	// Query Caddy after unregistration -> should no longer match targetHost
	reqAfter, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/", listenPort), nil)
	reqAfter.Host = targetHost
	respAfter, err := caddyClient.Do(reqAfter)
	if err == nil {
		defer respAfter.Body.Close()
		// Since no routes match targetHost, Caddy returns an empty 200 or 404 (no match), definitely not proxied
		if respAfter.Header.Get("X-Backend-Handled") == "true" {
			t.Errorf("route was not revoked: backend was still contacted")
		}
	}
}

// TestCaddyCustomDomainRouting verifies dynamic reverse proxy registration,
// target mapping (local sandbox and remote BYOS runner), status checking,
// unregistration, and validation.
func TestCaddyCustomDomainRouting(t *testing.T) {
	var mu sync.Mutex
	routesStore := make(map[string]map[string]interface{})
	errorsStore := make(map[string]map[string]interface{})
	tlsStore := make(map[string]interface{})

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		path := r.URL.Path

		if path == "/config/" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"admin": map[string]interface{}{}})
			return
		}

		if path == "/config/apps/http/servers/srv0" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"listen": []string{":443"}})
			return
		}

		if path == "/config/apps/http/servers/srv0/errors" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"routes": []interface{}{}})
			return
		}

		if path == "/config/apps/http/servers/srv0/routes" && r.Method == http.MethodPost {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			id, _ := body["@id"].(string)
			routesStore[id] = body
			w.WriteHeader(http.StatusOK)
			return
		}

		if path == "/config/apps/http/servers/srv0/errors/routes" && r.Method == http.MethodPost {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			id, _ := body["@id"].(string)
			errorsStore[id] = body
			w.WriteHeader(http.StatusOK)
			return
		}

		if strings.HasPrefix(path, "/config/apps/tls") {
			var body interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			tlsStore[path] = body
			w.WriteHeader(http.StatusOK)
			return
		}

		if strings.HasPrefix(path, "/id/") {
			id := strings.TrimPrefix(path, "/id/")
			if r.Method == http.MethodGet {
				if item, exists := routesStore[id]; exists {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(item)
					return
				}
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if r.Method == http.MethodDelete {
				delete(routesStore, id)
				delete(errorsStore, id)
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	ctx := context.Background()
	tempSnippetsDir := t.TempDir()

	cfg := CaddyConfig{
		AdminURL:     mockServer.URL,
		BaseDomain:   "cuan.dojo.cc",
		ServerName:   "srv0",
		SnippetsDir:  tempSnippetsDir,
		HTTPClient:   mockServer.Client(),
		FallbackHTML: "<html><body>Loading</body></html>",
		AskURL:       "http://localhost:8080/api/caddy/ask",
		OnDemandTLS:  true,
	}
	router := NewCaddyRouter(cfg)

	// 1. Register local sandbox custom domain route
	err := router.AddCustomDomainRoute(ctx, "app.clientbrand.com", LocalTarget(8081))
	if err != nil {
		t.Fatalf("AddCustomDomainRoute failed: %v", err)
	}

	// Verify route status
	active, err := router.GetCustomDomainRouteStatus(ctx, "app.clientbrand.com")
	if err != nil {
		t.Fatalf("GetCustomDomainRouteStatus error: %v", err)
	}
	if !active {
		t.Errorf("expected route for app.clientbrand.com to be active")
	}

	// Verify mock Caddy received correct payload
	mu.Lock()
	routePayload, exists := routesStore["cuan-custom-app.clientbrand.com"]
	mu.Unlock()
	if !exists {
		t.Fatalf("expected route cuan-custom-app.clientbrand.com in Caddy store")
	}

	handles := routePayload["handle"].([]interface{})
	upstreamDial := handles[0].(map[string]interface{})["upstreams"].([]interface{})[0].(map[string]interface{})["dial"].(string)
	if upstreamDial != "127.0.0.1:8081" {
		t.Errorf("expected upstream dial '127.0.0.1:8081', got %q", upstreamDial)
	}

	// Verify snippet file was written
	snippetPath := filepath.Join(tempSnippetsDir, "custom_app.clientbrand.com.caddy")
	content, err := os.ReadFile(snippetPath)
	if err != nil {
		t.Fatalf("failed to read snippet file: %v", err)
	}
	if !strings.Contains(string(content), "on_demand") || !strings.Contains(string(content), "127.0.0.1:8081") {
		t.Errorf("snippet content missing on_demand or upstream: %s", string(content))
	}

	// 2. Register remote BYOS runner custom domain route
	err = router.AddCustomDomainRoute(ctx, "remote.clientbrand.com", RemoteTarget("10.0.0.50", 9000))
	if err != nil {
		t.Fatalf("AddCustomDomainRoute for remote BYOS failed: %v", err)
	}

	mu.Lock()
	remotePayload, exists := routesStore["cuan-custom-remote.clientbrand.com"]
	mu.Unlock()
	if !exists {
		t.Fatalf("expected route cuan-custom-remote.clientbrand.com in Caddy store")
	}

	remoteHandles := remotePayload["handle"].([]interface{})
	remoteDial := remoteHandles[0].(map[string]interface{})["upstreams"].([]interface{})[0].(map[string]interface{})["dial"].(string)
	if remoteDial != "10.0.0.50:9000" {
		t.Errorf("expected remote upstream dial '10.0.0.50:9000', got %q", remoteDial)
	}

	// 3. Remove custom domain route (clean unregister without reload)
	err = router.RemoveCustomDomainRoute(ctx, "app.clientbrand.com")
	if err != nil {
		t.Fatalf("RemoveCustomDomainRoute failed: %v", err)
	}

	activeAfter, err := router.GetCustomDomainRouteStatus(ctx, "app.clientbrand.com")
	if err != nil {
		t.Fatalf("GetCustomDomainRouteStatus error: %v", err)
	}
	if activeAfter {
		t.Errorf("expected route for app.clientbrand.com to be inactive after removal")
	}

	// Verify snippet file was deleted
	if _, err := os.Stat(snippetPath); !os.IsNotExist(err) {
		t.Errorf("expected snippet file to be removed, but it still exists")
	}

	// 4. Test input validation
	if err := router.AddCustomDomainRoute(ctx, "", LocalTarget(8080)); err == nil {
		t.Errorf("expected error on empty domain, got nil")
	}
	if err := router.AddCustomDomainRoute(ctx, "http://invalid.com", LocalTarget(8080)); err == nil {
		t.Errorf("expected error on invalid domain format, got nil")
	}
	if err := router.AddCustomDomainRoute(ctx, "valid.domain.com", LocalTarget(0)); err == nil {
		t.Errorf("expected error on invalid port 0, got nil")
	}
	if err := router.AddCustomDomainRoute(ctx, "valid.domain.com", LocalTarget(70000)); err == nil {
		t.Errorf("expected error on invalid port 70000, got nil")
	}
}

// TestCaddyCustomDomainDuplicationPrevention verifies that registering the same custom domain
// multiple times updates the existing route cleanly without creating duplicate routes in Caddy or in memory.
func TestCaddyCustomDomainDuplicationPrevention(t *testing.T) {
	var mu sync.Mutex
	routesStore := make(map[string]map[string]interface{})

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		path := r.URL.Path

		if path == "/config/" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"admin": map[string]interface{}{}})
			return
		}

		if path == "/config/apps/http/servers/srv0" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"listen": []string{":443"}})
			return
		}

		if path == "/config/apps/http/servers/srv0/errors" {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"routes": []interface{}{}})
			return
		}

		if path == "/config/apps/http/servers/srv0/routes" && r.Method == http.MethodPost {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			id, _ := body["@id"].(string)
			routesStore[id] = body
			w.WriteHeader(http.StatusOK)
			return
		}

		if path == "/config/apps/http/servers/srv0/errors/routes" && r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			return
		}

		if strings.HasPrefix(path, "/config/apps/tls") {
			w.WriteHeader(http.StatusOK)
			return
		}

		if strings.HasPrefix(path, "/id/") {
			id := strings.TrimPrefix(path, "/id/")
			if r.Method == http.MethodGet {
				if item, exists := routesStore[id]; exists {
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(item)
					return
				}
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if r.Method == http.MethodDelete {
				delete(routesStore, id)
				w.WriteHeader(http.StatusOK)
				return
			}
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	ctx := context.Background()
	cfg := CaddyConfig{
		AdminURL:     mockServer.URL,
		BaseDomain:   "cuan.dojo.cc",
		ServerName:   "srv0",
		HTTPClient:   mockServer.Client(),
		FallbackHTML: "<html><body>Loading</body></html>",
	}
	router := NewCaddyRouter(cfg)

	// Step 1: First registration
	err := router.AddCustomDomainRoute(ctx, "app.startup.io", LocalTarget(8001))
	if err != nil {
		t.Fatalf("first AddCustomDomainRoute failed: %v", err)
	}

	mu.Lock()
	initialCount := len(routesStore)
	mu.Unlock()
	if initialCount != 1 {
		t.Errorf("expected 1 route in Caddy routesStore, got %d", initialCount)
	}

	routesList := router.ListCustomDomainRoutes()
	if len(routesList) != 1 {
		t.Errorf("expected 1 custom route in in-memory list, got %d", len(routesList))
	}
	if routesList[0].Target.InternalPort != 8001 {
		t.Errorf("expected target port 8001, got %d", routesList[0].Target.InternalPort)
	}

	// Step 2: Second registration (duplicate domain, updated port)
	err = router.AddCustomDomainRoute(ctx, "app.startup.io", LocalTarget(8002))
	if err != nil {
		t.Fatalf("second AddCustomDomainRoute failed: %v", err)
	}

	mu.Lock()
	updatedCount := len(routesStore)
	routePayload := routesStore["cuan-custom-app.startup.io"]
	mu.Unlock()

	// Duplication check: Caddy routesStore must STILL have exactly 1 route for this ID
	if updatedCount != 1 {
		t.Errorf("duplication prevention failed: expected 1 route in routesStore, got %d", updatedCount)
	}

	handles := routePayload["handle"].([]interface{})
	updatedDial := handles[0].(map[string]interface{})["upstreams"].([]interface{})[0].(map[string]interface{})["dial"].(string)
	if updatedDial != "127.0.0.1:8002" {
		t.Errorf("expected updated dial '127.0.0.1:8002', got %q", updatedDial)
	}

	// In-memory list must still have exactly 1 entry
	updatedRoutesList := router.ListCustomDomainRoutes()
	if len(updatedRoutesList) != 1 {
		t.Errorf("duplication prevention failed in-memory: expected 1 route, got %d", len(updatedRoutesList))
	}
	if updatedRoutesList[0].Target.InternalPort != 8002 {
		t.Errorf("expected updated target port 8002, got %d", updatedRoutesList[0].Target.InternalPort)
	}
}

// TestCaddyOnDemandTLSConfiguration verifies ask URL configuration and policies.
func TestCaddyOnDemandTLSConfiguration(t *testing.T) {
	var mu sync.Mutex
	tlsCalls := make(map[string]interface{})

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		path := r.URL.Path

		if path == "/config/" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"admin": map[string]interface{}{}})
			return
		}

		if strings.HasPrefix(path, "/config/apps/tls") {
			var body interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			tlsCalls[path] = body
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockServer.Close()

	ctx := context.Background()
	tempSnippetsDir := t.TempDir()

	cfg := CaddyConfig{
		AdminURL:    mockServer.URL,
		BaseDomain:  "cuan.dojo.cc",
		ServerName:  "srv0",
		SnippetsDir: tempSnippetsDir,
		HTTPClient:  mockServer.Client(),
		AskURL:      "http://127.0.0.1:8080/api/caddy/ask",
		OnDemandTLS: true,
	}
	router := NewCaddyRouter(cfg)

	err := router.ConfigureOnDemandTLS(ctx, "http://127.0.0.1:8080/api/caddy/ask")
	if err != nil {
		t.Fatalf("ConfigureOnDemandTLS failed: %v", err)
	}

	if router.AskURL() != "http://127.0.0.1:8080/api/caddy/ask" {
		t.Errorf("expected AskURL to be updated, got %q", router.AskURL())
	}
	if !router.OnDemandTLS() {
		t.Errorf("expected OnDemandTLS to be enabled")
	}

	// Verify global snippet was written
	globalSnippetPath := filepath.Join(tempSnippetsDir, "on_demand_tls.caddy")
	content, err := os.ReadFile(globalSnippetPath)
	if err != nil {
		t.Fatalf("failed to read global snippet: %v", err)
	}
	if !strings.Contains(string(content), "http://127.0.0.1:8080/api/caddy/ask") {
		t.Errorf("snippet content missing ask url: %s", string(content))
	}
}

