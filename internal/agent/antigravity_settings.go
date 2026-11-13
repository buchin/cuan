package agent

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var cliSettingsMu sync.Mutex

var defaultAllowedPermissions = []string{
	"read_file(*)",
	"read_url(*)",
	"mcp(*)",
	"mcp(cuan-inspect/*)",
	"mcp(cuan-inspect:*)",
	"mcp(cuan-platform-mcp/*)",
	"mcp(cuan-platform-mcp:*)",
	"mcp(cuan-inspect/inspect_git_status)",
	"mcp(cuan-inspect/inspect_git_log)",
	"mcp(cuan-inspect/inspect_git_diff)",
	"mcp(cuan-inspect/inspect_git_branches)",
	"mcp(cuan-inspect/inspect_system_health)",
	"mcp(cuan-inspect/inspect_typecheck)",
	"mcp(cuan-inspect/list_projects)",
	"mcp(cuan-inspect/get_project)",
	"mcp(cuan-inspect/list_tasks)",
	"mcp(cuan-inspect/get_task)",
	"mcp(cuan-inspect/create_task_draft)",
	"mcp(cuan-inspect/update_task_status)",
	"mcp(cuan-inspect/list_inbox_conversations)",
	"mcp(cuan-inspect/get_inbox_messages)",
	"mcp(cuan-inspect/create_inbox_conversation)",
	"mcp(cuan-inspect/send_inbox_message)",
	"mcp(cuan-inspect/list_contacts)",
	"mcp(cuan-inspect/get_contact)",
	"mcp(cuan-inspect/list_scheduled_jobs)",
	"mcp(cuan-inspect/get_system_settings)",
	"mcp(cuan-inspect/inspect_database_query)",
	"mcp(cuan-inspect/inspect_database_schema)",
	"mcp(cuan-inspect/inspect_app_logs)",
	"mcp(cuan-inspect/list_apps)",
	"mcp(cuan-inspect/list_skills)",
	"mcp(cuan-inspect/toggle_agent_skill)",
	"mcp(cuan-inspect/probe_endpoint_health)",
	"mcp(cuan-inspect/inspect_deployment_status)",
	"mcp(cuan-inspect/web_fetch)",
	"mcp(cuan-platform-mcp/inspect_git_status)",
	"mcp(cuan-platform-mcp/inspect_git_log)",
	"mcp(cuan-platform-mcp/inspect_git_diff)",
	"mcp(cuan-platform-mcp/inspect_git_branches)",
	"mcp(cuan-platform-mcp/inspect_system_health)",
	"mcp(cuan-platform-mcp/inspect_typecheck)",
	"mcp(cuan-platform-mcp/list_projects)",
	"mcp(cuan-platform-mcp/get_project)",
	"mcp(cuan-platform-mcp/list_tasks)",
	"mcp(cuan-platform-mcp/get_task)",
	"mcp(cuan-platform-mcp/create_task_draft)",
	"mcp(cuan-platform-mcp/update_task_status)",
	"mcp(cuan-platform-mcp/list_inbox_conversations)",
	"mcp(cuan-platform-mcp/get_inbox_messages)",
	"mcp(cuan-platform-mcp/create_inbox_conversation)",
	"mcp(cuan-platform-mcp/send_inbox_message)",
	"mcp(cuan-platform-mcp/list_contacts)",
	"mcp(cuan-platform-mcp/get_contact)",
	"mcp(cuan-platform-mcp/list_scheduled_jobs)",
	"mcp(cuan-platform-mcp/get_system_settings)",
	"mcp(cuan-platform-mcp/inspect_database_query)",
	"mcp(cuan-platform-mcp/inspect_database_schema)",
	"mcp(cuan-platform-mcp/inspect_app_logs)",
	"mcp(cuan-platform-mcp/list_apps)",
	"mcp(cuan-platform-mcp/list_skills)",
	"mcp(cuan-platform-mcp/toggle_agent_skill)",
	"mcp(cuan-platform-mcp/probe_endpoint_health)",
	"mcp(cuan-platform-mcp/inspect_deployment_status)",
	"mcp(cuan-platform-mcp/web_fetch)",
	"view_file(*)",
	"list_dir(*)",
	"grep_search(*)",
	"find_by_name(*)",
	"view_file_outline(*)",
	"view_code_item(*)",
	"inspect_git_status(*)",
	"inspect_git_log(*)",
	"inspect_git_diff(*)",
	"inspect_git_branches(*)",
	"inspect_system_health(*)",
	"inspect_typecheck(*)",
	"search_web(*)",
	"web_fetch(*)",
	"list_projects(*)",
	"get_project(*)",
	"list_tasks(*)",
	"get_task(*)",
	"create_task_draft(*)",
	"list_inbox_conversations(*)",
	"get_inbox_messages(*)",
	"list_contacts(*)",
	"get_contact(*)",
	"list_scheduled_jobs(*)",
	"get_system_settings(*)",
	"inspect_database_query(*)",
	"inspect_database_schema(*)",
	"inspect_app_logs(*)",
	"probe_endpoint_health(*)",
	"inspect_deployment_status(*)",
	"call_mcp_tool(*)",
	"cuan-inspect/*",
	"cuan-inspect:*",
	"cuan-platform-mcp/*",
	"cuan-platform-mcp:*",
}

