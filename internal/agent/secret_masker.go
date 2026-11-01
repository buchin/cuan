package agent

import (
	"github.com/buchin/agentrunner"
)

// MaskSecrets replaces occurrences of secret values and known credential patterns with [REDACTED].
// Ignores empty or short strings (less than 4 chars) to prevent over-masking common words.
func MaskSecrets(input string, secrets []string) string {
	return agentrunner.MaskSecrets(input, secrets)
}

// SanitizeEnv returns a slice of environment variables in KEY=VALUE format,
// ensuring system secrets are not exposed to untrusted logging.
func SanitizeEnv(env map[string]string, secrets []string) []string {
	return agentrunner.SanitizeEnv(env, secrets)
}
