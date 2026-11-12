package deployer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	cuanDB "github.com/buchin/cuan/internal/db"
)

// CustomDomainTarget represents the upstream dial destination for a custom domain.
type CustomDomainTarget struct {
	Host         string `json:"host,omitempty"` // dial host: empty or "127.0.0.1" for local, or remote IP/hostname
	InternalPort int    `json:"internal_port"`   // internal target port
}

// DialAddress returns the "host:port" upstream dial string.
func (t CustomDomainTarget) DialAddress() string {
	host := strings.TrimSpace(t.Host)
	if host == "" || host == "localhost" {
		host = "127.0.0.1"
	}
	return fmt.Sprintf("%s:%d", host, t.InternalPort)
}

// LocalTarget creates a CustomDomainTarget for local sandbox deployments.
func LocalTarget(internalPort int) CustomDomainTarget {
	return CustomDomainTarget{Host: "127.0.0.1", InternalPort: internalPort}
}

// RemoteTarget creates a CustomDomainTarget for remote BYOS runner deployments.
func RemoteTarget(host string, internalPort int) CustomDomainTarget {
	return CustomDomainTarget{Host: host, InternalPort: internalPort}
}

// CustomDomainRouteInfo captures active routing metadata for a custom domain.
type CustomDomainRouteInfo struct {
	Domain       string             `json:"domain"`
	Target       CustomDomainTarget `json:"target"`
	RegisteredAt time.Time          `json:"registered_at"`
}

// Router defines the contract for dynamic virtual host and reverse proxy routing.
type Router interface {
	// RegisterRoute dynamically injects or updates a virtual host reverse proxy route for the slug pointing to internalPort.
	RegisterRoute(ctx context.Context, slug string, internalPort int) error

	// UnregisterRoute revokes the dynamic virtual host route cleanly.
	UnregisterRoute(ctx context.Context, slug string) error

	// FormatDomain returns the FQDN for the slug (e.g. {slug}.example.com).
	FormatDomain(slug string) string

	// BaseDomain returns the active base domain.
	BaseDomain() string

	// GetRouteStatus returns true if the route is currently active in the reverse proxy.
	GetRouteStatus(ctx context.Context, slug string) (bool, error)

	// Reconcile synchronizes active routes with the given list of running deployments.
	Reconcile(ctx context.Context, activeDeployments []*Deployment) error

	// AddCustomDomainRoute maps a custom domain (e.g. app.client.com) to a deployment target.
	AddCustomDomainRoute(ctx context.Context, domain string, target CustomDomainTarget) error

	// RemoveCustomDomainRoute revokes a custom domain route cleanly without downtime.
	RemoveCustomDomainRoute(ctx context.Context, domain string) error

	// GetCustomDomainRouteStatus checks whether a custom domain route is currently registered.
	GetCustomDomainRouteStatus(ctx context.Context, domain string) (bool, error)

	// ConfigureOnDemandTLS configures Caddy's On-Demand TLS ask endpoint.
	ConfigureOnDemandTLS(ctx context.Context, askURL string) error
}

// CaddyConfig configures the dynamic Caddy virtual host engine.
type CaddyConfig struct {
	AdminURL              string       // e.g. "http://localhost:2019" (env: CADDY_ADMIN_URL)
	BaseDomain            string       // e.g. "example.com" (env: BASE_DOMAIN)
	ServerName            string       // e.g. "srv0" (env: CADDY_SERVER_NAME)
	SnippetsDir           string       // directory for Caddyfile snippets fallback (optional)
	FallbackHTML          string       // custom HTML body for 502/503 graceful fallback
	HTTPClient            *http.Client // custom HTTP client (for mocks/timeouts)
	AutoCreateServer      bool         // auto-initialize HTTP server if missing in Caddy
	AutomaticHTTPSDisable bool         // disable auto-https redirects (useful in local dev / non-root)
	AskURL                string       // e.g. "http://localhost:8080/api/caddy/ask" (env: CADDY_ASK_URL)
	OnDemandTLS           bool         // enable on-demand TLS automation in Caddy (env: CADDY_ON_DEMAND_TLS)
}

