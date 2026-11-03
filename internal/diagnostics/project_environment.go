package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/buchin/cuan/internal/project"
)

var environmentKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ProjectEnvironmentResult never includes values unless explicitly requested.
type ProjectEnvironmentResult struct {
	ProjectID string            `json:"project_id"`
	UpdatedAt string            `json:"updated_at"`
	Keys      []string          `json:"keys"`
	Values    map[string]string `json:"values"`
	Redacted  bool              `json:"redacted"`
}

func environmentResult(p map[string]any, reveal bool) ProjectEnvironmentResult {
	metadata, _ := p["metadata"].(map[string]any)
	settings, _ := metadata["cuan_settings"].(map[string]any)
	raw, _ := settings["environment"].(string)
	values := project.ParseEnvironment(raw)
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
		if !reveal {
			values[key] = "[REDACTED]"
		}
	}
	sort.Strings(keys)
	id, _ := p["id"].(string)
	version, _ := p["updated_at"].(string)
	return ProjectEnvironmentResult{id, version, keys, values, !reveal}
}

func (c *CuanClient) environmentProject(ctx context.Context, id string) (map[string]any, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("project_id is required")
	}
	if !c.IsConfigured() {
		return nil, fmt.Errorf("configure CUAN_API_URL and CUAN_AUTH_TOKEN first")
	}
	p, err := c.GetProject(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("could not load project settings")
	}
	if p["id"] == nil {
		return nil, fmt.Errorf("project not found")
	}
	return p, nil
}

func (c *CuanClient) GetProjectEnvironment(ctx context.Context, id string, reveal bool) (ProjectEnvironmentResult, error) {
	p, err := c.environmentProject(ctx, id)
	if err != nil {
		return ProjectEnvironmentResult{}, err
	}
	return environmentResult(p, reveal), nil
}

// UpdateProjectEnvironment merges selected keys while preserving other settings.
// The existing settings API enforces ownership and optimistic concurrency.
func (c *CuanClient) UpdateProjectEnvironment(ctx context.Context, id string, values map[string]string, unset []string) (ProjectEnvironmentResult, error) {
	if len(values) == 0 && len(unset) == 0 {
		return ProjectEnvironmentResult{}, fmt.Errorf("provide values or keys to unset")
	}
	for key, value := range values {
		if !environmentKey.MatchString(key) {
			return ProjectEnvironmentResult{}, fmt.Errorf("invalid environment variable name")
		}
		if strings.ContainsAny(value, "\r\n\x00") || strings.TrimSpace(value) != value {
			return ProjectEnvironmentResult{}, fmt.Errorf("environment values must be single-line without surrounding whitespace")
		}
		if strings.HasPrefix(value, "\"") || strings.HasPrefix(value, "'") {
			return ProjectEnvironmentResult{}, fmt.Errorf("pass unquoted environment values")
		}
	}
	removed := map[string]bool{}
	for _, key := range unset {
		if !environmentKey.MatchString(key) {
			return ProjectEnvironmentResult{}, fmt.Errorf("invalid environment variable name")
		}
		if _, ok := values[key]; ok {
			return ProjectEnvironmentResult{}, fmt.Errorf("cannot set and unset the same key")
		}
		removed[key] = true
	}
	p, err := c.environmentProject(ctx, id)
	if err != nil {
		return ProjectEnvironmentResult{}, err
	}
	metadata, _ := p["metadata"].(map[string]any)
	settings, _ := metadata["cuan_settings"].(map[string]any)
	raw, _ := settings["environment"].(string)
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		key, _, assignment := strings.Cut(trimmed, "=")
		_, replaced := values[strings.TrimSpace(key)]
		if assignment && !strings.HasPrefix(trimmed, "#") && (replaced || removed[strings.TrimSpace(key)]) {
			continue
		}
		lines = append(lines, line)
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		lines = append(lines, key+"="+values[key])
	}
	next := strings.TrimSpace(strings.Join(lines, "\n"))
	if len([]rune(next)) > 32000 {
		return ProjectEnvironmentResult{}, fmt.Errorf("environment exceeds 32,000 characters")
	}
	payload := map[string]any{"environment": next, "expected_updated_at": p["updated_at"]}
	for _, key := range []string{"name", "repo_url", "default_branch"} {
		payload[key] = p[key]
	}
	for _, key := range []string{"goal", "instructions", "deploy_branch"} {
		value, _ := settings[key].(string)
		payload[key] = value
	}
	if value, ok := settings["auto_deploy"].(bool); ok {
		payload["auto_deploy"] = value
	}
	resp, err := c.doRequest(ctx, http.MethodPatch, "/api/v1/projects/"+url.PathEscape(id)+"/settings", payload)
	if err != nil {
		return ProjectEnvironmentResult{}, fmt.Errorf("could not save project environment")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return ProjectEnvironmentResult{}, fmt.Errorf("project changed elsewhere; retry after reviewing current settings")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ProjectEnvironmentResult{}, fmt.Errorf("save project environment failed (HTTP %d); no response body displayed to protect secrets", resp.StatusCode)
	}
	var saved map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&saved); err != nil {
		return ProjectEnvironmentResult{}, fmt.Errorf("could not decode saved project settings; read back before retrying")
	}
	actual := environmentResult(saved, true)
	for key, value := range values {
		if got, ok := actual.Values[key]; !ok || got != value {
			return ProjectEnvironmentResult{}, fmt.Errorf("saved environment verification failed")
		}
	}
	for _, key := range unset {
		if _, ok := actual.Values[key]; ok {
			return ProjectEnvironmentResult{}, fmt.Errorf("removed environment key still present")
		}
	}
	return environmentResult(saved, false), nil
}

