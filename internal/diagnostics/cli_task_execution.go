package diagnostics

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/buchin/cuan/internal/task"
)

func runTaskExecution(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) < 1 || !taskLogID.MatchString(args[0]) {
		fmt.Fprintln(errOut, "Usage: cuan task execution <task-id> [--attempt <attempt-id>]")
		return 2
	}
	fs := flag.NewFlagSet("task execution", flag.ContinueOnError)
	fs.SetOutput(errOut)
	attempt := fs.String("attempt", "", "Inspect this saved attempt instead of the latest attempt")
	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 || (*attempt != "" && !taskLogID.MatchString(*attempt)) {
		fmt.Fprintln(errOut, "Invalid attempt selection.")
		return 2
	}
	if taskSessionOwnerWriteError() != nil {
		fmt.Fprintln(errOut, "Execution inspection requires the Owner connection.")
		return 1
	}
	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		fmt.Fprintln(errOut, "Configure the Cuan connection first.")
		return 1
	}
	path := "/api/v1/tasks/" + url.PathEscape(args[0]) + "/execution"
	if *attempt != "" {
		path += "?attempt_id=" + url.QueryEscape(*attempt)
	}
	response, err := client.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		fmt.Fprintln(errOut, "Saved execution context could not be read.")
		return 1
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		fmt.Fprintf(errOut, "Execution context unavailable (HTTP %d).\n", response.StatusCode)
		return 1
	}
	var info task.ExecutionInfo
	decoder := json.NewDecoder(io.LimitReader(response.Body, 16<<10))
	if decoder.Decode(&info) != nil || decoder.Decode(new(any)) != io.EOF || info.TaskID != args[0] || info.IntentKind == "" || (*attempt != "" && info.AttemptID != *attempt) {
		fmt.Fprintln(errOut, "Invalid saved execution context.")
		return 1
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if encoder.Encode(info) != nil {
		return 1
	}
	return 0
}
