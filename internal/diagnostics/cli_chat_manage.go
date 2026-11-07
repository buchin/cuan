package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/buchin/cuan/internal/db"
)

func runChatManage(ctx context.Context, command string, args []string, out, errOut io.Writer) int {
	count := 1
	if command == "cancel" {
		count = 2
	}
	if len(args) != count || !db.IsValidUUID(args[0]) || (count == 2 && !db.IsValidUUID(args[1])) {
		fmt.Fprintf(errOut, "Usage: cuan chat %s <room-id>", command)
		if count == 2 {
			fmt.Fprint(errOut, " <request-id>")
		}
		fmt.Fprintln(errOut)
		return 2
	}
	if err := taskSessionOwnerWriteError(); err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		fmt.Fprintln(errOut, "Configure the Cuan connection first.")
		return 1
	}
	method, endpoint := http.MethodDelete, "/api/v1/rooms/"+url.PathEscape(args[0])
	if command == "cancel" {
		method = http.MethodPost
		endpoint = "/api/v1/chat/conversations/" + url.PathEscape(args[0]) + "/requests/" + url.PathEscape(args[1]) + "/cancel"
	} else {
		snapshot, err := client.GetConversationSnapshot(ctx, args[0], 1)
		if err != nil {
			fmt.Fprintln(errOut, "Could not verify conversation state. Nothing was deleted.")
			return 1
		}
		requests, ok := snapshot["requests"].([]map[string]any)
		if !ok && snapshot["requests"] != nil {
			fmt.Fprintln(errOut, "Could not verify conversation requests. Nothing was deleted.")
			return 1
		}
		for _, request := range requests {
			switch request["state"] {
			case "queued", "running", "cancelling":
				fmt.Fprintln(errOut, "Stop active conversation requests with chat cancel and confirm they ended before deletion.")
				return 1
			case "completed", "failed", "interrupted", "cancelled", "canceled":
			default:
				fmt.Fprintln(errOut, "Could not verify a request state. Nothing was deleted.")
				return 1
			}
		}
	}
	response, err := client.doRequest(ctx, method, endpoint, nil)
	if err != nil {
		fmt.Fprintln(errOut, "Operation was not confirmed. Inspect the same conversation before retrying.")
		return 1
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		fmt.Fprintf(errOut, "Operation was not confirmed (HTTP %d). Inspect the same conversation.\n", response.StatusCode)
		return 1
	}
	var receipt struct {
		Status string `json:"status"`
		ID     string `json:"id"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&receipt) != nil ||
		(command == "delete" && (receipt.Status != "deleted" || receipt.ID != args[0])) ||
		(command == "cancel" && receipt.Status != "reconciled") {
		fmt.Fprintln(errOut, "Operation response could not be verified. Inspect the same conversation before retrying.")
		return 1
	}
	if command == "cancel" {
		fmt.Fprintln(out, "Cancellation requested. Check the same request to confirm it ended.")
	} else {
		fmt.Fprintln(out, "Conversation deleted.")
	}
	return 0
}
