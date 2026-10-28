package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

const inboxRuntimeLineLimit = 1024 * 1024
const inboxRuntimeByteLimit = 16 * 1024 * 1024

var inboxRuntimeID = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

// Consume NDJSON incrementally. Arguments, output, text deltas and reasoning are
// intentionally absent from the decoded event model and never become trace text.
type inboxRuntimeStream struct {
	pending                                 []byte
	result                                  json.RawMessage
	bytes, events, traceCount, toolFailures int
	invalid, overflow, truncated            bool
	unsafeTool                              bool
	seen                                    map[string]bool
	step                                    func(string, string)
	maxToolCalls                            int
	disallowTools                           bool
	toolCalls                               int
	toolIndices                             map[int]bool
	toolBudgetExceeded                      bool
	cancel                                  func()
}

func (s *inboxRuntimeStream) emit(kind, text string) {
	if s.step == nil {
		return
	}
	if s.traceCount >= 64 {
		if !s.truncated {
			s.truncated = true
			s.step("runtime_trace_limited", "Activity detail limit reached. The final result will still be checked.")
		}
		return
	}
	s.traceCount++
	s.step(kind, text)
}
func isSafeInboxTool(tool string) bool {
	switch tool {
	case "read_file", "view_file", "view_file_outline", "view_code_item", "list_dir", "grep_search", "find_by_name", "finish",
		"inspect_git_status", "inspect_git_log", "inspect_git_diff", "inspect_git_branches", "inspect_system_health", "inspect_typecheck",
		"search_web",
		"list_projects", "get_project", "list_tasks", "get_task", "create_task_draft",
		"list_inbox_conversations", "get_inbox_messages", "list_contacts", "get_contact",
		"list_scheduled_jobs", "get_system_settings", "call_mcp_tool":
		return true
	default:
		return false
	}
}

