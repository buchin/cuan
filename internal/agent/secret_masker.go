package agent

import (
	"encoding/json"
	"regexp"
	"strings"
)

var (
	githubPATRegex         = regexp.MustCompile(`(ghp_[a-zA-Z0-9]{20,}|github_pat_[a-zA-Z0-9_]{20,})`)
	bearerRegex            = regexp.MustCompile(`(?i)(Bearer\s+)[a-zA-Z0-9_\-\.]{12,}`)
	dbURIRegex             = regexp.MustCompile(`(postgres(?:ql)?:\/\/[^:]+:)([^@]+)(@)`)
	passwordRegex          = regexp.MustCompile(`(?i)(password|secret|token|api_key)\s*[:=]\s*["']?([^\s"']{6,})["']?`)
	apiKeyRegex            = regexp.MustCompile(`(sk-[a-zA-Z0-9_\-]{15,}|AIzaSy[a-zA-Z0-9_\-]{30,}|xai-[a-zA-Z0-9_\-]{20,}|gsk_[a-zA-Z0-9_\-]{20,})`)
	labeledCredentialRegex = regexp.MustCompile(`(?im)^([ \t]*(?:[-*+]\s*)?(?:\*\*)?(?:kredensial|credentials?|password|kata sandi|ssh password|root password)[^:\r\n]{0,40}[:=])[^\r\n]*`)
	// SSH host keys and private keys are credentials: they pin trust for release
	// publication and must never be copied into stored task or chat text. The
	// key material stops at a quote so masking cannot consume JSON structure.
	sshKeyRegex = regexp.MustCompile(`(?i)((?:ssh-rsa|ssh-ed25519|ecdsa-sha2-nistp\d+|ssh-dss)[\s=:]+)[A-Za-z0-9+/=]{20,}(?:[ \t]+[^\s"']+)?`)
	// Environment assignments carry secrets without a password-like label, for
	// example CUAN_AUTH_SECRET=..., DOJO_*_PASSWORD=..., *_TOKEN=... . Only the
	// value is replaced so surrounding diagnostics stay readable.
	envSecretRegex = regexp.MustCompile(`(?m)\b([A-Z][A-Z0-9_]*(?:PASSWORD|PASSWD|SECRET|TOKEN|APIKEY|API_KEY|PRIVATE_KEY|HOST_KEY|CREDENTIAL)[A-Z0-9_]*[ \t]*=[ \t]*)(?:"[^"\n]*"|'[^'\n]*'|[^\s"'\n]+)`)
)

// MaskSecrets replaces occurrences of secret values and known credential patterns with [REDACTED].
// When input is or contains a JSON document (such as task guidance snapshots), it preserves
// JSON structural integrity by masking values within parsed JSON rather than mangling syntax.
func MaskSecrets(input string, secrets []string) string {
	if input == "" {
		return ""
	}

	trimmed := strings.TrimSpace(input)
	if (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")) {
		if json.Valid([]byte(trimmed)) {
			return maskJSON([]byte(trimmed), secrets)
		}
	}

	// Check if input contains fenced cuan-project-context block
	const contextMarker = "```cuan-project-context\n"
	if idx := strings.Index(input, contextMarker); idx >= 0 {
		fenceStart := idx + len(contextMarker)
		if endIdx := strings.Index(input[fenceStart:], "```"); endIdx >= 0 {
			fenceEnd := fenceStart + endIdx
			prefix := input[:idx]
			body := strings.TrimSpace(input[fenceStart:fenceEnd])
			suffix := input[fenceEnd:]

			maskedPrefix := maskPlainText(prefix, secrets)
			maskedBody := body
			if json.Valid([]byte(body)) {
				maskedBody = maskJSON([]byte(body), secrets)
			} else {
				maskedBody = maskPlainText(body, secrets)
			}
			maskedSuffix := MaskSecrets(suffix, secrets)

			return maskedPrefix + contextMarker + maskedBody + "\n" + maskedSuffix
		}
	}

	return maskPlainText(input, secrets)
}

func maskJSON(raw []byte, secrets []string) string {
	var val any
	if err := json.Unmarshal(raw, &val); err != nil {
		return maskPlainText(string(raw), secrets)
	}
	val = maskJSONNode(val, secrets)
	marshaled, err := json.Marshal(val)
	if err != nil {
		return maskPlainText(string(raw), secrets)
	}
	return string(marshaled)
}

func maskJSONNode(node any, secrets []string) any {
	switch v := node.(type) {
	case string:
		return maskPlainText(v, secrets)
	case map[string]any:
		res := make(map[string]any, len(v))
		for k, val := range v {
			res[k] = maskJSONNode(val, secrets)
		}
		return res
	case []any:
		res := make([]any, len(v))
		for i, val := range v {
			res[i] = maskJSONNode(val, secrets)
		}
		return res
	default:
		return node
	}
}

func maskPlainText(input string, secrets []string) string {
	if input == "" {
		return ""
	}

	result := input

	// 1. Mask explicit secret values
	for _, sec := range secrets {
		cleaned := strings.TrimSpace(sec)
		if len(cleaned) < 4 {
			continue
		}
		result = strings.ReplaceAll(result, cleaned, "[REDACTED]")
	}

	// 2. Mask known credential patterns
	result = githubPATRegex.ReplaceAllString(result, "[REDACTED]")
	result = apiKeyRegex.ReplaceAllString(result, "[REDACTED]")
	result = bearerRegex.ReplaceAllString(result, "${1}[REDACTED]")
	result = dbURIRegex.ReplaceAllString(result, "${1}[REDACTED]${3}")
	result = passwordRegex.ReplaceAllString(result, "${1}=[REDACTED]")
	result = labeledCredentialRegex.ReplaceAllString(result, "${1} [REDACTED]")
	result = sshKeyRegex.ReplaceAllString(result, "${1}[REDACTED]")
	result = envSecretRegex.ReplaceAllString(result, "${1}[REDACTED]")

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