// DefaultCaddyConfig builds a default configuration using environment variables and sensible defaults.
func DefaultCaddyConfig() CaddyConfig {
	baseDomain := os.Getenv("BASE_DOMAIN")
	if baseDomain == "" {
		baseDomain = os.Getenv("CUAN_BASE_DOMAIN")
	}
	if baseDomain == "" {
		baseDomain = "localhost"
	}

	adminURL := os.Getenv("CADDY_ADMIN_URL")
	if adminURL == "" {
		adminURL = "http://localhost:2019"
	}

	serverName := os.Getenv("CADDY_SERVER_NAME")
	if serverName == "" {
		serverName = "srv0"
	}

	snippetsDir := os.Getenv("CADDY_SNIPPETS_DIR")

	askURL := os.Getenv("CADDY_ASK_URL")
	if askURL == "" {
		askURL = os.Getenv("CUAN_ASK_URL")
	}
	if askURL == "" {
		askURL = "http://localhost:8080/api/caddy/ask"
	}

	onDemandTLSEnv := strings.ToLower(strings.TrimSpace(os.Getenv("CADDY_ON_DEMAND_TLS")))
	onDemandTLS := true
	if onDemandTLSEnv == "false" || onDemandTLSEnv == "0" {
		onDemandTLS = false
	}

	return CaddyConfig{
		AdminURL:              adminURL,
		BaseDomain:            baseDomain,
		ServerName:            serverName,
		SnippetsDir:           snippetsDir,
		FallbackHTML:          DefaultFallbackHTML,
		HTTPClient:            &http.Client{Timeout: 5 * time.Second},
		AutoCreateServer:      true,
		AutomaticHTTPSDisable: false,
		AskURL:                askURL,
		OnDemandTLS:           onDemandTLS,
	}
}

// CaddyRouteInfo captures active routing metadata for a deployment.
type CaddyRouteInfo struct {
	Slug         string    `json:"slug"`
	Domain       string    `json:"domain"`
	InternalPort int       `json:"internal_port"`
	RegisteredAt time.Time `json:"registered_at"`
}

// CaddyRouter implements the Router interface using Caddy Admin API (v2) and dynamic snippets.
type CaddyRouter struct {
	adminURL              string
	baseDomain            string
	serverName            string
	snippetsDir           string
	fallbackHTML          string
	httpClient            *http.Client
	autoCreateServer      bool
	automaticHTTPSDisable bool
	askURL                string
	onDemandTLS           bool

	routes       map[string]*CaddyRouteInfo
	customRoutes map[string]*CustomDomainRouteInfo
	mu           sync.RWMutex
}

// NewCaddyRouter initializes a new CaddyRouter with the supplied configuration.
func NewCaddyRouter(cfg CaddyConfig) *CaddyRouter {
	if cfg.AdminURL == "" {
		cfg.AdminURL = "http://localhost:2019"
	}
	cfg.AdminURL = strings.TrimRight(cfg.AdminURL, "/")

	if cfg.BaseDomain == "" {
		cfg.BaseDomain = "localhost"
	}
	cfg.BaseDomain = strings.ToLower(strings.TrimSpace(cfg.BaseDomain))

	if cfg.ServerName == "" {
		cfg.ServerName = "srv0"
	}

	if cfg.FallbackHTML == "" {
		cfg.FallbackHTML = DefaultFallbackHTML
	}

	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 5 * time.Second}
	}

	if cfg.AskURL == "" {
		cfg.AskURL = "http://localhost:8080/api/caddy/ask"
	}

	return &CaddyRouter{
		adminURL:              cfg.AdminURL,
		baseDomain:            cfg.BaseDomain,
		serverName:            cfg.ServerName,
		snippetsDir:           cfg.SnippetsDir,
		fallbackHTML:          cfg.FallbackHTML,
		httpClient:            cfg.HTTPClient,
		autoCreateServer:      cfg.AutoCreateServer,
		automaticHTTPSDisable: cfg.AutomaticHTTPSDisable,
		askURL:                cfg.AskURL,
		onDemandTLS:           cfg.OnDemandTLS,
		routes:                make(map[string]*CaddyRouteInfo),
		customRoutes:          make(map[string]*CustomDomainRouteInfo),
	}
}

// BaseDomain returns the configured base domain.
func (r *CaddyRouter) BaseDomain() string {
	return r.baseDomain
}

// AdminURL returns the configured Caddy Admin API base URL.
func (r *CaddyRouter) AdminURL() string {
	return r.adminURL
}

// FormatDomain formats the full domain name for a given slug: {slug}.{baseDomain}.
func (r *CaddyRouter) FormatDomain(slug string) string {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if slug == "" {
		slug = "app"
	}
	return fmt.Sprintf("%s.%s", slug, r.baseDomain)
}