func (s *inboxRuntimeStream) Write(p []byte) (int, error) {
	n := len(p)
	s.bytes += n
	if s.overflow {
		return n, nil
	}
	if s.bytes > inboxRuntimeByteLimit {
		s.overflow = true
		s.pending = nil
		return n, nil
	}
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		part := p
		if i >= 0 {
			part = p[:i]
		}
		if len(s.pending)+len(part) > inboxRuntimeLineLimit {
			s.overflow = true
			s.pending = nil
			return n, nil
		}
		s.pending = append(s.pending, part...)
		if i < 0 {
			break
		}
		s.consume(s.pending)
		if s.overflow {
			s.pending = nil
			return n, nil
		}
		s.pending = nil
		p = p[i+1:]
	}
	return n, nil
}
func (s *inboxRuntimeStream) consume(line []byte) {
	if len(bytes.TrimSpace(line)) == 0 {
		return
	}
	if len(s.result) > 0 {
		s.invalid = true
		return
	}
	s.events++
	if s.events > 10000 {
		s.overflow = true
		return
	}
	var e struct {
		Event          string          `json:"event"`
		ConversationID string          `json:"conversation_id"`
		Result         json.RawMessage `json:"result"`
		Update         struct {
			Index    *int     `json:"step_index"`
			State    string   `json:"state"`
			Type     string   `json:"step_type"`
			Tool     string   `json:"tool_name"`
			Duration *float64 `json:"duration_seconds"`
		} `json:"step_update"`
	}
	if json.Unmarshal(line, &e) != nil {
		s.invalid = true
		return
	}
	switch e.Event {
	case "init":
		s.emit("runtime_init", "Runtime session started.")
		if inboxRuntimeID.MatchString(e.ConversationID) {
			s.emit("runtime_session", "Runtime session ID: "+e.ConversationID)
		}
	case "result":
		if len(s.result) > 0 || len(e.Result) == 0 || string(e.Result) == "null" {
			s.invalid = true
			return
		}
		s.result = append([]byte(nil), e.Result...)
		var final struct {
			DeniedActions []json.RawMessage `json:"denied_actions"`
		}
		if json.Unmarshal(e.Result, &final) == nil && len(final.DeniedActions) > 0 {
			s.unsafeTool = true
			s.emit("runtime_permission_denied", "Tool permission denied by the runtime. Activity ending does not mean the command succeeded.")
		}
	case "step_update":
		// Native AGY also emits input/finish lifecycle steps. They are not
		// tools, and must not make a schema-only reply look like execution.
		if e.Update.Type == "user_input" || e.Update.Type == "finish" || e.Update.Type == "error_message" {
			if e.Update.Tool != "" {
				s.unsafeTool = true
			}
			return
		}
		// Unknown/malformed tool activity cannot authorize a replay on another account.
		if e.Update.Type != "agent_response" && e.Update.Type != "tool" {
			s.unsafeTool = true
		}
		if e.Update.Type == "tool" {
			if !isSafeInboxTool(e.Update.Tool) {
				s.unsafeTool = true
			}
		}
		u := e.Update
		if u.Index == nil || *u.Index < 0 || *u.Index > 1000000 {
			return
		}
		if u.Type != "tool" && u.Type != "agent_response" {
			return
		}
		if u.State != "ACTIVE" && u.State != "DONE" && u.State != "ERROR" {
			return
		}
		key := fmt.Sprintf("%s:%d:%s", u.Type, *u.Index, u.State)
		if s.seen == nil {
			s.seen = map[string]bool{}
		}
		if s.seen[key] {
			return
		}
		if len(s.seen) < 256 {
			s.seen[key] = true
		}
		duration := ""
		if u.Duration != nil && !math.IsNaN(*u.Duration) && !math.IsInf(*u.Duration, 0) && *u.Duration >= 0 && *u.Duration <= 86400 {
			duration = fmt.Sprintf(" (%.1f s)", *u.Duration)
		}
		if u.Type == "tool" {
			if !isSafeInboxTool(u.Tool) {
				s.unsafeTool = true
			}
			if u.Tool != "finish" && u.Index != nil {
				if s.disallowTools {
					s.toolBudgetExceeded = true
					s.emit("runtime_tool_budget_exceeded", "Tools are disallowed for this response stage. Finishing response.")
					if s.cancel != nil {
						s.cancel()
					}
					return
				}
				if s.toolIndices == nil {
					s.toolIndices = make(map[int]bool)
				}
				if !s.toolIndices[*u.Index] {
					s.toolIndices[*u.Index] = true
					s.toolCalls++
					if s.maxToolCalls > 0 && s.toolCalls > s.maxToolCalls {
						s.toolBudgetExceeded = true
						s.emit("runtime_tool_budget_exceeded", fmt.Sprintf("Tool limit of %d calls reached for coordinator chat. Finishing response.", s.maxToolCalls))
						if s.cancel != nil {
							s.cancel()
						}
					}
				}
			}

			names := map[string]string{
				"read_file":             "Read file",
				"view_file":             "Read file",
				"view_file_outline":     "File outline",
				"view_code_item":        "Read code",
				"list_dir":              "List directory",
				"grep_search":           "Search text",
				"find_by_name":          "Find file",
				"run_command":           "Terminal command",
				"finish":                "Format response",
				"inspect_git_status":    "Inspect Git status",
				"inspect_git_log":       "Inspect Git log",
				"inspect_git_diff":      "Inspect Git diff summary",
				"inspect_git_branches":  "Inspect Git branches",
				"inspect_system_health":    "Inspect system health",
				"inspect_typecheck":        "Inspect project typecheck",
				"search_web":               "Web search",
				"list_projects":            "List projects",
				"get_project":              "Get project",
				"list_tasks":                "List tasks",
				"get_task":                 "Get task",
				"create_task_draft":        "Create task draft",
				"list_inbox_conversations": "List inbox conversations",
				"get_inbox_messages":       "Get inbox messages",
				"list_contacts":            "List contacts",
				"get_contact":              "Get contact",
				"list_scheduled_jobs":      "List scheduled jobs",
				"get_system_settings":      "Get system settings",
				"call_mcp_tool":            "Call MCP tool",
			}
			name := names[u.Tool]
			if name == "" {
				name = "Tool"
			}
			kind, label := "runtime_tool_started", "started"
			if u.State == "DONE" {
				kind, label = "runtime_tool_finished", "activity ended"
			}
			if u.State == "ERROR" {
				kind, label = "runtime_tool_failed", "failed"
				s.toolFailures++
			}
			s.emit(kind, fmt.Sprintf("Tool #%d · %s %s%s.", *u.Index, name, label, duration))
		} else {
			kind, label := "runtime_provider_started", "The provider is preparing a response."
			if u.State == "DONE" {
				kind, label = "runtime_provider_finished", "Provider response stage finished"+duration+"."
			}
			if u.State == "ERROR" {
				kind, label = "runtime_provider_failed", "Provider response stage failed"+duration+"."
			}
			s.emit(kind, label)
		}
	default:
		s.unsafeTool = true
	}
}
func (s *inboxRuntimeStream) finish() (string, string, string) {
	if len(s.pending) > 0 {
		s.consume(s.pending)
		s.pending = nil
	}
	if s.toolBudgetExceeded {
		return "", "Provider tool budget exceeded for chat.", "tool_budget_exceeded"
	}
	if s.overflow {
		return "", "Provider activity exceeded the size limit. Your message is saved. The request was not rerun.", "runtime_overflow"
	}
	if s.invalid {
		return "", "Invalid runtime event format. Your message is saved. The request was not rerun.", "runtime_invalid"
	}
	if len(s.result) == 0 {
		return "", "The process stopped before a final result. Your message is saved, but the result is unknown. The request was not rerun.", "runtime_missing_result"
	}
	return strings.TrimSpace(string(s.result)), "", ""
}
