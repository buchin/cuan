package diagnostics

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

// RunChatCLI handles the `cuan chat` CLI subcommands.
func RunChatCLI(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "help" {
		printChatHelp(out)
		return 0
	}

	subcommand := strings.ToLower(strings.TrimSpace(args[0]))
	extraArgs := args[1:]

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	switch subcommand {
	case "delete", "cancel":
		return runChatManage(ctx, subcommand, extraArgs, out, errOut)
	case "trace":
		return runChatTrace(ctx, extraArgs, out, errOut)
	case "retry":
		return runChatRetry(ctx, extraArgs, out, errOut)
	case "download":
		return runChatDownload(ctx, extraArgs, out, errOut)
	case "new", "create":
		return runChatNew(ctx, extraArgs, out, errOut)
	case "send":
		return runChatSend(ctx, extraArgs, out, errOut)
	case "list", "ls":
		return runChatList(ctx, extraArgs, out, errOut)
	case "messages", "history", "log":
		return runChatMessages(ctx, extraArgs, out, errOut)
	default:
		fmt.Fprintf(errOut, "Unknown chat subcommand: %s. Run 'cuan chat help' for usage.\n", subcommand)
		return 2
	}
}

func printChatHelp(out io.Writer) {
	fmt.Fprintln(out, `Cuan Chat CLI — Interact directly with agents and Inbox conversations

Usage:
  cuan chat new [--agent @sm] [--project default] [--title "New conversation"] [--json]
  cuan chat send <room-id> "<message>" [--wait] [--timeout 30] [--json]
  cuan chat list [--project default] [--limit 20] [--json]
  cuan chat messages <room-id> [--limit 30] [--json]
  cuan chat trace <room-id> <request-id> [--after-seq 0] [--limit 100]
  cuan chat retry <room-id> <request-id>
  cuan chat download <attachment-id> --output PATH
  cuan chat cancel <room-id> <request-id>
  cuan chat delete <room-id>

Subcommands:
  new       Create a new agent conversation room in Inbox
  send      Send a message to an agent room and optionally wait for reply
  list      List recent conversations and topics in Inbox
  messages  Inspect message history for a conversation room

Examples:
  cuan chat new --agent @sm --title "Audit CI/CD pipeline"
  cuan chat send <room-id> "Verify go.mod dependencies" --wait
  cuan chat list --limit 10
  cuan chat messages <room-id>`)
}

func runChatNew(ctx context.Context, args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("chat new", flag.ContinueOnError)
	fs.SetOutput(errOut)
	agentFlag := fs.String("agent", "@sm", "Agent persona alias (@sm, @dev, @support) or ID")
	projFlag := fs.String("project", "default", "Project ID or name")
	titleFlag := fs.String("title", "New conversation", "Conversation topic title")
	jsonFlag := fs.Bool("json", false, "Output in JSON format")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	res, err := CreateInboxConversation(ctx, *agentFlag, *projFlag, *titleFlag)
	if err != nil {
		fmt.Fprintf(errOut, "Error creating conversation: %v\n", err)
		return 1
	}

	if *jsonFlag {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		return 0
	}

	resMap, _ := res.(map[string]any)
	roomID, _ := resMap["room_id"].(string)
	if roomID == "" {
		roomID, _ = resMap["conversation_id"].(string)
	}
	title, _ := resMap["title"].(string)
	agentName, _ := resMap["agent_name"].(string)
	if agentName == "" {
		agentName = *agentFlag
	}

	cfg := GetCuanConfig()
	fmt.Fprintln(out, "Conversation created successfully:")
	fmt.Fprintf(out, "  Room ID:      %s\n", roomID)
	fmt.Fprintf(out, "  Agent:        %s\n", agentName)
	fmt.Fprintf(out, "  Title:        %s\n", title)
	if cfg.BaseURL != "" {
		fmt.Fprintf(out, "  URL:          %s/inbox/%s\n", cfg.BaseURL, roomID)
	}
	fmt.Fprintf(out, "\nSend a message:\n  cuan chat send %s \"Your question here\" --wait\n", roomID)
	return 0
}

func runChatSend(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(errOut, "Usage: cuan chat send <room-id> \"<message>\" [--wait] [--timeout 30] [--json]")
		return 2
	}

	roomID := args[0]
	message := args[1]
	extra := args[2:]

	fs := flag.NewFlagSet("chat send", flag.ContinueOnError)
	fs.SetOutput(errOut)
	waitFlag := fs.Bool("wait", true, "Wait and stream/print the assistant's reply")
	timeoutFlag := fs.Int("timeout", 45, "Max seconds to wait for assistant reply")
	jsonFlag := fs.Bool("json", false, "Output in JSON format")

	if err := fs.Parse(extra); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	res, err := SendInboxMessage(ctx, roomID, message, *waitFlag, *timeoutFlag)
	if err != nil {
		fmt.Fprintf(errOut, "Error sending message: %v\n", err)
		return 1
	}

	if *jsonFlag {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		return 0
	}

	resMap, _ := res.(map[string]any)
	status, _ := resMap["status"].(string)
	if status == "replied" {
		reply, _ := resMap["reply"].(string)
		agent, _ := resMap["agent_name"].(string)
		if agent == "" {
			agent = "Agent"
		}
		fmt.Fprintf(out, "[%s]:\n%s\n", agent, reply)
		return 0
	}

	if warning, ok := resMap["warning"].(string); ok {
		fmt.Fprintln(errOut, warning)
		return 0
	}

	fmt.Fprintf(out, "Message sent to room %s (status: %s)\n", roomID, status)
	return 0
}