// IsAdminReachable checks if the Caddy Admin API is responding to health requests.
func (r *CaddyRouter) IsAdminReachable(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.adminURL+"/config/", nil)
	if err != nil {
		return false
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// RegisterRoute registers or updates a dynamic reverse proxy route in Caddy for the given slug and internalPort.
// It also configures a custom 503 fallback error route in Caddy for when the upstream is booting or down.
func (r *CaddyRouter) RegisterRoute(ctx context.Context, slug string, internalPort int) error {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if slug == "" {
		return fmt.Errorf("slug cannot be empty")
	}
	if internalPort <= 0 || internalPort > 65535 {
		return fmt.Errorf("invalid internal_port: %d (must be between 1 and 65535)", internalPort)
	}

	domain := r.FormatDomain(slug)
	routeID := fmt.Sprintf("cuan-route-%s", slug)
	errorRouteID := fmt.Sprintf("cuan-error-%s", slug)

	// 1. If Caddy Admin API is reachable, inject route via API
	if r.IsAdminReachable(ctx) {
		serverName, err := r.resolveServerName(ctx)
		if err != nil {
			return fmt.Errorf("resolve caddy server name: %w", err)
		}

		// Idempotency: Clean up any previous route or error route with these IDs
		_ = r.deleteID(ctx, routeID)
		_ = r.deleteID(ctx, errorRouteID)

		// A. Register main reverse proxy route
		routePayload := map[string]interface{}{
			"@id": routeID,
			"match": []map[string]interface{}{
				{
					"host": []string{domain},
				},
			},
			"handle": []map[string]interface{}{
				{
					"handler": "reverse_proxy",
					"headers": map[string]interface{}{
						"request": map[string]interface{}{
							"set": map[string][]string{
								"Host":              {"{http.request.host}"},
								"X-Real-IP":         {"{http.request.remote.host}"},
								"X-Forwarded-For":   {"{http.request.remote.host}"},
								"X-Forwarded-Proto": {"{http.request.scheme}"},
							},
						},
					},
					"upstreams": []map[string]interface{}{
						{
							"dial": fmt.Sprintf("127.0.0.1:%d", internalPort),
						},
					},
				},
			},
			"terminal": true,
		}

		routesURL := fmt.Sprintf("%s/config/apps/http/servers/%s/routes", r.adminURL, serverName)
		if err := r.postJSON(ctx, routesURL, routePayload); err != nil {
			return fmt.Errorf("register caddy route: %w", err)
		}

		// B. Register custom 503 graceful fallback in errors/routes
		if err := r.ensureErrorsRoutesExists(ctx, serverName); err == nil {
			errorRoutePayload := map[string]interface{}{
				"@id": errorRouteID,
				"match": []map[string]interface{}{
					{
						"host": []string{domain},
					},
				},
				"handle": []map[string]interface{}{
					{
						"handler": "subroute",
						"routes": []map[string]interface{}{
							{
								"handle": []map[string]interface{}{
									{
										"handler":     "static_response",
										"status_code": 503,
										"headers": map[string][]string{
											"Content-Type": {"text/html; charset=utf-8"},
											"Retry-After":  {"5"},
										},
										"body": r.fallbackHTML,
									},
								},
							},
						},
					},
				},
				"terminal": true,
			}

			errRoutesURL := fmt.Sprintf("%s/config/apps/http/servers/%s/errors/routes", r.adminURL, serverName)
			_ = r.postJSON(ctx, errRoutesURL, errorRoutePayload)
		}
	}

	// 2. Write dynamic snippet if SnippetsDir is specified
	if r.snippetsDir != "" {
		_ = r.WriteSnippet(slug, internalPort)
	}

	// 3. Update in-memory route registry
	r.mu.Lock()
	r.routes[slug] = &CaddyRouteInfo{
		Slug:         slug,
		Domain:       domain,
		InternalPort: internalPort,
		RegisteredAt: time.Now().UTC(),
	}
	r.mu.Unlock()

	return nil
}

// UnregisterRoute revokes the dynamic virtual host route cleanly from Caddy.
func (r *CaddyRouter) UnregisterRoute(ctx context.Context, slug string) error {
	slug = strings.TrimSpace(strings.ToLower(slug))
	if slug == "" {
		return nil
	}

	routeID := fmt.Sprintf("cuan-route-%s", slug)
	errorRouteID := fmt.Sprintf("cuan-error-%s", slug)

	// 1. Delete from Caddy Admin API if reachable
	if r.IsAdminReachable(ctx) {
		_ = r.deleteID(ctx, routeID)
		_ = r.deleteID(ctx, errorRouteID)
	}

	// 2. Remove snippet if SnippetsDir is specified
	if r.snippetsDir != "" {
		_ = r.RemoveSnippet(slug)
	}

	// 3. Remove from in-memory cache
	r.mu.Lock()
	delete(r.routes, slug)
	r.mu.Unlock()

	return nil
}

// GetRouteStatus checks whether a route for the given slug is currently registered.
func (r *CaddyRouter) GetRouteStatus(ctx context.Context, slug string) (bool, error) {
	slug = strings.TrimSpace(strings.ToLower(slug))
	routeID := fmt.Sprintf("cuan-route-%s", slug)

	if r.IsAdminReachable(ctx) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/id/%s", r.adminURL, routeID), nil)
		if err != nil {
			return false, err
		}
		resp, err := r.httpClient.Do(req)
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return true, nil
		}
		if resp.StatusCode == http.StatusNotFound {
			return false, nil
		}
	}

	// Fallback to in-memory cache
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.routes[slug]
	return exists, nil
}

