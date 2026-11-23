package mcp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var slugPattern = regexp.MustCompile(`[^a-zA-Z0-9_\-]+`)

func toSlug(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = slugPattern.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		s = fmt.Sprintf("skill_%d", time.Now().UnixNano()%1000000)
	}
	return s
}

// SetSkillsDir sets the directory path for loading and storing modular skills on disk.
func (r *Registry) SetSkillsDir(dir string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skillsDir = dir
}

// SkillsDir returns the configured disk directory for modular skills.
func (r *Registry) SkillsDir() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.skillsDir
}

// LoadDiskSkills reads all skills stored in skillsDir (e.g. .agents/skills) and registers them.
func (r *Registry) LoadDiskSkills() error {
	r.mu.RLock()
	dir := r.skillsDir
	r.mu.RUnlock()

	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillPath := filepath.Join(dir, entry.Name(), "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}

		meta, content := parseSkillMarkdown(string(data))
		name, _ := meta["name"].(string)
		if name == "" {
			name = entry.Name()
		}
		desc, _ := meta["description"].(string)
		if desc == "" {
			desc = fmt.Sprintf("Skill modular: %s", name)
		}
		cat, _ := meta["category"].(string)
		if cat == "" {
			cat = "custom"
		}
		icon, _ := meta["icon"].(string)
		if icon == "" {
			icon = "sparkles"
		}
		tools, _ := meta["tools"].([]string)

		skillID := entry.Name()
		now := time.Now()

		r.mu.Lock()
		r.skills[skillID] = &Skill{
			ID:          skillID,
			Name:        name,
			Description: desc,
			Category:    cat,
			Icon:        icon,
			Enabled:     true,
			Tools:       tools,
			Content:     content,
			IsCustom:    true,
			BoundAgents: []string{"agent_cuan", "agent_sm", "agent_dev", "agent_support"},
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		for _, agentID := range []string{"agent_cuan", "agent_sm", "agent_dev", "agent_support"} {
			if r.agentSkills[agentID] == nil {
				r.agentSkills[agentID] = make(map[string]bool)
			}
			r.agentSkills[agentID][skillID] = true
		}
		r.mu.Unlock()
	}

	return nil
}

func parseSkillMarkdown(raw string) (map[string]any, string) {
	meta := make(map[string]any)
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")

	if strings.HasPrefix(strings.TrimSpace(normalized), "---") {
		trimmed := strings.TrimSpace(normalized)
		first := strings.Index(trimmed, "---")
		second := strings.Index(trimmed[first+3:], "---")
		if second > 0 {
			fm := trimmed[first+3 : first+3+second]
			content := strings.TrimSpace(trimmed[first+3+second+3:])

			inToolsList := false
			var toolsList []string

			for _, line := range strings.Split(fm, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				if strings.HasPrefix(line, "-") && inToolsList {
					toolName := strings.TrimSpace(strings.TrimPrefix(line, "-"))
					if toolName != "" {
						toolsList = append(toolsList, toolName)
					}
					continue
				}
				inToolsList = false

				key, val, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				key = strings.ToLower(strings.TrimSpace(key))
				val = strings.Trim(strings.TrimSpace(val), "\"'")

				switch key {
				case "name":
					meta["name"] = val
				case "description":
					meta["description"] = val
				case "category":
					meta["category"] = val
				case "icon":
					meta["icon"] = val
				case "tools":
					if val == "" {
						inToolsList = true
					} else {
						meta["tools"] = []string{val}
					}
				}
			}
			if len(toolsList) > 0 {
				meta["tools"] = toolsList
			}
			return meta, content
		}
	}

	// No frontmatter: extract title from first heading if present
	lines := strings.Split(normalized, "\n")
	title := ""
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(l, "# "))
			break
		}
	}
	if title != "" {
		meta["name"] = title
	}
	return meta, normalized
}

