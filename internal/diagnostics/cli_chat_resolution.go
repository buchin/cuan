package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/buchin/cuan/internal/db"
)

func runChatResolution(ctx context.Context, command string, args []string, out, errOut io.Writer) int {
	if len(args) != 1 || !db.IsValidUUID(args[0]) {
		fmt.Fprintf(errOut, "Usage: cuan chat %s <room-id>\n", command)
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
	payload := map[string]any{"resolved": command == "resolve"}
	if command == "resolve" {
		response, err := client.doRequest(ctx, http.MethodGet, "/api/v1/chat/contacts?view=list", nil)
		if err != nil {
			fmt.Fprintln(errOut, "Could not observe conversation status. Nothing was resolved.")
			return 1
		}
		var data struct {
			Contacts []struct {
				ID         string    `json:"conversation_id"`
				ObservedAt time.Time `json:"resolution_observed_at"`
			} `json:"contacts"`
		}
		err = json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&data)
		response.Body.Close()
		if err != nil || response.StatusCode != 200 {
			fmt.Fprintln(errOut, "Could not observe conversation status. Nothing was resolved.")
			return 1
		}
		var observed time.Time
		for _, contact := range data.Contacts {
			if contact.ID == args[0] {
				observed = contact.ObservedAt
				break
			}
		}
		if observed.IsZero() {
			fmt.Fprintln(errOut, "Conversation observation unavailable. Nothing was resolved.")
			return 1
		}
		payload["observed_at"] = observed
	}
	response, err := client.doRequest(ctx, http.MethodPatch, "/api/v1/chat/conversations/"+url.PathEscape(args[0])+"/resolution", payload)
	if err != nil {
		fmt.Fprintln(errOut, "Status change was not confirmed. Inspect the same conversation before retrying.")
		return 1
	}
	defer response.Body.Close()
	bodyBytes, readErr := io.ReadAll(io.LimitReader(response.Body, 65536))
	if readErr != nil {
		fmt.Fprintln(errOut, "Status change was not confirmed. Inspect the same conversation before retrying.")
		return 1
	}
	if response.StatusCode != 200 {
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(bodyBytes, &errResp) == nil && errResp.Error != "" {
			fmt.Fprintln(errOut, errResp.Error)
			return 1
		}
		fmt.Fprintln(errOut, "Status change was not confirmed. Inspect the same conversation before retrying.")
		return 1
	}
	var receipt struct {
		ID         string     `json:"id"`
		Resolved   *bool      `json:"resolved"`
		ResolvedAt *time.Time `json:"resolved_at"`
	}
	if json.Unmarshal(bodyBytes, &receipt) != nil || receipt.ID != args[0] || receipt.Resolved == nil || *receipt.Resolved != (command == "resolve") || (command == "resolve" && receipt.ResolvedAt == nil) {
		fmt.Fprintln(errOut, "Status change was not confirmed. Inspect the same conversation before retrying.")
		return 1
	}
	_ = json.NewEncoder(out).Encode(receipt)
	return 0
}