// Reconcile synchronizes active routes with all currently running deployments.
// This addresses the sprint risk: Caddy restart leading to route drop.
func (r *CaddyRouter) Reconcile(ctx context.Context, activeDeployments []*Deployment) error {
	for _, dep := range activeDeployments {
		if dep.Status == StatusRunning && dep.InternalPort > 0 && dep.Slug != "" {
			if err := r.RegisterRoute(ctx, dep.Slug, dep.InternalPort); err != nil {
				return fmt.Errorf("reconcile route for slug %s: %w", dep.Slug, err)
			}
		}
	}
	return nil
}

// GenerateSnippet renders a Caddyfile snippet string for the specified slug and internalPort.
func (r *CaddyRouter) GenerateSnippet(slug string, internalPort int) string {
	domain := r.FormatDomain(slug)
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Auto-generated Caddyfile snippet by Cuan Deployer\n"))
	sb.WriteString(fmt.Sprintf("# Slug: %s | Port: %d | GeneratedAt: %s\n\n", slug, internalPort, time.Now().UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("%s {\n", domain))
	sb.WriteString(fmt.Sprintf("\treverse_proxy 127.0.0.1:%d {\n", internalPort))
	sb.WriteString("\t\theader_up Host {host}\n")
	sb.WriteString("\t\theader_up X-Real-IP {remote_host}\n")
	sb.WriteString("\t\theader_up X-Forwarded-For {remote_host}\n")
	sb.WriteString("\t\theader_up X-Forwarded-Proto {scheme}\n")
	sb.WriteString("\t}\n")
	sb.WriteString("\thandle_errors {\n")
	sb.WriteString("\t\trespond `")
	sb.WriteString(strings.ReplaceAll(r.fallbackHTML, "`", "'"))
	sb.WriteString("` 503 {\n")
	sb.WriteString("\t\t\tclose\n")
	sb.WriteString("\t\t}\n")
	sb.WriteString("\t}\n")
	sb.WriteString("}\n")

	return sb.String()
}

// WriteSnippet saves the generated Caddyfile snippet to the snippets directory.
func (r *CaddyRouter) WriteSnippet(slug string, internalPort int) error {
	if r.snippetsDir == "" {
		return nil
	}
	if err := os.MkdirAll(r.snippetsDir, 0755); err != nil {
		return fmt.Errorf("create snippets directory: %w", err)
	}
	filePath := filepath.Join(r.snippetsDir, fmt.Sprintf("%s.caddy", slug))
	content := r.GenerateSnippet(slug, internalPort)
	return os.WriteFile(filePath, []byte(content), 0644)
}

// RemoveSnippet removes the snippet file for the specified slug.
func (r *CaddyRouter) RemoveSnippet(slug string) error {
	if r.snippetsDir == "" {
		return nil
	}
	filePath := filepath.Join(r.snippetsDir, fmt.Sprintf("%s.caddy", slug))
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// AskURL returns the configured Caddy On-Demand TLS ask URL.
func (r *CaddyRouter) AskURL() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.askURL
}

// SetAskURL updates the Caddy On-Demand TLS ask URL.
func (r *CaddyRouter) SetAskURL(url string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.askURL = strings.TrimSpace(url)
}

// OnDemandTLS returns whether On-Demand TLS is enabled.
func (r *CaddyRouter) OnDemandTLS() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.onDemandTLS
}