func applyDefaultReadOnlyPermissions(settings map[string]json.RawMessage) bool {
	perms := map[string]json.RawMessage{}
	if raw, ok := settings["permissions"]; ok {
		if json.Unmarshal(raw, &perms) != nil || perms == nil {
			return false
		}
	}
	var allow []string
	if raw := perms["allow"]; len(raw) > 0 && json.Unmarshal(raw, &allow) != nil {
		return false
	}
	allowSet := make(map[string]bool)
	for _, rule := range allow {
		allowSet[rule] = true
	}
	permsChanged := false
	for _, rule := range defaultAllowedPermissions {
		if !allowSet[rule] {
			allow = append(allow, rule)
			allowSet[rule] = true
			permsChanged = true
		}
	}
	if permsChanged || settings["permissions"] == nil {
		perms["allow"], _ = json.Marshal(allow)
		if data, err := json.Marshal(perms); err == nil {
			settings["permissions"] = data
			return true
		}
	}
	return false
}

// Antigravity's documented permission engine gives deny precedence over allow.
// Restrict agent file tools without hiding the profile from the provider process
// itself, which still needs native authentication and session state.
// https://antigravity.google/docs/permissions/#cli-fine-grained-permissions
func applyChatProfileReadDeny(settings map[string]json.RawMessage, home string) (bool, error) {
	home, err := filepath.Abs(home)
	if err != nil || strings.ContainsAny(home, "()\r\n") {
		return false, errors.New("CLI profile permission path needs review")
	}
	permissions := map[string]json.RawMessage{}
	if raw := settings["permissions"]; len(raw) > 0 {
		if json.Unmarshal(raw, &permissions) != nil || permissions == nil {
			return false, errors.New("CLI permissions could not be read; existing settings were preserved")
		}
	}
	var denied []string
	for _, field := range []string{"allow", "ask", "deny"} {
		var rules []string
		if raw := permissions[field]; len(raw) > 0 && json.Unmarshal(raw, &rules) != nil {
			return false, errors.New("CLI permission rules could not be read; existing settings were preserved")
		}
		if field == "deny" {
			denied = rules
		}
	}
	seen := map[string]bool{}
	for _, rule := range denied {
		seen[rule] = true
	}
	changed := false
	for _, dir := range []string{".gemini", ".cuan", ".ssh", ".claude", ".codex"} {
		rule := "read_file(" + filepath.Join(home, dir) + ")"
		if !seen[rule] {
			denied = append(denied, rule)
			seen[rule] = true
			changed = true
		}
	}
	if !changed {
		return false, nil
	}
	permissions["deny"], _ = json.Marshal(denied)
	settings["permissions"], _ = json.Marshal(permissions)
	return true, nil
}