func runChatList(ctx context.Context, args []string, out, errOut io.Writer) int {
	fs := flag.NewFlagSet("chat list", flag.ContinueOnError)
	fs.SetOutput(errOut)
	projFlag := fs.String("project", "", "Project ID filter")
	limitFlag := fs.Int("limit", 20, "Maximum conversations to display")
	jsonFlag := fs.Bool("json", false, "Output in JSON format")

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	res, err := ListInboxConversations(ctx, *projFlag, *limitFlag)
	if err != nil {
		fmt.Fprintf(errOut, "Error listing conversations: %v\n", err)
		return 1
	}

	if *jsonFlag {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		return 0
	}

	resMap, _ := res.(map[string]any)
	convs, _ := resMap["conversations"].([]any)
	if convs == nil {
		if rawConvs, ok := resMap["conversations"].([]InboxConversation); ok {
			for _, c := range rawConvs {
				convs = append(convs, c)
			}
		} else if rawList, ok := resMap["conversations"].([]map[string]any); ok {
			for _, c := range rawList {
				convs = append(convs, c)
			}
		}
	}

	fmt.Fprintf(out, "Found %d conversation(s):\n\n", len(convs))
	for i, item := range convs {
		var id, title, agent, updatedAt string
		if m, ok := item.(map[string]any); ok {
			id, _ = m["id"].(string)
			if id == "" {
				id, _ = m["conversation_id"].(string)
			}
			title, _ = m["title"].(string)
			agent, _ = m["agent_id"].(string)
			updatedAt, _ = m["updated_at"].(string)
		} else if ic, ok := item.(InboxConversation); ok {
			id = ic.ID
			title = ic.Title
			agent = ic.Topic
			updatedAt = ic.UpdatedAt
		}
		fmt.Fprintf(out, "%2d. [%s] %s\n    Room ID: %s | Updated: %s\n", i+1, agent, title, id, updatedAt)
	}
	return 0
}

func runChatMessages(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "Usage: cuan chat messages <room-id> [--limit 30] [--json]")
		return 2
	}

	roomID := args[0]
	fs := flag.NewFlagSet("chat messages", flag.ContinueOnError)
	fs.SetOutput(errOut)
	limitFlag := fs.Int("limit", 30, "Maximum messages to retrieve")
	jsonFlag := fs.Bool("json", false, "Output in JSON format")

	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	res, err := GetInboxMessages(ctx, roomID, *limitFlag)
	if err != nil {
		fmt.Fprintf(errOut, "Error retrieving messages: %v\n", err)
		return 1
	}

	if *jsonFlag {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		return 0
	}

	resMap, _ := res.(map[string]any)
	rawMsgs, _ := resMap["messages"].([]any)
	if rawMsgs == nil {
		if ms, ok := resMap["messages"].([]map[string]any); ok {
			for _, m := range ms {
				rawMsgs = append(rawMsgs, m)
			}
		} else if ims, ok := resMap["messages"].([]InboxMessage); ok {
			for _, m := range ims {
				rawMsgs = append(rawMsgs, m)
			}
		}
	}

	fmt.Fprintf(out, "Conversation history for room %s (%d messages):\n\n", roomID, len(rawMsgs))
	for _, raw := range rawMsgs {
		var author, content, timeStr string
		if m, ok := raw.(map[string]any); ok {
			author, _ = m["author_name"].(string)
			if author == "" {
				author, _ = m["author_type"].(string)
			}
			content, _ = m["content"].(string)
			if content == "" {
				content, _ = m["message"].(string)
			}
			timeStr, _ = m["created_at"].(string)
		} else if im, ok := raw.(InboxMessage); ok {
			author = im.AuthorName
			if author == "" {
				author = im.AuthorType
			}
			content = im.Content
			timeStr = im.CreatedAt
		}
		fmt.Fprintf(out, "--- [%s] at %s ---\n%s\n\n", author, timeStr, content)
	}
	if requests, ok := resMap["requests"].([]map[string]any); ok {
		for _, request := range requests {
			state, _ := request["state"].(string)
			if state == "queued" || state == "running" || state == "failed" || state == "interrupted" {
				fmt.Fprintf(out, "Request %v: %s", request["request_id"], state)
				if reason, _ := request["safe_error"].(string); reason != "" && state != "running" {
					fmt.Fprintf(out, " — %s", reason)
				}
				fmt.Fprintln(out)
			}
		}
	}

	return 0
}