// SetOnDemandTLS enables or disables On-Demand TLS.
func (r *CaddyRouter) SetOnDemandTLS(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onDemandTLS = enabled
}

// ConfigureOnDemandTLS dynamically configures Caddy's TLS automation policy with the ask URL.
func (r *CaddyRouter) ConfigureOnDemandTLS(ctx context.Context, askURL string) error {
	askURL = strings.TrimSpace(askURL)
	if askURL == "" {
		askURL = r.AskURL()
	}
	if askURL == "" {
		return fmt.Errorf("ask URL cannot be empty")
	}

	r.mu.Lock()
	r.askURL = askURL
	r.onDemandTLS = true
	r.mu.Unlock()

	if !r.IsAdminReachable(ctx) {
		if r.snippetsDir != "" {
			_ = r.writeOnDemandTLSSnippet(askURL)
		}
		return nil
	}

	// 1. Configure on_demand ask URL in /config/apps/tls/automation/on_demand
	onDemandURL := fmt.Sprintf("%s/config/apps/tls/automation/on_demand", r.adminURL)
	onDemandPayload := map[string]interface{}{
		"ask": askURL,
	}

	if err := r.putJSON(ctx, onDemandURL, onDemandPayload); err != nil {
		// If tls or automation doesn't exist yet, initialize full apps/tls
		tlsURL := fmt.Sprintf("%s/config/apps/tls", r.adminURL)
		fullTLSPayload := map[string]interface{}{
			"automation": map[string]interface{}{
				"on_demand": map[string]interface{}{
					"ask": askURL,
				},
				"policies": []map[string]interface{}{
					{
						"on_demand": true,
					},
				},
			},
		}
		if err2 := r.putJSON(ctx, tlsURL, fullTLSPayload); err2 != nil {
			return fmt.Errorf("configure caddy on-demand tls: %w", err2)
		}
	} else {
		// Ensure policies have on_demand: true
		policiesURL := fmt.Sprintf("%s/config/apps/tls/automation/policies", r.adminURL)
		policiesPayload := []map[string]interface{}{
			{
				"on_demand": true,
			},
		}
		_ = r.putJSON(ctx, policiesURL, policiesPayload)
	}

	if r.snippetsDir != "" {
		_ = r.writeOnDemandTLSSnippet(askURL)
	}

	return nil
}

// EnsureOnDemandTLS verifies and activates On-Demand TLS with the active ask URL.
func (r *CaddyRouter) EnsureOnDemandTLS(ctx context.Context) error {
	askURL := r.AskURL()
	if askURL == "" {
		return nil
	}
	return r.ConfigureOnDemandTLS(ctx, askURL)
}

func (r *CaddyRouter) writeOnDemandTLSSnippet(askURL string) error {
	if r.snippetsDir == "" {
		return nil
	}
	if err := os.MkdirAll(r.snippetsDir, 0755); err != nil {
		return fmt.Errorf("create snippets directory: %w", err)
	}
	filePath := filepath.Join(r.snippetsDir, "on_demand_tls.caddy")
	content := fmt.Sprintf("{\n\ton_demand_tls {\n\t\task %s\n\t}\n}\n", askURL)
	return os.WriteFile(filePath, []byte(content), 0644)
}

