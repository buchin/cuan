package terminal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type gitAskpassConfig struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	if existing, err := os.ReadFile(path); err == nil && string(existing) == string(data) {
		_ = os.Chmod(path, perm)
		return nil
	}
	tmpPath := fmt.Sprintf("%s.%d.tmp", path, time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

// EnsureGitAskpassScript writes a lightweight executable wrapper and optional 0600 config file
// in the temp directory that delegates Git credential prompts to `<binaryPath> git-askpass`.
// Passing credentials via a 0600 config file ensures AI CLIs (agy/codex/claude) that scrub *_TOKEN
// environment variables from shell subcommands do not strip the askpass endpoint token.
func EnsureGitAskpassScript(binaryPath string, opts ...string) (string, error) {
	if strings.TrimSpace(binaryPath) == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", err
		}
		binaryPath = exe
	}
	var projectID, apiURL, scopedToken string
	if len(opts) >= 3 {
		projectID, apiURL, scopedToken = opts[0], opts[1], opts[2]
	}

	suffix := fmt.Sprintf("%d", os.Getuid())
	if projectID != "" {
		safeID := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
				return r
			}
			return '_'
		}, projectID)
		suffix += "-" + safeID
	}

	scriptPath := filepath.Join(os.TempDir(), "cuan-git-askpass-"+suffix+".sh")
	quotedBin := "'" + strings.ReplaceAll(binaryPath, "'", "'\"'\"'") + "'"

	var desired string
	if apiURL != "" && scopedToken != "" {
		cfgPath := filepath.Join(os.TempDir(), "cuan-git-askpass-"+suffix+".json")
		cfgBytes, err := json.Marshal(gitAskpassConfig{URL: apiURL, Token: scopedToken})
		if err != nil {
			return "", err
		}
		if err := writeFileAtomic(cfgPath, cfgBytes, 0o600); err != nil {
			return "", err
		}
		quotedCfg := "'" + strings.ReplaceAll(cfgPath, "'", "'\"'\"'") + "'"
		desired = "#!/bin/sh\nexec " + quotedBin + " git-askpass --config " + quotedCfg + " \"$@\"\n"
	} else {
		desired = "#!/bin/sh\nexec " + quotedBin + " git-askpass \"$@\"\n"
	}

	if err := writeFileAtomic(scriptPath, []byte(desired), 0o700); err != nil {
		return "", err
	}
	return scriptPath, nil
}

// RunGitAskpassCLI implements `cuan git-askpass [--config PATH] <prompt>` for GIT_ASKPASS in Project Terminal.
func RunGitAskpassCLI(args []string) int {
	var cfgPath string
	var rest []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--config" && i+1 < len(args) {
			cfgPath = args[i+1]
			i++
			continue
		}
		rest = append(rest, args[i])
	}

	prompt := ""
	if len(rest) > 0 {
		prompt = strings.ToLower(strings.TrimSpace(rest[0]))
	}
	if strings.HasPrefix(prompt, "username") {
		fmt.Println("x-access-token")
		return 0
	}

	apiURL := strings.TrimSpace(os.Getenv("CUAN_GIT_ASKPASS_URL"))
	token := strings.TrimSpace(os.Getenv("CUAN_GIT_ASKPASS_TOKEN"))
	if cfgPath != "" {
		if raw, err := os.ReadFile(cfgPath); err == nil {
			var cfg gitAskpassConfig
			if json.Unmarshal(raw, &cfg) == nil {
				if strings.TrimSpace(cfg.URL) != "" {
					apiURL = strings.TrimSpace(cfg.URL)
				}
				if strings.TrimSpace(cfg.Token) != "" {
					token = strings.TrimSpace(cfg.Token)
				}
			}
		}
	}
	if apiURL == "" || token == "" {
		fmt.Fprintln(os.Stderr, "cuan git-askpass: terminal git credentials are not configured")
		return 1
	}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cuan git-askpass: %v\n", err)
		return 1
	}
	req.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cuan git-askpass: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "cuan git-askpass: server returned status %d\n", resp.StatusCode)
		return 1
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil || strings.TrimSpace(payload.Token) == "" {
		fmt.Fprintln(os.Stderr, "cuan git-askpass: missing repository token in server response")
		return 1
	}
	fmt.Println(payload.Token)
	return 0
}