// prepareAntigravitySettings uses the provider's documented settings.json keys
// before first sign-in. It never records acceptance of provider terms or opts in
// to interaction sharing. Existing explicit preferences remain unchanged.
// If trustedWorkspaces are provided, they are registered to avoid interactive trust prompts.
// Reference: https://antigravity.google/docs/cli/reference#configuration-keys-settingsjson
func prepareAntigravitySettings(home string, trustedWorkspaces ...string) error {
	cliSettingsMu.Lock()
	defer cliSettingsMu.Unlock()
	dir := AntigravityConfigDirPath(home)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	dirLstat, err := os.Lstat(dir)
	if err != nil || !dirLstat.IsDir() || dirLstat.Mode()&os.ModeSymlink != 0 {
		return errors.New("CLI settings directory needs review; no settings were changed")
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return errors.New("CLI settings directory needs review; no settings were changed")
	}
	parentResolved, err := filepath.EvalSymlinks(filepath.Dir(dir))
	if err != nil || resolved != filepath.Join(parentResolved, filepath.Base(dir)) {
		return errors.New("CLI settings directory needs review; no settings were changed")
	}
	path := filepath.Join(dir, "settings.json")
	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	settings := map[string]json.RawMessage{}
	if err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("CLI settings need review; existing settings were preserved")
		}
		data, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		if json.Unmarshal(data, &settings) != nil || settings == nil {
			return errors.New("CLI settings could not be read; existing settings were preserved")
		}
	}
	changed := false
	for key, value := range map[string]json.RawMessage{"colorScheme": json.RawMessage(`"terminal"`), "enableTelemetry": json.RawMessage(`false`)} {
		if _, exists := settings[key]; !exists {
			settings[key] = value
			changed = true
		}
	}
	if len(trustedWorkspaces) > 0 {
		var currentWorkspaces []string
		if raw, ok := settings["trustedWorkspaces"]; ok {
			_ = json.Unmarshal(raw, &currentWorkspaces)
		}
		trustedSet := make(map[string]bool)
		for _, w := range currentWorkspaces {
			trustedSet[w] = true
		}
		workspacesChanged := false
		for _, w := range trustedWorkspaces {
			w = strings.TrimSpace(w)
			if w != "" && !trustedSet[w] {
				currentWorkspaces = append(currentWorkspaces, w)
				trustedSet[w] = true
				workspacesChanged = true
			}
		}
		if workspacesChanged {
			data, err := json.Marshal(currentWorkspaces)
			if err == nil {
				settings["trustedWorkspaces"] = data
				changed = true
			}
		}
		if applyDefaultReadOnlyPermissions(settings) {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return writeAntigravitySettingsFile(dir, path, settings)
}

func writeAntigravitySettingsFile(dir, path string, settings map[string]json.RawMessage) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".settings-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(append(data, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

// PrepareAntigravityReadOnlyPermissions ensures read-only file/search tools are allowed in settings.json
// to avoid interactive permission prompts or auto-denials in headless mode.
func PrepareAntigravityReadOnlyPermissions(home string) error {
	_ = EnsureAntigravityMCPConfig(home)
	cliSettingsMu.Lock()
	defer cliSettingsMu.Unlock()
	dir := AntigravityConfigDirPath(home)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	dirLstat, err := os.Lstat(dir)
	if err != nil || !dirLstat.IsDir() || dirLstat.Mode()&os.ModeSymlink != 0 {
		return errors.New("CLI settings directory needs review; no settings were changed")
	}
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return errors.New("CLI settings directory needs review; no settings were changed")
	}
	parentResolved, err := filepath.EvalSymlinks(filepath.Dir(dir))
	if err != nil || resolved != filepath.Join(parentResolved, filepath.Base(dir)) {
		return errors.New("CLI settings directory needs review; no settings were changed")
	}
	path := filepath.Join(dir, "settings.json")
	info, err := os.Lstat(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	settings := map[string]json.RawMessage{}
	if err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("CLI settings need review; existing settings were preserved")
		}
		data, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		if json.Unmarshal(data, &settings) != nil || settings == nil {
			return errors.New("CLI settings could not be read; existing settings were preserved")
		}
	}
	changed, err := applyChatProfileReadDeny(settings, home)
	if err != nil {
		return err
	}
	for key, value := range map[string]json.RawMessage{"colorScheme": json.RawMessage(`"terminal"`), "enableTelemetry": json.RawMessage(`false`)} {
		if _, exists := settings[key]; !exists {
			settings[key] = value
			changed = true
		}
	}
	if applyDefaultReadOnlyPermissions(settings) {
		changed = true
	}
	if !changed {
		return nil
	}
	return writeAntigravitySettingsFile(dir, path, settings)
}

