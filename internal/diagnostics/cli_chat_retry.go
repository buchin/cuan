package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func runChatRetry(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) != 2 || strings.TrimSpace(args[0]) == "" || strings.TrimSpace(args[1]) == "" || strings.HasPrefix(args[0], "-") {
		fmt.Fprintln(errOut, "Usage: cuan chat retry <room-id> <request-id>")
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
	response, err := client.doRequest(ctx, http.MethodPost, "/api/v1/chat/conversations/"+url.PathEscape(args[0])+"/requests/"+url.PathEscape(args[1])+"/retry", nil)
	if err != nil {
		fmt.Fprintln(errOut, "Retry was not confirmed. Read the saved request before retrying.")
		return 1
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		fmt.Fprintf(errOut, "Retry was not confirmed (HTTP %d). Check the saved request.\n", response.StatusCode)
		return 1
	}
	var result struct {
		Request struct {
			ID    string `json:"request_id"`
			State string `json:"state"`
		} `json:"request"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&result) != nil || result.Request.ID == "" {
		fmt.Fprintln(errOut, "Retry response could not be verified. Check the saved request.")
		return 1
	}
	returnCode := 0
	if json.NewEncoder(out).Encode(result) != nil {
		returnCode = 1
	}
	return returnCode
}