// AddCustomDomainRoute registers or updates a dynamic reverse proxy route for a custom domain.
// It maps the custom domain (Host/SNI) to the target upstream (local port or remote BYOS runner address)
// with zero downtime and without server reload.
func (r *CaddyRouter) AddCustomDomainRoute(ctx context.Context, domain string, target CustomDomainTarget) error {
	domain = cuanDB.NormalizeDomain(domain)
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}
	if err := cuanDB.ValidateDomain(domain); err != nil {
		return fmt.Errorf("invalid custom domain %q: %w", domain, err)
	}

	if target.InternalPort <= 0 || target.InternalPort > 65535 {
		return fmt.Errorf("invalid internal_port: %d (must be between 1 and 65535)", target.InternalPort)
	}

	upstreamDial := target.DialAddress()
	routeID := fmt.Sprintf("cuan-custom-%s", domain)
	errorRouteID := fmt.Sprintf("cuan-custom-error-%s", domain)

	// 1. If Caddy Admin API is reachable, inject route via API
	if r.IsAdminReachable(ctx) {
		serverName, err := r.resolveServerName(ctx)
		if err != nil {
			return fmt.Errorf("resolve caddy server name: %w", err)
		}

		// Ensure On-Demand TLS is configured if enabled
		if r.onDemandTLS {
			_ = r.EnsureOnDemandTLS(ctx)
		}

		// Idempotency / duplication prevention: Clean up any previous route with these IDs
		_ = r.deleteID(ctx, routeID)
		_ = r.deleteID(ctx, errorRouteID)

		// Register reverse proxy route
		routePayload := map[string]interface{}{
			"@id": routeID,
			"match": []map[string]interface{}{
				{
					"host": []string{domain},
				},
			},
			"handle": []map[string]interface{}{
				{
					"handler": "reverse_proxy",
					"headers": map[string]interface{}{
						"request": map[string]interface{}{
							"set": map[string][]string{
								"Host":              {"{http.request.host}"},
								"X-Real-IP":         {"{http.request.remote.host}"},
								"X-Forwarded-For":   {"{http.request.remote.host}"},
								"X-Forwarded-Proto": {"{http.request.scheme}"},
							},
						},
					},
					"upstreams": []map[string]interface{}{
						{
							"dial": upstreamDial,
						},
					},
				},
			},
			"terminal": true,
		}

		routesURL := fmt.Sprintf("%s/config/apps/http/servers/%s/routes", r.adminURL, serverName)
		if err := r.postJSON(ctx, routesURL, routePayload); err != nil {
			return fmt.Errorf("register custom domain route: %w", err)
		}

		// Register custom 503 graceful fallback in errors/routes
		if err := r.ensureErrorsRoutesExists(ctx, serverName); err == nil {
			errorRoutePayload := map[string]interface{}{
				"@id": errorRouteID,
				"match": []map[string]interface{}{
					{
						"host": []string{domain},
					},
				},
				"handle": []map[string]interface{}{
					{
						"handler": "subroute",
						"routes": []map[string]interface{}{
							{
								"handle": []map[string]interface{}{
									{
										"handler":     "static_response",
										"status_code": 503,
										"headers": map[string][]string{
											"Content-Type": {"text/html; charset=utf-8"},
											"Retry-After":  {"5"},
										},
										"body": r.fallbackHTML,
									},
								},
							},
						},
					},
				},
				"terminal": true,
			}
			errRoutesURL := fmt.Sprintf("%s/config/apps/http/servers/%s/errors/routes", r.adminURL, serverName)
			_ = r.postJSON(ctx, errRoutesURL, errorRoutePayload)
		}
	}

	// 2. Write dynamic snippet if SnippetsDir is specified
	if r.snippetsDir != "" {
		_ = r.WriteCustomDomainSnippet(domain, target)
	}

	// 3. Update in-memory registry
	r.mu.Lock()
	r.customRoutes[domain] = &CustomDomainRouteInfo{
		Domain:       domain,
		Target:       target,
		RegisteredAt: time.Now().UTC(),
	}
	r.mu.Unlock()

	return nil
}

// AddCustomDomainRoutePort is a convenience method that routes a custom domain to a local port.
func (r *CaddyRouter) AddCustomDomainRoutePort(ctx context.Context, domain string, internalPort int) error {
	return r.AddCustomDomainRoute(ctx, domain, LocalTarget(internalPort))
}

// RemoveCustomDomainRoute revokes the dynamic custom domain route cleanly without downtime.
func (r *CaddyRouter) RemoveCustomDomainRoute(ctx context.Context, domain string) error {
	domain = cuanDB.NormalizeDomain(domain)
	if domain == "" {
		return nil
	}

	routeID := fmt.Sprintf("cuan-custom-%s", domain)
	errorRouteID := fmt.Sprintf("cuan-custom-error-%s", domain)

	// 1. Delete from Caddy Admin API if reachable
	if r.IsAdminReachable(ctx) {
		_ = r.deleteID(ctx, routeID)
		_ = r.deleteID(ctx, errorRouteID)
	}

	// 2. Remove snippet if SnippetsDir is specified
	if r.snippetsDir != "" {
		_ = r.RemoveCustomDomainSnippet(domain)
	}

	// 3. Remove from in-memory cache
	r.mu.Lock()
	delete(r.customRoutes, domain)
	r.mu.Unlock()

	return nil
}