func environmentFromProcess(names []string, values map[string]string) error {
	for _, name := range names {
		if !environmentKey.MatchString(name) {
			return fmt.Errorf("invalid environment variable name")
		}
		value, ok := os.LookupEnv(name)
		if !ok {
			return fmt.Errorf("requested variable %s is not present in the process environment", name)
		}
		if _, exists := values[name]; exists {
			return fmt.Errorf("duplicate environment variable %s", name)
		}
		values[name] = value
	}
	return nil
}

func updateEnvironmentMCP(ctx context.Context, args map[string]any) (ProjectEnvironmentResult, error) {
	id, _ := args["project_id"].(string)
	values := map[string]string{}
	if raw, exists := args["values"]; exists {
		entries, ok := raw.(map[string]any)
		if !ok {
			return ProjectEnvironmentResult{}, fmt.Errorf("values must be an object of strings")
		}
		for key, rawValue := range entries {
			value, ok := rawValue.(string)
			if !ok {
				return ProjectEnvironmentResult{}, fmt.Errorf("environment values must be strings")
			}
			values[key] = value
		}
	}
	lists := map[string][]string{}
	for _, key := range []string{"from_env", "unset"} {
		if raw, exists := args[key]; exists {
			entries, ok := raw.([]any)
			if !ok {
				return ProjectEnvironmentResult{}, fmt.Errorf("%s must be an array of variable names", key)
			}
			for _, entry := range entries {
				name, ok := entry.(string)
				if !ok || !environmentKey.MatchString(name) {
					return ProjectEnvironmentResult{}, fmt.Errorf("invalid environment variable name")
				}
				lists[key] = append(lists[key], name)
			}
		}
	}
	if err := environmentFromProcess(lists["from_env"], values); err != nil {
		return ProjectEnvironmentResult{}, err
	}
	return GetCuanClient().UpdateProjectEnvironment(ctx, id, values, lists["unset"])
}

// General project inspection must not disclose environment values incidentally.
func redactProjectEnvironment(p map[string]any) {
	metadata, _ := p["metadata"].(map[string]any)
	settings, _ := metadata["cuan_settings"].(map[string]any)
	if _, exists := settings["environment"]; exists {
		settings["environment"] = "[REDACTED: use get_project_environment]"
	}
}

func redactProjectEnvironments(projects []map[string]any) []map[string]any {
	for _, p := range projects {
		redactProjectEnvironment(p)
	}
	return projects
}
