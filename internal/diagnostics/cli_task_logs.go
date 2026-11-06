package diagnostics

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
)

var taskLogID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,200}$`)

// Logs require an explicit attempt so a later launch cannot silently replace it.
func runTaskLogs(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) < 2 || !taskLogID.MatchString(args[0]) || !taskLogID.MatchString(args[1]) {
		fmt.Fprintln(errOut, "Usage: cuan task logs <task-id> <attempt-id> [--after-id 0 | --tail] [--limit 100] [--json]")
		return 2
	}
	fs := flag.NewFlagSet("task logs", flag.ContinueOnError)
	fs.SetOutput(errOut)
	tail := fs.Bool("tail", false, "Return the latest records in chronological order; cannot combine with after-id")
	after := fs.Int64("after-id", 0, "Only return logs after this record ID")
	limit := fs.Int("limit", 100, "Maximum records (1-1000)")
	asJSON := fs.Bool("json", false, "Output structured log records")
	if err := fs.Parse(args[2:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 || (*tail && *after != 0) || *after < 0 || *limit < 1 || *limit > 1000 {
		fmt.Fprintln(errOut, "Invalid log range: use after-id or tail, and limit must be 1-1000.")
		return 2
	}
	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		fmt.Fprintln(errOut, "Configure the Cuan connection first.")
		return 1
	}
	path := fmt.Sprintf("/api/v1/tasks/%s/attempts/%s/logs?after_id=%d&limit=%d", url.PathEscape(args[0]), url.PathEscape(args[1]), *after, *limit)
	if *tail {
		path += "&tail=true"
	}
	response, err := client.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		fmt.Fprintln(errOut, "Could not retrieve attempt logs.")
		return 1
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		fmt.Fprintf(errOut, "Could not retrieve attempt logs (HTTP %d).\n", response.StatusCode)
		return 1
	}
	// Bound the response before decoding; failed or oversized responses produce no log output.
	const maxBytes = 8 << 20
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil || len(payload) > maxBytes {
		fmt.Fprintln(errOut, "Attempt log response was incomplete or too large.")
		return 1
	}
	var result struct {
		Logs []struct {
			ID       int64  `json:"id"`
			Stream   string `json:"stream"`
			Line     string `json:"line"`
			LoggedAt string `json:"logged_at"`
		} `json:"logs"`
		Total int `json:"total"`
	}
	if json.Unmarshal(payload, &result) != nil {
		fmt.Fprintln(errOut, "Invalid attempt log response.")
		return 1
	}
	if *asJSON {
		if json.NewEncoder(out).Encode(result) != nil {
			return 1
		}
	} else {
		for _, record := range result.Logs {
			fmt.Fprintf(out, "%d [%s] %s\n", record.ID, record.Stream, record.Line)
		}
	}
	return 0
}
