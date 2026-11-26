package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	cuanPrompt "github.com/buchin/cuan/internal/prompt"
	"github.com/buchin/cuan/internal/workflow"
)

// This is a transport contract, not a grant of authority. The existing owner,
// persisted approval, project, cancellation and task launch checks still apply.
// Keep the provider's delegation fields aligned with the native parser. A free
// object here lets the formatter emit payloads that the application cannot use.
const inboxDelegationPayloadSchema = `{"type":"object","properties":{"proposal_message_id":{"type":"string"},"reconcile":{"type":"boolean"},"execution_category":{"type":"string","enum":["code","ops","readonly"]},"action":{"type":"string","enum":["run","save"]},"title":{"type":"string","maxLength":200},"description":{"type":"string","maxLength":6000},"project_id":{"type":"string"},"project_name":{"type":"string","maxLength":100},"authorization_message_id":{"type":"string"},"task_id":{"type":"string"},"revision":{"type":"string","maxLength":6000},"agent_id":{"type":"string"},"required_stages":{"type":"array","items":{"type":"string","enum":["implement","verify","publish"]}}},"anyOf":[{"required":["proposal_message_id","authorization_message_id"]},{"required":["title","description","authorization_message_id"],"anyOf":[{"required":["project_id"]},{"required":["project_name"]}]},{"required":["task_id","revision"]}],"additionalProperties":false}`

const inboxCoordinatorSchema = `{"type":"object","properties":{"reply":{"type":"string"},"topic_title":{"type":"string"},"action":{"anyOf":[{"type":"null"},{"type":"object","properties":{"kind":{"enum":["delegation"]},"payload":` + inboxDelegationPayloadSchema + `},"required":["kind","payload"],"additionalProperties":false},{"type":"object","properties":{"kind":{"enum":["approval","memory","memory_search","task_draft","task_cancel","task_reopen","task_link","prompt_choices","customer_reply","schedule","project","skill","team","brain","brand_memory"]},"payload":{"type":"object"}},"required":["kind","payload"],"additionalProperties":false}]}},"required":["reply","topic_title","action"],"additionalProperties":false}`

const inboxAnalysisRecoverySchema = `{"type":"object","properties":{"reply":{"type":"string"},"topic_title":{"type":"string","maxLength":0},"action":{"type":"null"}},"required":["reply","topic_title","action"],"additionalProperties":false}`

const inboxDeniedToolRecoverySchema = `{"type":"object","properties":{"reply":{"type":"string"},"topic_title":{"type":"string","maxLength":0},"action":{"anyOf":[{"type":"null"},{"type":"object","properties":{"kind":{"enum":["approval"]},"payload":{"type":"object"}},"required":["kind","payload"],"additionalProperties":false}]}},"required":["reply","topic_title","action"],"additionalProperties":false}`

var inboxCoordinatorInstruction = cuanPrompt.ConversationStyle + "\n\n" + workflow.Coordinator

const inboxAnalysisRecoveryInstruction = `The previous reply attempted a tool that the runtime denied. Its partial reply was not accepted and none of its application actions were applied. This does not establish the outcome of other native activity. Answer the SAME original target message, preserving its intent and all supplied constraints: a request for a plan remains planning, a clarification remains clarification, and an actual code question remains that question. Do not turn every recovery into technical investigation. Use supplied conversation and task evidence first. Do not read platform/orchestration source merely to compose a plan, ask a question or format JSON. Use native read-only file/search tools only when the original requested answer depends on specific missing current implementation facts. Never call run_command, terminal, browser, write or execution tools, even for a search. Do not repeat implementation or external effects, or inspect provider HOME/transcripts to reconstruct history. Preserve uncertainty; never claim denied activity succeeded or work started.
Return topic_title:"". Return action:null for ordinary discussion. If the original request needs a concrete plan for a new user approval, you may return a NEW approval proposal containing its complete visible scope, constraints, acceptance and project binding. It remains unexecuted until separately approved; never reuse an action or partial plan from the rejected reply. Delegation, execution, task mutation, memory writes and all other application actions remain forbidden in this recovery. Do not ask for fresh approval merely to retry already authorized work. If evidence is insufficient, state the specific missing evidence without inventing a plan or outcome. Answer naturally in the user's language with detail appropriate to the original request.`

