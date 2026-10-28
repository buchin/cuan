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
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil || resolved != dir {
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
		var perms struct {
			Allow []string `json:"allow"`
			Ask   []string `json:"ask,omitempty"`
			Deny  []string `json:"deny,omitempty"`
		}
		if raw, ok := settings["permissions"]; ok {
			_ = json.Unmarshal(raw, &perms)
		}
		readTools := []string{
			"read_file(*)",
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
			"call_mcp_tool(*)",
			"cuan-inspect/*",
			"cuan-inspect:*",
		}
		allowSet := make(map[string]bool)
		for _, rule := range perms.Allow {
			allowSet[rule] = true
		}
		permsChanged := false
		for _, rule := range readTools {
			if !allowSet[rule] {
				perms.Allow = append(perms.Allow, rule)
				allowSet[rule] = true
				permsChanged = true
			}
		}
		if permsChanged || settings["permissions"] == nil {
			if data, err := json.Marshal(perms); err == nil {
				settings["permissions"] = data
				changed = true
			}
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
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil || resolved != dir {
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
	var perms struct {
		Allow []string `json:"allow"`
		Ask   []string `json:"ask,omitempty"`
		Deny  []string `json:"deny,omitempty"`
	}
	if raw, ok := settings["permissions"]; ok {
		_ = json.Unmarshal(raw, &perms)
	}
	readTools := []string{
		"read_file(*)",
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
		"call_mcp_tool(*)",
		"cuan-inspect/*",
		"cuan-inspect:*",
	}
	allowSet := make(map[string]bool)
	for _, rule := range perms.Allow {
		allowSet[rule] = true
	}
	permsChanged := false
	for _, rule := range readTools {
		if !allowSet[rule] {
			perms.Allow = append(perms.Allow, rule)
			allowSet[rule] = true
			permsChanged = true
		}
	}
	if permsChanged || settings["permissions"] == nil {
		if data, err := json.Marshal(perms); err == nil {
			settings["permissions"] = data
			changed = true
		}
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
	for _, dir := range []string{
		filepath.Join(home, ".gemini", "config"),
		AntigravityConfigDirPath(home),
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