// UnregisterCustomDomainRoute is an alias for RemoveCustomDomainRoute.
func (r *CaddyRouter) UnregisterCustomDomainRoute(ctx context.Context, domain string) error {
	return r.RemoveCustomDomainRoute(ctx, domain)
}

// GetCustomDomainRouteStatus checks whether a route for the given custom domain is currently registered.
func (r *CaddyRouter) GetCustomDomainRouteStatus(ctx context.Context, domain string) (bool, error) {
	domain = cuanDB.NormalizeDomain(domain)
	if domain == "" {
		return false, nil
	}
	routeID := fmt.Sprintf("cuan-custom-%s", domain)

	if r.IsAdminReachable(ctx) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/id/%s", r.adminURL, url.PathEscape(routeID)), nil)
		if err != nil {
			return false, err
		}
		resp, err := r.httpClient.Do(req)
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			return true, nil
		}
		if resp.StatusCode == http.StatusNotFound {
			return false, nil
		}
	}

	// Fallback to in-memory cache
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.customRoutes[domain]
	return exists, nil
}

// ListCustomDomainRoutes returns all currently registered custom domain routes.
func (r *CaddyRouter) ListCustomDomainRoutes() []*CustomDomainRouteInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*CustomDomainRouteInfo, 0, len(r.customRoutes))
	for _, info := range r.customRoutes {
		list = append(list, info)
	}
	return list
}

// GenerateCustomDomainSnippet produces a Caddyfile snippet for the custom domain with On-Demand TLS.
func (r *CaddyRouter) GenerateCustomDomainSnippet(domain string, target CustomDomainTarget) string {
	upstreamDial := target.DialAddress()
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Auto-generated Caddyfile snippet for Custom Domain by Cuan Deployer\n"))
	sb.WriteString(fmt.Sprintf("# Domain: %s | Target: %s | GeneratedAt: %s\n\n", domain, upstreamDial, time.Now().UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("%s {\n", domain))
	sb.WriteString("\ttls {\n")
	sb.WriteString("\t\ton_demand\n")
	sb.WriteString("\t}\n")
	sb.WriteString(fmt.Sprintf("\treverse_proxy %s {\n", upstreamDial))
	sb.WriteString("\t\theader_up Host {host}\n")
	sb.WriteString("\t\theader_up X-Real-IP {remote_host}\n")
	sb.WriteString("\t\theader_up X-Forwarded-For {remote_host}\n")
	sb.WriteString("\t\theader_up X-Forwarded-Proto {scheme}\n")
	sb.WriteString("\t}\n")
	sb.WriteString("\thandle_errors {\n")
	sb.WriteString("\t\trespond `")
	sb.WriteString(strings.ReplaceAll(r.fallbackHTML, "`", "'"))
	sb.WriteString("` 503 {\n")
	sb.WriteString("\t\t\tclose\n")
	sb.WriteString("\t\t}\n")
	sb.WriteString("\t}\n")
	sb.WriteString("}\n")

	return sb.String()
}

// WriteCustomDomainSnippet writes the custom domain snippet to disk.
func (r *CaddyRouter) WriteCustomDomainSnippet(domain string, target CustomDomainTarget) error {
	if r.snippetsDir == "" {
		return nil
	}
	if err := os.MkdirAll(r.snippetsDir, 0755); err != nil {
		return fmt.Errorf("create snippets directory: %w", err)
	}
	filePath := filepath.Join(r.snippetsDir, fmt.Sprintf("custom_%s.caddy", domain))
	content := r.GenerateCustomDomainSnippet(domain, target)
	return os.WriteFile(filePath, []byte(content), 0644)
}

// RemoveCustomDomainSnippet deletes the custom domain snippet file.
func (r *CaddyRouter) RemoveCustomDomainSnippet(domain string) error {
	if r.snippetsDir == "" {
		return nil
	}
	filePath := filepath.Join(r.snippetsDir, fmt.Sprintf("custom_%s.caddy", domain))
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// resolveServerName queries Caddy to verify or detect the target HTTP server name (default: "srv0").
func (r *CaddyRouter) resolveServerName(ctx context.Context) (string, error) {
	if r.serverName != "" {
		checkURL := fmt.Sprintf("%s/config/apps/http/servers/%s", r.adminURL, r.serverName)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
		if resp, err := r.httpClient.Do(req); err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return r.serverName, nil
			}
		}
	}

	// Query available servers
	serversURL := fmt.Sprintf("%s/config/apps/http/servers", r.adminURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, serversURL, nil)
	if err != nil {
		return "srv0", err
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "srv0", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var servers map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&servers); err == nil && len(servers) > 0 {
			for name := range servers {
				r.serverName = name
				return name, nil
			}
		}
	}

	// If no server exists and AutoCreateServer is true, auto-initialize
	if r.autoCreateServer {
		initPayload := map[string]interface{}{
			"servers": map[string]interface{}{
				"srv0": map[string]interface{}{
					"listen": []string{":443"},
					"routes": []interface{}{},
				},
			},
		}
		if r.automaticHTTPSDisable {
			srv := initPayload["servers"].(map[string]interface{})["srv0"].(map[string]interface{})
			srv["automatic_https"] = map[string]interface{}{"disable": true}
		}

		httpAppURL := fmt.Sprintf("%s/config/apps/http", r.adminURL)
		_ = r.putJSON(ctx, httpAppURL, initPayload)
		r.serverName = "srv0"
		return "srv0", nil
	}

	return "srv0", nil
}