const inboxToolBudgetRecoveryInstruction = `Direct coordinator synthesis mode: The tool turn budget has been reached for this chat turn. Do not call any tools. Answer the user's actual question from the supplied conversation, recorded request outcomes, task evidence and gathered facts. Keep the chat reply concise and readable, preserving material context and uncertainty. Cite the sources supporting material facts, distinguish upstream claims from measurements, and omit a technical recap unless requested. Return action: null and topic_title: "".`

type inboxCoordinatorResponse struct {
	Reply      string `json:"reply"`
	TopicTitle string `json:"topic_title"`
	Action     *struct {
		Kind    string          `json:"kind"`
		Payload json.RawMessage `json:"payload"`
	} `json:"action"`
}

// Reject duplicate keys before typed decoding: ambiguous actions must never
// depend on a JSON decoder's last-key-wins behavior.
func uniqueCoordinatorJSON(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 12 {
			return errors.New("JSON nesting limit")
		}
		token, err := d.Token()
		if err != nil {
			return err
		}
		delim, compound := token.(json.Delim)
		if !compound {
			return nil
		}
		if delim != '{' && delim != '[' {
			return errors.New("unexpected delimiter")
		}
		keys := map[string]bool{}
		for d.More() {
			if delim == '{' {
				key, err := d.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || keys[name] {
					return errors.New("duplicate JSON key")
				}
				keys[name] = true
			}
			if err := walk(depth + 1); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	}
	if err := walk(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("extra JSON data")
	}
	return nil
}

func strictCoordinatorJSON(raw []byte, value any) error {
	if err := uniqueCoordinatorJSON(raw); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(value); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("extra JSON value")
	}
	return nil
}

// Native schema output may repeat its identical JSON after a fenced response.
// Only that exact semantic duplicate is accepted; conflicting actions are not.
func decodeInboxCoordinatorResponse(raw string) (string, error) {
	return decodeInboxCoordinatorResponseForMode(raw, false)
}

func decodeInboxDeniedToolRecoveryResponse(raw string) (string, error) {
	return decodeInboxCoordinatorResponseWithLimits(raw, inboxResponseLimits{AnalysisOnly: true, AllowProposal: true})
}

func decodeInboxCoordinatorResponseForMode(raw string, analysisOnly bool, reconciliation ...bool) (string, error) {
	return decodeInboxCoordinatorResponseWithLimits(raw, inboxResponseLimits{AnalysisOnly: analysisOnly, Reconcile: len(reconciliation) > 0 && reconciliation[0]})
}

type inboxResponseLimits struct {
	AnalysisOnly  bool
	Reconcile     bool
	AllowProposal bool
}

