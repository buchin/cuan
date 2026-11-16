package executionpolicy

import (
	"encoding/json"
	"errors"
	"path"
	"strings"
)

// PHPUnitTargets is explicit project configuration, never an agent command.
// Only bounded relative test paths are accepted; options and traversals fail.
func PHPUnitTargets(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	if len(trimmed) > 4096 {
		return nil, errors.New("invalid registered PHPUnit targets")
	}
	var targets []string
	if err := json.Unmarshal([]byte(trimmed), &targets); err != nil {
		// Fallback for flexible formatting: [tests/foo.php], 'tests/foo.php', or comma-separated
		content := trimmed
		if strings.HasPrefix(content, "[") && strings.HasSuffix(content, "]") {
			content = content[1 : len(content)-1]
		}
		for _, part := range strings.Split(content, ",") {
			t := strings.TrimSpace(part)
			t = strings.Trim(t, "\"'`")
			if t != "" {
				targets = append(targets, t)
			}
		}
	}
	if len(targets) == 0 || len(targets) > 16 {
		return nil, errors.New("invalid registered PHPUnit targets")
	}
	for _, target := range targets {
		if len(target) > 256 || path.Clean(target) != target || strings.ContainsAny(target, "\\\x00\r\n\t ") || strings.HasPrefix(target, "/") || strings.Contains(target, "..") || strings.HasPrefix(target, "-") {
			return nil, errors.New("invalid registered PHPUnit target")
		}
		valid := strings.HasPrefix(target, "tests/") || strings.HasPrefix(target, "packages/") && len(strings.Split(target, "/")) >= 3 && strings.Split(target, "/")[2] == "tests"
		if !valid {
			return nil, errors.New("registered PHPUnit target must be a test path")
		}
	}
	return targets, nil
}
