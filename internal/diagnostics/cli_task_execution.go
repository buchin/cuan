package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/buchin/cuan/internal/task"
)

func runTaskExecution(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) != 1 || !taskLogID.MatchString(args[0]) {
		fmt.Fprintln(errOut, "Usage: cuan task execution <task-id>")
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
	response, err := client.doRequest(ctx, http.MethodGet, "/api/v1/tasks/"+url.PathEscape(args[0])+"/execution", nil)
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
	if decoder.Decode(&info) != nil || decoder.Decode(new(any)) != io.EOF || info.TaskID != args[0] || info.IntentKind == "" {
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