func decodeInboxCoordinatorResponseWithLimits(raw string, limits inboxResponseLimits) (string, error) {
	analysisOnly, reconcile := limits.AnalysisOnly, limits.Reconcile
	bad := errors.New("invalid coordinator response")
	if len(raw) > 40000 {
		return "", bad
	}
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```json") {
		newline := strings.IndexByte(raw, '\n')
		if newline > 0 {
			end := strings.Index(raw[newline+1:], "```")
			if end >= 0 {
				end += newline + 1
				raw = strings.TrimSpace(raw[newline+1:end] + "\n" + raw[end+3:])
			}
		}
	}
	d := json.NewDecoder(strings.NewReader(raw))
	var firstRaw json.RawMessage
	var value inboxCoordinatorResponse
	if d.Decode(&firstRaw) != nil || strictCoordinatorJSON(firstRaw, &value) != nil {
		return "", bad
	}
	var extraRaw json.RawMessage
	if err := d.Decode(&extraRaw); err != io.EOF {
		var extra inboxCoordinatorResponse
		if strictCoordinatorJSON(extraRaw, &extra) != nil {
			return "", bad
		}
		first, _ := json.Marshal(value)
		second, _ := json.Marshal(extra)
		// Compare normalized payloads too: native schema echoes may reorder keys.
		var a, b any
		_ = json.Unmarshal(first, &a)
		_ = json.Unmarshal(second, &b)
		first, _ = json.Marshal(a)
		second, _ = json.Marshal(b)
		if err != nil || !bytes.Equal(first, second) || d.Decode(new(any)) != io.EOF {
			return "", bad
		}
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(firstRaw, &fields)
	if len(fields) != 3 || len(fields["reply"]) == 0 || fields["reply"][0] != '"' || len(fields["topic_title"]) == 0 || fields["topic_title"][0] != '"' || len(fields["action"]) == 0 {
		return "", bad
	}
	value.Reply = strings.TrimSpace(value.Reply)
	if len(value.Reply) > 18000 || len([]rune(value.TopicTitle)) > 72 || strings.Contains(value.Reply, "```cuan-") || strings.Contains(strings.ToLower(value.Reply), "file://") {
		return "", bad
	}
	output := value.Reply
	if analysisOnly {
		if value.TopicTitle != "" {
			return "", bad
		}
		if value.Action != nil && !(reconcile && value.Action.Kind == "delegation") && !(limits.AllowProposal && value.Action.Kind == "approval") {
			return "", bad
		}
	}
	if !analysisOnly && isExplicitChoicePrompt(value.Reply) && (value.Action == nil || value.Action.Kind != "prompt_choices") {
		// Let the existing format-only repair supply the structured options.
		// Never turn a naming/preference question into executable approval.
		return "", bad
	}
	if !analysisOnly && value.Action == nil && isConfirmationQuestion(value.Reply) {
		title := extractTitleFromReply(value.Reply)
		projectName := "Default"
		lower := strings.ToLower(value.Reply)
		if strings.Contains(lower, "cuan") || strings.Contains(lower, "studio") || strings.Contains(lower, "form") {
			projectName = "Cuan"
		}
		desc := value.Reply
		if len([]rune(desc)) > 6000 {
			desc = string([]rune(desc)[:6000])
		}
		payloadBytes, _ := json.Marshal(inboxApprovalPlan{
			Title:       title,
			Description: desc,
			ProjectName: projectName,
		})
		value.Action = &struct {
			Kind    string          `json:"kind"`
			Payload json.RawMessage `json:"payload"`
		}{
			Kind:    "approval",
			Payload: payloadBytes,
		}
	}
	if value.Action != nil {
		if len(value.Action.Payload) == 0 || bytes.Equal(bytes.TrimSpace(value.Action.Payload), []byte("null")) {
			return "", bad
		}
		var marker string
		var payload any
		switch value.Action.Kind {
		case "approval":
			marker, payload = "cuan-approval", &inboxApprovalPlan{}
		case "delegation":
			marker, payload = "cuan-delegate", &inboxDelegation{}
		case "memory":
			marker, payload = "cuan-memory", &inboxMemoryAction{}
		case "memory_search":
			marker, payload = "cuan-search", &inboxMemorySearch{}
		case "task_draft":
			marker, payload = "cuan-task", &InboxTaskDraft{}
		case "task_cancel":
			marker, payload = "cuan-task-cancel", &inboxTaskCancelAction{}
		case "task_reopen":
			marker, payload = "cuan-task-reopen", &inboxTaskReopenAction{}
		case "task_link":
			marker, payload = "cuan-task-link", &inboxTaskLinkAction{}
		case "prompt_choices":
			marker, payload = "cuan-prompt", &InboxPromptChoices{}
		case "customer_reply":
			marker, payload = "cuan-customer-reply", &customerReplyPayload{}
		case "schedule":
			marker, payload = "cuan-schedule", &inboxScheduleAction{}
		case "project":
			marker, payload = "cuan-project", &inboxProjectAction{}
		case "skill":
			marker, payload = "cuan-skill", &inboxSkillAction{}
		case "team":
			marker, payload = "cuan-team", &inboxTeamAction{}
		case "brain":
			marker, payload = "cuan-brain", &inboxBrainAction{}
		case "brand_memory":
			marker, payload = "cuan-brand-memory", &inboxBrandMemoryAction{}
		default:
			return "", bad
		}
		if strictCoordinatorJSON(value.Action.Payload, payload) != nil {
			return "", bad
		}
		encoded, _ := json.Marshal(payload)
		output += "\n```" + marker + "\n" + string(encoded) + "\n```"
		switch value.Action.Kind {
		case "approval":
			if parseInboxApprovalPlan(output) == nil {
				return "", bad
			}
		case "delegation":
			_, delegation, err := parseInboxDelegation(output)
			if err != nil || delegation == nil {
				return "", bad
			}
			if reconcile != delegation.Reconcile || (reconcile && (delegation.Action == "save" || delegation.ProjectID == "" || delegation.ProjectName != "")) {
				return "", bad
			}
			if delegation.ProjectID != "" && strings.TrimSpace(delegation.ProjectName) != "" && !strings.EqualFold(delegation.ProjectID, delegation.ProjectName) {
				return "", bad
			}
		case "task_draft":
			if _, draft := splitInboxTaskDraft("agent", output); draft == nil {
				return "", bad
			}
		case "task_cancel":
			if _, cancelAction, err := parseInboxTaskCancel(output); err != nil || cancelAction == nil {
				return "", bad
			}
		case "task_reopen":
			if _, reopenAction, err := parseInboxTaskReopen(output); err != nil || reopenAction == nil {
				return "", bad
			}
		case "prompt_choices":
			if _, prompt := splitInboxPromptChoices("agent", output); prompt == nil {
				return "", bad
			}
		case "customer_reply":
			if _, draft := splitCustomerReplyDraft(output); draft == "" {
				return "", bad
			}
		case "schedule":
			if _, scheduleAction, err := parseInboxSchedule(output); err != nil || scheduleAction == nil {
				return "", bad
			}
		case "project":
			if _, projectAction, err := parseInboxProject(output); err != nil || projectAction == nil {
				return "", bad
			}
		case "skill":
			if _, skillAction, err := parseInboxSkill(output); err != nil || skillAction == nil {
				return "", bad
			}
		case "team":
			if _, teamAction, err := parseInboxTeam(output); err != nil || teamAction == nil {
				return "", bad
			}
		case "brain":
			if _, brainAction, err := parseInboxBrain(output); err != nil || brainAction == nil {
				return "", bad
			}
		case "brand_memory":
			if _, brandMemoryAction, err := parseInboxBrandMemory(output); err != nil || brandMemoryAction == nil {
				return "", bad
			}
		}
	}
	if strings.TrimSpace(output) == "" {
		return "", bad
	}
	if value.TopicTitle != "" {
		encoded, _ := json.Marshal(map[string]string{"title": value.TopicTitle})
		output += "\n```cuan-title\n" + string(encoded) + "\n```"
	}
	return strings.TrimSpace(output), nil
}