// ensureErrorsRoutesExists verifies that /config/apps/http/servers/{server}/errors exists.
func (r *CaddyRouter) ensureErrorsRoutesExists(ctx context.Context, serverName string) error {
	checkURL := fmt.Sprintf("%s/config/apps/http/servers/%s/errors", r.adminURL, serverName)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	resp, err := r.httpClient.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			trimmed := strings.TrimSpace(string(body))
			if trimmed != "" && trimmed != "null" && strings.Contains(trimmed, "routes") {
				return nil
			}
		}
	}

	// Initialize errors block
	initPayload := map[string]interface{}{
		"routes": []interface{}{},
	}
	return r.putJSON(ctx, checkURL, initPayload)
}

func (r *CaddyRouter) deleteID(ctx context.Context, id string) error {
	url := fmt.Sprintf("%s/id/%s", r.adminURL, url.PathEscape(id))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (r *CaddyRouter) postJSON(ctx context.Context, url string, data interface{}) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy api error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

func (r *CaddyRouter) putJSON(ctx context.Context, url string, data interface{}) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("caddy api error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// DefaultFallbackHTML provides an elegant, user-friendly 503 service unavailable page.
const DefaultFallbackHTML = `<!DOCTYPE html>
<html lang="id">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta http-equiv="refresh" content="5">
    <title>Aplikasi Sedang Dimuat | Cuan Deployer</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background-color: #0f172a;
            color: #f8fafc;
            display: flex;
            align-items: center;
            justify-content: center;
            min-height: 100vh;
            padding: 1.5rem;
        }
        .container {
            background-color: #1e293b;
            border: 1px solid #334155;
            border-radius: 1rem;
            padding: 2.5rem 2rem;
            max-width: 520px;
            width: 100%;
            text-align: center;
            box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.4), 0 8px 10px -6px rgba(0, 0, 0, 0.3);
        }
        .spinner {
            width: 48px;
            height: 48px;
            border: 4px solid #334155;
            border-top: 4px solid #38bdf8;
            border-radius: 50%;
            animation: spin 1s linear infinite;
            margin: 0 auto 1.5rem;
        }
        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }
        h1 {
            font-size: 1.35rem;
            font-weight: 700;
            color: #f1f5f9;
            margin-bottom: 0.75rem;
            line-height: 1.3;
        }
        p {
            font-size: 0.925rem;
            color: #94a3b8;
            line-height: 1.6;
            margin-bottom: 1.5rem;
        }
        .badge {
            display: inline-flex;
            align-items: center;
            background-color: #0369a1;
            color: #e0f2fe;
            padding: 0.35rem 0.85rem;
            border-radius: 9999px;
            font-size: 0.75rem;
            font-weight: 600;
            letter-spacing: 0.025em;
            text-transform: uppercase;
        }
        .hint {
            font-size: 0.8rem;
            color: #64748b;
            margin-top: 1.25rem;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="spinner"></div>
        <h1>Aplikasi Sedang Dimuat (Booting)</h1>
        <p>Aplikasi Anda di Cuan Deployer sedang dalam proses inisialisasi runtime atau backend port belum siap menerima koneksi. Halaman ini akan otomatis memuat ulang dalam 5 detik.</p>
        <span class="badge">Status: 503 Initializing Runtime</span>
        <div class="hint">Cuan Virtual Host Routing Engine • Graceful Fallback</div>
    </div>
</body>
</html>`
