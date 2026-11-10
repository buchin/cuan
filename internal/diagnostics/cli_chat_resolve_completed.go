package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

func runChatResolveCompleted(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(errOut, "Usage: cuan chat resolve-completed")
		return 2
	}
	if err := taskSessionOwnerWriteError(); err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	c := GetCuanClient()
	if c == nil || !c.IsConfigured() {
		fmt.Fprintln(errOut, "Configure the Cuan connection first.")
		return 1
	}
	response, err := c.doRequest(ctx, http.MethodGet, "/api/v1/chat/contacts?view=list", nil)
	if err != nil {
		fmt.Fprintln(errOut, "Could not observe conversations. Nothing was resolved.")
		return 1
	}
	var data struct {
		Contacts []struct {
			ID         string    `json:"conversation_id"`
			Resolved   bool      `json:"resolved"`
			ObservedAt time.Time `json:"resolution_observed_at"`
			Total      int       `json:"task_count"`
			Done       int       `json:"done_tasks"`
			Canceled   int       `json:"canceled_tasks"`
			Active     int       `json:"active_tasks"`
			Queued     int       `json:"queued_tasks"`
			Review     int       `json:"review_tasks"`
			Failed     int       `json:"failed_tasks"`
		} `json:"contacts"`
	}
	err = json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&data)
	response.Body.Close()
	if err != nil || response.StatusCode != 200 {
		fmt.Fprintln(errOut, "Could not observe conversations. Nothing was resolved.")
		return 1
	}
	resolved, failed := []string{}, []string{}
	seen := map[string]bool{}
	for _, room := range data.Contacts {
		if seen[room.ID] || room.Resolved || room.Done == 0 || room.Total != room.Done+room.Canceled || room.Active+room.Queued+room.Review+room.Failed != 0 || room.ObservedAt.IsZero() {
			continue
		}
		seen[room.ID] = true
		response, err := c.doRequest(ctx, http.MethodPatch, "/api/v1/chat/conversations/"+url.PathEscape(room.ID)+"/resolution", map[string]any{"resolved": true, "observed_at": room.ObservedAt})
		if err != nil {
			failed = append(failed, room.ID)
			continue
		}
		var receipt struct {
			ID       string `json:"id"`
			Resolved bool   `json:"resolved"`
		}
		err = json.NewDecoder(io.LimitReader(response.Body, 65536)).Decode(&receipt)
		response.Body.Close()
		if err != nil || response.StatusCode != 200 || receipt.ID != room.ID || !receipt.Resolved {
			failed = append(failed, room.ID)
		} else {
			resolved = append(resolved, room.ID)
		}
	}
	_ = json.NewEncoder(out).Encode(map[string]any{"resolved": resolved, "failed": failed, "cleanup": "requested; inspect maintenance status for completion"})
	if len(failed) > 0 {
		return 1
	}
	return 0
}
