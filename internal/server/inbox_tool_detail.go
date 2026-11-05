package server

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/buchin/cuan/internal/agent"
	"github.com/buchin/cuan/internal/terminal"
)

// Only explicit tool_info fields are eligible. Never capture text deltas,
// reasoning, native profile files or the format tool's completed reply.
type inboxToolDetailRecorder struct {
	secrets   []string
	workspace string
	steps     map[int]map[string]string
	private   map[int]bool
}

var inboxPrivateKeyBlock = regexp.MustCompile(`(?s)-----BEGIN [^-]*PRIVATE KEY-----.*?-----END [^-]*PRIVATE KEY-----`)
var inboxSensitiveFile = regexp.MustCompile(`(?i)(^|[/\\"\s])(?:\.env(?:[.\/\\"\s]|$)|\.(?:ssh|cuan|gemini|claude|codex|antigravity)[/\\]|credentials(?:[.\/\\"\s]|$)|(?:oauth[^/\\]*|auth|mcp_config)\.json)`)
var inboxCredentialLine = regexp.MustCompile(`(?im)^.*(?:password|passwd|secret|token|key|credential|authorization|cookie|database_url)[a-z0-9_]*\s*["']?\s*[:=].*$`)

func inboxSensitiveDetailKey(key string) bool {
	return terminal.IsSensitiveKey(key) || strings.EqualFold(key, "COMMAND_CODE_API_KEY") || strings.EqualFold(key, "cookie")
}

func newInboxToolDetailRecorder(workspace string) *inboxToolDetailRecorder {
	r := &inboxToolDetailRecorder{workspace: workspace, steps: map[int]map[string]string{}, private: map[int]bool{}}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok && inboxSensitiveDetailKey(key) && len(value) >= 4 {
			r.secrets = append(r.secrets, value)
		}
	}
	return r
}

func (r *inboxToolDetailRecorder) sanitize(value any) any {
	switch v := value.(type) {
	case map[string]any:
		// Providers also return environment rows as {name/key, value}.
		// The secret belongs to the labelled value, not just the field name.
		for label, item := range v {
			if !strings.EqualFold(label, "key") && !strings.EqualFold(label, "name") {
				continue
			}
			if name, ok := item.(string); ok && inboxSensitiveDetailKey(name) {
				for field := range v {
					if strings.EqualFold(field, "value") {
						v[field] = "[REDACTED]"
					}
				}
			}
		}
		for key, item := range v {
			if inboxSensitiveDetailKey(key) {
				v[key] = "[REDACTED]"
			} else {
				v[key] = r.sanitize(item)
			}
		}
	case []any:
		for i := range v {
			v[i] = r.sanitize(v[i])
		}
	case string:
		// Serialized JSON embedded in a tool string needs structural redaction
		// too; regex masking alone misses quoted credential keys.
		var nested any
		if len(v) < 65536 && json.Unmarshal([]byte(v), &nested) == nil {
			switch nested.(type) {
			case map[string]any, []any:
				encoded, _ := json.MarshalIndent(r.sanitize(nested), "", "  ")
				return string(encoded)
			}
		}
		v = inboxPrivateKeyBlock.ReplaceAllString(v, "[REDACTED PRIVATE KEY]")
		v = agent.MaskSecrets(v, r.secrets)
		v = inboxCredentialLine.ReplaceAllString(v, "[Credential-bearing line redacted]")
		v = taskOutputANSI.ReplaceAllString(v, "")
		if r.workspace != "" {
			v = strings.ReplaceAll(v, strings.TrimRight(r.workspace, "/")+"/", "")
		}
		return strings.Map(func(c rune) rune {
			if unicode.IsControl(c) && c != '\n' && c != '\t' {
				return -1
			}
			return c
		}, v)
	}
	return value
}

func (r *inboxToolDetailRecorder) observe(index int, tool string, info map[string]json.RawMessage) {
	if !isSafeInboxTool(tool) || tool == "finish" || strings.HasSuffix(tool, "/finish") || strings.HasSuffix(tool, ":finish") || len(info) == 0 {
		return
	}
	if r.steps[index] == nil {
		if len(r.steps) >= 24 {
			return
		}
		r.steps[index] = map[string]string{}
	}
	for _, section := range []struct {
		label string
		keys  []string
	}{
		{"Input", []string{"parameters", "arguments", "input"}},
		{"Output", []string{"output", "result"}},
		{"Error", []string{"error"}},
	} {
		for _, key := range section.keys {
			raw := info[key]
			if len(raw) == 0 || string(raw) == "null" {
				continue
			}
			if section.label == "Input" && inboxSensitiveFile.Match(raw) {
				r.private[index] = true
			}
			if len(raw) > 65536 {
				r.steps[index][section.label] = fmt.Sprintf("[Too large to retain: %d bytes]", len(raw))
				break
			}
			var value any
			if json.Unmarshal(raw, &value) != nil {
				continue
			}
			value = r.sanitize(value)
			text, ok := value.(string)
			if !ok {
				encoded, _ := json.MarshalIndent(value, "", "  ")
				text = string(encoded)
			}
			chars := []rune(strings.TrimSpace(text))
			if len(chars) > 2000 {
				text = string(chars[:2000]) + "\n[Excerpt truncated]"
			}
			r.steps[index][section.label] = text
			break
		}
	}
}

func (r *inboxToolDetailRecorder) body(index int) string {
	parts := []string{}
	for _, key := range []string{"Input", "Output", "Error"} {
		if text := r.steps[index][key]; text != "" {
			if r.private[index] && key != "Input" {
				text = "[Sensitive file contents not retained]"
			}
			parts = append(parts, key+"\n"+text)
		}
	}
	return strings.Join(parts, "\n\n")
}
