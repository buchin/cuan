package mcp

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// DefaultToolTimeout defines the default bounded execution timeout for MCP tools.
const DefaultToolTimeout = 15 * time.Second

// SafeEnvironmentAllowlist defines the safe environment variable names that may be inherited.
var SafeEnvironmentAllowlist = map[string]bool{
	"PATH":   true,
	"HOME":   true,
	"USER":   true,
	"LANG":   true,
	"LC_ALL": true,
	"TZ":     true,
	"TMPDIR": true,
}

// CredentialMaskPattern matches secret and token tokens for redaction.
var CredentialMaskPattern = regexp.MustCompile(`(?i)(bearer\s+[a-zA-Z0-9_\-\.]{10,}|(?:token|secret|password|api[_\-]?key|private[_\-]?key)[\s:=]+['"]?[a-zA-Z0-9_\-\.]{8,}['"]?)`)

// ToolHandler defines an executable in-process tool function.
type ToolHandler func(ctx context.Context, args map[string]any) (*ToolCallResponse, error)

// SafeExecuteTool wraps tool execution with:
// 1. Bounded execution timeout.
// 2. Panic recovery to prevent process or session crashes.
// 3. Error recovery with structured responses.
// 4. Input argument sanitization and zero ambient credential leakage.
// 5. Output text secret masking.
func SafeExecuteTool(ctx context.Context, handler ToolHandler, args map[string]any, timeout time.Duration) (resp *ToolCallResponse, err error) {
	if timeout <= 0 {
		timeout = DefaultToolTimeout
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 1. Sanitize incoming arguments to ensure no ambient credentials are leaked
	cleanArgs := SanitizeArgs(args)

	// 2. Panic recovery guard
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("tool execution panic recovered: %v", r)
			resp = &ToolCallResponse{
				IsError: true,
				Content: []ToolContent{
					{
						Type: "text",
						Text: fmt.Sprintf("Error: internal tool execution encountered an unexpected panic: %v", r),
					},
				},
			}
		}
	}()

	// 3. Channel for bounded execution
	type execResult struct {
		resp *ToolCallResponse
		err  error
	}
	resultChan := make(chan execResult, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultChan <- execResult{
					resp: &ToolCallResponse{
						IsError: true,
						Content: []ToolContent{
							{
								Type: "text",
								Text: fmt.Sprintf("Error: tool panic in execution thread: %v", r),
							},
						},
					},
					err: fmt.Errorf("panic: %v", r),
				}
			}
		}()

		r, e := handler(execCtx, cleanArgs)
		resultChan <- execResult{resp: r, err: e}
	}()

	select {
	case <-execCtx.Done():
		if errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			return &ToolCallResponse{
				IsError: true,
				Content: []ToolContent{
					{
						Type: "text",
						Text: fmt.Sprintf("Error: tool execution timed out after %v", timeout),
					},
				},
			}, fmt.Errorf("tool execution timed out: %w", execCtx.Err())
		}
		return &ToolCallResponse{
			IsError: true,
			Content: []ToolContent{
				{
					Type: "text",
					Text: "Error: tool execution cancelled",
				},
			},
		}, execCtx.Err()

	case res := <-resultChan:
		if res.err != nil {
			if res.resp == nil {
				res.resp = &ToolCallResponse{
					IsError: true,
					Content: []ToolContent{
						{
							Type: "text",
							Text: fmt.Sprintf("Error: %v", res.err),
						},
					},
				}
			} else {
				res.resp.IsError = true
			}
		}

		// 4. Output masking to prevent credential leaks in results
		if res.resp != nil {
			for i := range res.resp.Content {
				res.resp.Content[i].Text = MaskSensitiveText(res.resp.Content[i].Text)
			}
		}

		return res.resp, res.err
	}
}

// IsAmbientCredentialKey returns true if the environment variable name looks like an ambient host/cloud secret.
func IsAmbientCredentialKey(key string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	prefixes := []string{
		"AWS_", "DOJO_", "FURAL_", "RESEND_", "NERD_", "TINYFISH_",
		"GITHUB_", "GH_", "GOOGLE_", "OPENAI_", "ANTHROPIC_", "GEMINI_",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(upper, p) {
			return true
		}
	}
	keywords := []string{
		"SECRET", "TOKEN", "PASSWORD", "PASSWD", "API_KEY", "APIKEY",
		"PRIVATE_KEY", "CREDENTIAL", "AUTH",
	}
	for _, kw := range keywords {
		if strings.Contains(upper, kw) {
			return true
		}
	}
	return false
}

// FilterSafeEnvironment removes any ambient credential variables from a list of KEY=VAL environment entries.
// Only safe, allowlisted variables are preserved, strictly adhering to docs/product/integration-principles.md.
func FilterSafeEnvironment(env []string) []string {
	safe := make([]string, 0, len(env))
	for _, entry := range env {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 0 {
			continue
		}
		key := parts[0]
		if SafeEnvironmentAllowlist[key] && !IsAmbientCredentialKey(key) {
			safe = append(safe, entry)
		}
	}
	return safe
}

// SanitizeArgs cleans tool input arguments, stripping any injected ambient credentials.
func SanitizeArgs(args map[string]any) map[string]any {
	if args == nil {
		return make(map[string]any)
	}
	clean := make(map[string]any, len(args))
	for k, v := range args {
		if IsAmbientCredentialKey(k) {
			continue
		}
		switch val := v.(type) {
		case string:
			clean[k] = MaskSensitiveText(val)
		case map[string]any:
			clean[k] = SanitizeArgs(val)
		default:
			clean[k] = val
		}
	}
	return clean
}

// MaskSensitiveText masks credentials and API tokens in text output.
func MaskSensitiveText(text string) string {
	if text == "" {
		return ""
	}
	return CredentialMaskPattern.ReplaceAllStringFunc(text, func(m string) string {
		if len(m) <= 6 {
			return "[REDACTED]"
		}
		return m[:3] + "..." + "[REDACTED]"
	})
}
