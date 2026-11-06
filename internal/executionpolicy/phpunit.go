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
	if raw == "" {
		return nil, nil
	}
	var targets []string
	if len(raw) > 4096 || json.Unmarshal([]byte(raw), &targets) != nil || len(targets) == 0 || len(targets) > 16 {
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
