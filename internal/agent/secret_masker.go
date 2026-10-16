package agent

import (
	"strings"
)

// MaskSecrets replaces occurrences of secret values with [REDACTED].
// Ignores empty or short strings (less than 4 chars) to prevent over-masking common words.
func MaskSecrets(input string, secrets []string) string {
	if input == "" {
		return ""
	}

	result := input
	for _, sec := range secrets {
		cleaned := strings.TrimSpace(sec)
		if len(cleaned) < 4 {
			continue
		}
		result = strings.ReplaceAll(result, cleaned, "[REDACTED]")
	}
	return result
}

// SanitizeEnv returns a slice of environment variables in KEY=VALUE format,
// ensuring system secrets are not exposed to untrusted logging.
func SanitizeEnv(env map[string]string, secrets []string) []string {
	var list []string
	for k, v := range env {
		safeVal := MaskSecrets(v, secrets)
		list = append(list, k+"="+safeVal)
	}
	return list
}