// FetchSkillFromURL downloads markdown from a URL, resolving GitHub repo links to SKILL.md.
func FetchSkillFromURL(ctx context.Context, rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("invalid skill URL: must be http or https")
	}

	targetURLs := []string{rawURL}

	// Transform GitHub repository URL (e.g. https://github.com/owner/repo)
	if parsed.Host == "github.com" {
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) == 2 {
			// https://github.com/owner/repo -> try main then master
			targetURLs = []string{
				fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/SKILL.md", parts[0], parts[1]),
				fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/master/SKILL.md", parts[0], parts[1]),
			}
		} else if len(parts) >= 4 && parts[2] == "blob" {
			// https://github.com/owner/repo/blob/branch/path/to/SKILL.md
			branch := parts[3]
			subpath := strings.Join(parts[4:], "/")
			targetURLs = []string{
				fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", parts[0], parts[1], branch, subpath),
			}
		}
	}

	client := &http.Client{Timeout: 15 * time.Second}
	var lastErr error

	for _, u := range targetURLs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "Cuan-Agent-Skill-Installer/1.0")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			data, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024)) // 2MB max
			if err != nil {
				return "", err
			}
			return string(data), nil
		}
		lastErr = fmt.Errorf("HTTP status %d from %s", resp.StatusCode, u)
	}

	if lastErr != nil {
		return "", lastErr
	}
	return "", errors.New("could not fetch skill from URL")
}