// PrepareAntigravitySettings configures settings.json defaults and trusts the given workspaces.
func PrepareAntigravitySettings(home string, trustedWorkspaces ...string) error {
	_ = EnsureAntigravityMCPConfig(home)
	return prepareAntigravitySettings(home, trustedWorkspaces...)
}

// EnsureAntigravityMCPConfig writes the cuan-inspect MCP server config to mcp_config.json
func EnsureAntigravityMCPConfig(home string) error {
	cliSettingsMu.Lock()
	defer cliSettingsMu.Unlock()
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if home == "" {
		return nil
	}
	for _, dir := range []string{
		filepath.Join(home, ".gemini", "config"),
		AntigravityConfigDirPath(home),
		filepath.Join(home, ".config", "antigravity"),
	} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			continue
		}
		path := filepath.Join(dir, "mcp_config.json")
		var cfg struct {
			MCPServers map[string]any `json:"mcpServers"`
		}
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			_ = json.Unmarshal(data, &cfg)
		}
		if cfg.MCPServers == nil {
			cfg.MCPServers = make(map[string]any)
		}
		cfg.MCPServers["cuan-inspect"] = map[string]any{
			"command": "cuan",
			"args":    []string{"mcp"},
		}
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err == nil {
			_ = os.WriteFile(path, append(data, '\n'), 0600)
		}
	}
	return nil
}

// EnsureWorkspaceMCPConfig configures mcp_config.json in the project workspace directory
// so that workspace-scoped MCP tool discovery in Antigravity automatically detects cuan-inspect.
func EnsureWorkspaceMCPConfig(workspaceDir string) error {
	workspaceDir = strings.TrimSpace(workspaceDir)
	if workspaceDir == "" {
		return nil
	}
	cliSettingsMu.Lock()
	defer cliSettingsMu.Unlock()
	for _, sub := range []string{
		filepath.Join(workspaceDir, ".gemini", "config"),
		filepath.Join(workspaceDir, ".agents"),
	} {
		if err := os.MkdirAll(sub, 0755); err != nil {
			continue
		}
		path := filepath.Join(sub, "mcp_config.json")
		var cfg struct {
			MCPServers map[string]any `json:"mcpServers"`
		}
		if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
			_ = json.Unmarshal(data, &cfg)
		}
		if cfg.MCPServers == nil {
			cfg.MCPServers = make(map[string]any)
		}
		cfg.MCPServers["cuan-inspect"] = map[string]any{
			"command": "cuan",
			"args":    []string{"mcp"},
		}
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err == nil {
			_ = os.WriteFile(path, append(data, '\n'), 0644)
		}
	}
	return nil
}

// GeneratedWorkspaceMCPConfig is the reconstructible default Cuan writes in a
// fresh workspace. Additional provider/user configuration is never disposable.
func GeneratedWorkspaceMCPConfig() []byte {
	data, _ := json.MarshalIndent(map[string]any{"mcpServers": map[string]any{"cuan-inspect": map[string]any{"command": "cuan", "args": []string{"mcp"}}}}, "", "  ")
	return append(data, '\n')
}
