package diagnostics

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// RunConfigCLI handles `cuan config` subcommands.
func RunConfigCLI(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(out, `Cuan Config CLI — View and configure Cuan CLI & MCP connection settings

Usage:
  cuan config show
  cuan config set <key> <value>

Keys:
  api_url     Cuan server URL (e.g. https://cuan.dojo.cc)
  auth_token  Owner authentication token

Example:
  cuan config set api_url https://cuan.dojo.cc
  cuan config set auth_token YOUR_CUAN_TOKEN`)
		return 0
	}

	sub := strings.ToLower(strings.TrimSpace(args[0]))
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".cuan")
	configFile := filepath.Join(configDir, "config.json")

	switch sub {
	case "show", "get":
		cfg := GetCuanConfig()
		fmt.Fprintf(out, "Cuan Configuration (%s):\n", configFile)
		fmt.Fprintf(out, "  API URL:    %s\n", cfg.BaseURL)
		maskedToken := "(not set)"
		if cfg.AuthToken != "" {
			if len(cfg.AuthToken) > 10 {
				maskedToken = cfg.AuthToken[:6] + "..." + cfg.AuthToken[len(cfg.AuthToken)-4:]
			} else {
				maskedToken = "******"
			}
		}
		fmt.Fprintf(out, "  Auth Token: %s\n", maskedToken)
		return 0

	case "set":
		if err := taskSessionOwnerWriteError(); err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		if len(args) < 3 {
			fmt.Fprintln(errOut, "Usage: cuan config set <key> <value>")
			return 2
		}
		key := strings.ToLower(strings.TrimSpace(args[1]))
		val := strings.TrimSpace(args[2])

		_ = os.MkdirAll(configDir, 0700)
		data, _ := os.ReadFile(configFile)
		curr := map[string]string{}
		_ = json.Unmarshal(data, &curr)

		switch key {
		case "api_url", "url", "host":
			curr["api_url"] = strings.TrimRight(val, "/")
		case "auth_token", "token":
			curr["auth_token"] = val
		default:
			fmt.Fprintf(errOut, "Unknown config key: %s (allowed: api_url, auth_token)\n", key)
			return 2
		}

		outData, err := json.MarshalIndent(curr, "", "  ")
		if err != nil {
			fmt.Fprintf(errOut, "Error serializing config: %v\n", err)
			return 1
		}
		if err := os.WriteFile(configFile, outData, 0600); err != nil {
			fmt.Fprintf(errOut, "Error writing config to %s: %v\n", configFile, err)
			return 1
		}

		cachedCuanConfig = nil
		globalCuanClient = nil
		fmt.Fprintf(out, "Configuration updated: %s = %s\n", key, val)
		return 0

	default:
		fmt.Fprintf(errOut, "Unknown config subcommand: %s\n", sub)
		return 2
	}
}