func isConfirmationQuestion(reply string) bool {
	lower := strings.ToLower(reply)
	markers := []string{
		"apakah anda ingin", "apakah anda setuju", "apakah mau", "apakah perlu",
		"bolehkah saya", "setuju?", "lanjut?", "apakah ingin", "ingin saya jalankan",
		"mau saya jalankan", "apakah saya", "ingin saya buatkan", "mau saya buatkan",
		"apakah kita", "bagaimana kalau saya",
		"ada yang ingin diperdalam", "ada yang mau dikerjakan", "mau mulai dari mana",
		"ingin kita mulai", "bagaimana menurut anda", "tertarik untuk mencoba",
		"sesi analisis mandiri", "analisis mandiri", "investigasi mandiri",
		"investigasi mendalam", "analisis mendalam",
		"pendekatan terbaik untuk implementasi", "solusi terbaik adalah",
		"solusi yang direkomendasikan", "rekomendasi perbaikan",
		"rencana perbaikan", "rekomendasi implementasi",
		"solusi dan standar", "rencana kerja", "kartu persetujuan",
		"solusinya adalah", "langkah perbaikan",
		"rencana perapian", "susun rencana", "tinggal klik", "klik gas", "balas gas",
		"langsung dirapikan", "langsung dieksekusi", "mau dirapikan", "mau dieksekusi",
		"apakah mau dirapikan", "bisa langsung dirapikan",
		"best approach for implementation", "recommended implementation",
		"would you like me to", "should i", "do you want me to", "shall i",
		"do you approve", "please confirm",
	}
	for _, m := range markers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

func isExplicitChoicePrompt(reply string) bool {
	paragraphs := strings.Split(strings.TrimSpace(reply), "\n\n")
	last := strings.ToLower(strings.TrimSpace(paragraphs[len(paragraphs)-1]))
	if strings.HasPrefix(last, ">") {
		return false
	}
	for _, marker := range []string{"silakan pilih", "silahkan pilih", "pilih salah satu", "please choose", "please select", "choose one", "select one"} {
		if strings.Contains(last, marker) {
			return true
		}
	}
	return strings.Contains(last, "pilih") && strings.Contains(last, "di bawah") || strings.Contains(last, "choose") && strings.Contains(last, "below")
}

func extractTitleFromReply(reply string) string {
	lower := strings.ToLower(reply)
	if strings.Contains(lower, "analisis mandiri") || strings.Contains(lower, "investigasi mandiri") || strings.Contains(lower, "investigasi mendalam") {
		return "Sesi Analisis Mandiri"
	}
	start := strings.Index(reply, "**")
	if start >= 0 {
		end := strings.Index(reply[start+2:], "**")
		if end > 0 {
			candidate := strings.TrimSpace(reply[start+2 : start+2+end])
			if len([]rune(candidate)) > 3 && len([]rune(candidate)) <= 100 {
				return candidate
			}
		}
	}
	start = strings.Index(reply, `"`)
	if start >= 0 {
		end := strings.Index(reply[start+1:], `"`)
		if end > 0 {
			candidate := strings.TrimSpace(reply[start+1 : start+1+end])
			if len([]rune(candidate)) > 5 && len([]rune(candidate)) <= 100 {
				return candidate
			}
		}
	}
	return "Usulan Tugas Baru"
}

func synthesizeApprovalReply(textReply string) string {
	if strings.Contains(textReply, "```cuan-approval") || strings.Contains(textReply, "```cuan-delegate") {
		return textReply
	}
	title := extractTitleFromReply(textReply)
	projectName := "Default"
	lower := strings.ToLower(textReply)
	if strings.Contains(lower, "cuan") || strings.Contains(lower, "studio") || strings.Contains(lower, "form") {
		projectName = "Cuan"
	}
	desc := textReply
	if len([]rune(desc)) > 6000 {
		desc = string([]rune(desc)[:6000])
	}
	payloadBytes, _ := json.Marshal(inboxApprovalPlan{
		Title:       title,
		Description: desc,
		ProjectName: projectName,
	})
	return textReply + "\n```cuan-approval\n" + string(payloadBytes) + "\n```"
}

func stripThinkingTags(s string) string {
	for {
		start := strings.Index(s, "<think>")
		if start == -1 {
			break
		}
		end := strings.Index(s[start:], "</think>")
		if end == -1 {
			s = s[:start]
			break
		}
		s = s[:start] + s[start+end+8:]
	}
	for {
		start := strings.Index(s, "<thought>")
		if start == -1 {
			break
		}
		end := strings.Index(s[start:], "</thought>")
		if end == -1 {
			s = s[:start]
			break
		}
		s = s[:start] + s[start+end+10:]
	}
	return strings.TrimSpace(s)
}

func sanitizeReplyText(text string) string {
	text = stripThinkingTags(text)
	text = strings.ReplaceAll(text, "file:///", "/")
	text = strings.ReplaceAll(text, "file://", "/")
	return strings.TrimSpace(text)
}

func recoverCoordinatorReply(raw string, analysisRecovery bool) (string, string) {
	output, failure := recoverCoordinatorReplyContent(raw, analysisRecovery)
	// Lenient extraction is never authority to recover an application action.
	// A fresh recovery proposal must pass the strict proposal-only decoder.
	if analysisRecovery && strings.Contains(strings.ToLower(output), "```cuan-") {
		return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
	}
	return output, failure
}

func recoverCoordinatorReplyContent(raw string, analysisRecovery bool) (string, string) {
	clean := sanitizeReplyText(raw)
	if clean == "" || clean == "invalid transport" || clean == "invalid non-json transport" {
		return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
	}

	// 1. Try to extract JSON candidate from markdown code fence or braces
	jsonCandidate := clean
	if startIdx := strings.Index(clean, "```json"); startIdx != -1 {
		rest := clean[startIdx+7:]
		if endIdx := strings.Index(rest, "```"); endIdx != -1 {
			jsonCandidate = strings.TrimSpace(rest[:endIdx])
		}
	} else if startIdx := strings.Index(clean, "```"); startIdx != -1 {
		rest := clean[startIdx+3:]
		if nl := strings.IndexByte(rest, '\n'); nl != -1 {
			rest = rest[nl+1:]
		}
		if endIdx := strings.Index(rest, "```"); endIdx != -1 {
			jsonCandidate = strings.TrimSpace(rest[:endIdx])
		}
	} else if openBrace := strings.Index(clean, "{"); openBrace != -1 {
		if closeBrace := strings.LastIndex(clean, "}"); closeBrace > openBrace {
			jsonCandidate = strings.TrimSpace(clean[openBrace : closeBrace+1])
		}
	}

	// Try standard decoding on extracted JSON candidate if it differs from clean
	if jsonCandidate != clean {
		if out, err := decodeInboxCoordinatorResponseForMode(jsonCandidate, analysisRecovery); err == nil && out != "" {
			return out, ""
		}
	}

	// 2. Lenient JSON decoding (tolerates extra fields like 'thought', 'reasoning', alternative field names)
	var lenient struct {
		Reply      string `json:"reply"`
		Content    string `json:"content"`
		Message    string `json:"message"`
		Text       string `json:"text"`
		Response   string `json:"response"`
		TopicTitle string `json:"topic_title"`
		Action     *struct {
			Kind    string          `json:"kind"`
			Payload json.RawMessage `json:"payload"`
		} `json:"action"`
	}
	if json.Unmarshal([]byte(jsonCandidate), &lenient) == nil {
		textReply := strings.TrimSpace(lenient.Reply)
		if textReply == "" {
			textReply = strings.TrimSpace(lenient.Content)
		}
		if textReply == "" {
			textReply = strings.TrimSpace(lenient.Message)
		}
		if textReply == "" {
			textReply = strings.TrimSpace(lenient.Text)
		}
		if textReply == "" {
			textReply = strings.TrimSpace(lenient.Response)
		}
		textReply = sanitizeReplyText(textReply)

		if textReply != "" {
			if analysisRecovery {
				if (lenient.Action != nil && len(lenient.Action.Payload) > 0 && !bytes.Equal(bytes.TrimSpace(lenient.Action.Payload), []byte("null"))) || lenient.TopicTitle != "" {
					return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
				}
				return textReply, ""
			}
			if isExplicitChoicePrompt(textReply) && (lenient.Action == nil || strings.ToLower(strings.TrimSpace(lenient.Action.Kind)) != "prompt_choices") {
				return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
			}
			if lenient.Action != nil && len(lenient.Action.Payload) > 0 && !bytes.Equal(bytes.TrimSpace(lenient.Action.Payload), []byte("null")) {
				var marker string
				kind := strings.ToLower(strings.TrimSpace(lenient.Action.Kind))
				switch kind {
				case "approval", "proposal", "plan", "task_proposal":
					marker = "cuan-approval"
				case "delegation", "delegate":
					marker = "cuan-delegate"
				case "memory":
					marker = "cuan-memory"
				case "memory_search", "search":
					marker = "cuan-search"
				case "task_draft", "task":
					marker = "cuan-task"
				case "task_cancel":
					marker = "cuan-task-cancel"
				case "task_reopen":
					marker = "cuan-task-reopen"
				case "prompt_choices":
					marker = "cuan-prompt"
				case "customer_reply":
					marker = "cuan-customer-reply"
				case "schedule":
					marker = "cuan-schedule"
				case "project":
					marker = "cuan-project"
				case "skill":
					marker = "cuan-skill"
				case "team":
					marker = "cuan-team"
				case "brain":
					marker = "cuan-brain"
				case "brand_memory":
					marker = "cuan-brand-memory"
				}
				if marker != "" {
					out := textReply
					if lenient.TopicTitle != "" && len([]rune(lenient.TopicTitle)) <= 72 {
						encodedTitle, _ := json.Marshal(map[string]string{"title": lenient.TopicTitle})
						out += "\n```cuan-title\n" + string(encodedTitle) + "\n```"
					}
					payloadBytes := lenient.Action.Payload
					var strPayload string
					if json.Unmarshal(payloadBytes, &strPayload) == nil && json.Valid([]byte(strPayload)) {
						payloadBytes = []byte(strPayload)
					}
					out += "\n```" + marker + "\n" + string(payloadBytes) + "\n```"
					if marker == "cuan-delegate" {
						if _, delegation, err := parseInboxDelegation(out); err != nil || delegation == nil {
							return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
						}
					}
					if marker == "cuan-prompt" {
						if _, choices := splitInboxPromptChoices("agent", out); choices == nil {
							return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
						}
					}
					if marker == "cuan-schedule" {
						if _, action, err := parseInboxSchedule(out); err != nil || action == nil {
							return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
						}
					}
					if marker == "cuan-project" {
						if _, action, err := parseInboxProject(out); err != nil || action == nil {
							return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
						}
					}
					if marker == "cuan-skill" {
						if _, action, err := parseInboxSkill(out); err != nil || action == nil {
							return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
						}
					}
					if marker == "cuan-team" {
						if _, action, err := parseInboxTeam(out); err != nil || action == nil {
							return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
						}
					}
					if marker == "cuan-brain" {
						if _, action, err := parseInboxBrain(out); err != nil || action == nil {
							return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
						}
					}
					if marker == "cuan-brand-memory" {
						if _, action, err := parseInboxBrandMemory(out); err != nil || action == nil {
							return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
						}
					}
					return out, ""
				}
			}

			if isExplicitChoicePrompt(textReply) {
				return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
			}
			if isConfirmationQuestion(textReply) {
				return synthesizeApprovalReply(textReply), ""
			}
			if lenient.TopicTitle != "" && len([]rune(lenient.TopicTitle)) <= 72 {
				encodedTitle, _ := json.Marshal(map[string]string{"title": lenient.TopicTitle})
				return textReply + "\n```cuan-title\n" + string(encodedTitle) + "\n```", ""
			}
			return textReply, ""
		}
		// If JSON decoded successfully but had no reply text, do not treat raw JSON as plain text
		return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
	}

	// 2b. Attempt to extract "reply": "..." if JSON was slightly broken
	if strings.Contains(clean, `"reply"`) {
		idx := strings.Index(clean, `"reply"`)
		rest := clean[idx+7:]
		if colon := strings.Index(rest, ":"); colon != -1 {
			rest = strings.TrimSpace(rest[colon+1:])
			if strings.HasPrefix(rest, `"`) {
				rest = rest[1:]
				for i := 0; i < len(rest); i++ {
					if rest[i] == '"' && (i == 0 || rest[i-1] != '\\') {
						extracted := sanitizeReplyText(rest[:i])
						if extracted != "" {
							if analysisRecovery {
								return extracted, ""
							}
							if isExplicitChoicePrompt(extracted) {
								return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
							}
							if isConfirmationQuestion(extracted) {
								return synthesizeApprovalReply(extracted), ""
							}
							return extracted, ""
						}
						break
					}
				}
			}
		}
	}

	// If the entire text was a JSON object that failed to decode, do not treat as plain text
	trimmedClean := strings.TrimSpace(clean)
	if strings.HasPrefix(trimmedClean, "{") && strings.HasSuffix(trimmedClean, "}") {
		return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
	}

	// 3. Fallback to extracting plain text (stripping markdown code fences if wrapped)
	textReply := clean
	if strings.HasPrefix(textReply, "```") {
		textReply = strings.TrimPrefix(textReply, "```json")
		textReply = strings.TrimPrefix(textReply, "```")
		textReply = strings.TrimPrefix(textReply, "\n")
		textReply = strings.TrimPrefix(textReply, "\r\n")
		if idx := strings.LastIndex(textReply, "```"); idx != -1 {
			textReply = textReply[:idx]
		}
		textReply = strings.TrimSpace(textReply)
	}
	textReply = sanitizeReplyText(textReply)

	if textReply != "" {
		if analysisRecovery {
			if strings.Contains(clean, "cuan-approval") || strings.Contains(clean, "cuan-delegate") || strings.Contains(clean, "cuan-title") {
				return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
			}
			return textReply, ""
		}
		if isExplicitChoicePrompt(textReply) {
			return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
		}
		if isConfirmationQuestion(textReply) {
			return synthesizeApprovalReply(textReply), ""
		}
		return textReply, ""
	}

	return "", "The agent's response could not be validated. Your message and approved work are saved; no new task was started."
}