// InstallSkill downloads and installs a modular skill from a web URL or GitHub repository.
func (r *Registry) InstallSkill(ctx context.Context, sourceURL string, boundAgents []string) (*Skill, error) {
	content, err := FetchSkillFromURL(ctx, sourceURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch skill from URL: %w", err)
	}

	meta, body := parseSkillMarkdown(content)
	name, _ := meta["name"].(string)
	if name == "" {
		name = filepath.Base(strings.TrimSuffix(sourceURL, ".git"))
		if name == "" || name == "." {
			name = "Custom Skill"
		}
	}
	desc, _ := meta["description"].(string)
	if desc == "" {
		desc = fmt.Sprintf("Skill modular terpasang dari %s", sourceURL)
	}
	cat, _ := meta["category"].(string)
	if cat == "" {
		cat = "custom"
	}
	icon, _ := meta["icon"].(string)
	if icon == "" {
		icon = "sparkles"
	}
	tools, _ := meta["tools"].([]string)

	slug := toSlug(name)
	skillID := slug

	// Persist to disk if skillsDir configured
	r.mu.RLock()
	dir := r.skillsDir
	r.mu.RUnlock()

	if dir != "" {
		targetDir := filepath.Join(dir, skillID)
		if err := os.MkdirAll(targetDir, 0755); err == nil {
			_ = os.WriteFile(filepath.Join(targetDir, "SKILL.md"), []byte(content), 0644)
		}
	}

	if len(boundAgents) == 0 {
		boundAgents = []string{"agent_cuan", "agent_sm", "agent_dev", "agent_support"}
	}

	now := time.Now()
	skill := &Skill{
		ID:          skillID,
		Name:        name,
		Description: desc,
		Category:    cat,
		Icon:        icon,
		Enabled:     true,
		Tools:       tools,
		Content:     body,
		SourceURL:   sourceURL,
		IsCustom:    true,
		BoundAgents: boundAgents,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	r.mu.Lock()
	r.skills[skillID] = skill
	for _, agentID := range boundAgents {
		if r.agentSkills[agentID] == nil {
			r.agentSkills[agentID] = make(map[string]bool)
		}
		r.agentSkills[agentID][skillID] = true
	}
	r.mu.Unlock()

	return skill, nil
}

// InstallSkillFromContent registers a custom skill directly from markdown content.
func (r *Registry) InstallSkillFromContent(ctx context.Context, id, name, description, category, content string, boundAgents []string) (*Skill, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("skill name is required")
	}
	if strings.TrimSpace(id) == "" {
		id = toSlug(name)
	}
	if strings.TrimSpace(description) == "" {
		description = fmt.Sprintf("Custom skill: %s", name)
	}
	if strings.TrimSpace(category) == "" {
		category = "custom"
	}
	if len(boundAgents) == 0 {
		boundAgents = []string{"agent_cuan", "agent_sm", "agent_dev", "agent_support"}
	}

	// Persist to disk
	r.mu.RLock()
	dir := r.skillsDir
	r.mu.RUnlock()

	if dir != "" {
		targetDir := filepath.Join(dir, id)
		if err := os.MkdirAll(targetDir, 0755); err == nil {
			formatted := fmt.Sprintf("---\nname: %s\ndescription: %s\ncategory: %s\n---\n\n%s", name, description, category, content)
			_ = os.WriteFile(filepath.Join(targetDir, "SKILL.md"), []byte(formatted), 0644)
		}
	}

	now := time.Now()
	skill := &Skill{
		ID:          id,
		Name:        name,
		Description: description,
		Category:    category,
		Icon:        "sparkles",
		Enabled:     true,
		Content:     content,
		IsCustom:    true,
		BoundAgents: boundAgents,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	r.mu.Lock()
	r.skills[id] = skill
	for _, agentID := range boundAgents {
		if r.agentSkills[agentID] == nil {
			r.agentSkills[agentID] = make(map[string]bool)
		}
		r.agentSkills[agentID][id] = true
	}
	r.mu.Unlock()

	return skill, nil
}

// UninstallSkill deletes a custom skill from registry and disk.
func (r *Registry) UninstallSkill(ctx context.Context, skillID string) error {
	r.mu.Lock()
	skill, ok := r.skills[skillID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("skill '%s' not found", skillID)
	}
	delete(r.skills, skillID)
	for agentID := range r.agentSkills {
		delete(r.agentSkills[agentID], skillID)
	}
	dir := r.skillsDir
	r.mu.Unlock()

	if dir != "" && skill.IsCustom {
		targetDir := filepath.Join(dir, skillID)
		_ = os.RemoveAll(targetDir)
	}

	return nil
}

// RegisterRemoteMCP connects to an external MCP server, discovers its tools, and wires them into the registry.
func (r *Registry) RegisterRemoteMCP(ctx context.Context, cfg ServerConfig) (*App, []ToolDefinition, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, nil, errors.New("base_url is required for remote MCP server")
	}
	if strings.TrimSpace(cfg.Name) == "" {
		cfg.Name = "Custom MCP Server"
	}
	if strings.TrimSpace(cfg.ID) == "" {
		cfg.ID = "mcp_" + toSlug(cfg.Name)
	}
	if cfg.Transport == "" {
		cfg.Transport = TransportHTTP
	}

	client := NewClient(cfg)
	tools, err := client.DiscoverTools(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to discover tools from MCP server %s: %w", cfg.BaseURL, err)
	}

	toolNames := make([]string, 0, len(tools))
	r.mu.Lock()
	for _, tool := range tools {
		tName := tool.Name
		toolNames = append(toolNames, tName)
		r.tools[tName] = tool
		r.handlers[tName] = func(execCtx context.Context, args map[string]any) (*ToolCallResponse, error) {
			return client.ExecuteTool(execCtx, tName, args)
		}
	}

	now := time.Now()
	app := &App{
		ID:           cfg.ID,
		Name:         cfg.Name,
		Description:  cfg.Description,
		Icon:         "plug",
		Category:     "connector",
		Status:       "connected",
		BaseURL:      cfg.BaseURL,
		Transport:    string(cfg.Transport),
		IsCustom:     true,
		ConnectedAt:  &now,
		Capabilities: toolNames,
	}
	if app.Description == "" {
		app.Description = fmt.Sprintf("Konektor MCP eksternal (%s) dengan %d tools aktif.", cfg.BaseURL, len(tools))
	}
	r.apps[cfg.ID] = app
	r.remoteClients[cfg.ID] = client

	// Companion Skill for agent prompt awareness
	companionSkillID := "skill_" + cfg.ID
	companionSkill := &Skill{
		ID:          companionSkillID,
		AppID:       cfg.ID,
		Name:        cfg.Name + " Tools",
		Description: fmt.Sprintf("Alat eksternal dan kapabilitas dari konektor %s (%d tools).", cfg.Name, len(tools)),
		Icon:        "plug",
		Category:    "automation",
		Enabled:     true,
		IsCustom:    true,
		Tools:       toolNames,
		BoundAgents: []string{"agent_cuan", "agent_sm", "agent_dev", "agent_support"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	r.skills[companionSkillID] = companionSkill
	for _, agentID := range []string{"agent_cuan", "agent_sm", "agent_dev", "agent_support"} {
		if r.agentSkills[agentID] == nil {
			r.agentSkills[agentID] = make(map[string]bool)
		}
		r.agentSkills[agentID][companionSkillID] = true
	}
	r.mu.Unlock()

	return app, tools, nil
}

// UnregisterRemoteMCP disconnects a remote MCP connector and unregisters its tools.
func (r *Registry) UnregisterRemoteMCP(ctx context.Context, appID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	app, ok := r.apps[appID]
	if !ok {
		return fmt.Errorf("app '%s' not found", appID)
	}

	for _, cap := range app.Capabilities {
		delete(r.tools, cap)
		delete(r.handlers, cap)
	}
	delete(r.apps, appID)
	delete(r.remoteClients, appID)

	companionSkillID := "skill_" + appID
	delete(r.skills, companionSkillID)
	for agentID := range r.agentSkills {
		delete(r.agentSkills[agentID], companionSkillID)
	}

	return nil
}
