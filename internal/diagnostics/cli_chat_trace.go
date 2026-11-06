package diagnostics

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/buchin/cuan/internal/db"
)

func runChatTrace(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(errOut, "Usage: cuan chat trace <room-id> <request-id> [--after-seq 0] [--limit 100]")
		return 2
	}
	fs := flag.NewFlagSet("chat trace", flag.ContinueOnError)
	fs.SetOutput(errOut)
	after := fs.Int("after-seq", 0, "Read stages after this sequence")
	limit := fs.Int("limit", 100, "Maximum stages (1–200)")
	if err := fs.Parse(args[2:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *after < 0 || *limit < 1 || *limit > 200 || fs.NArg() != 0 {
		fmt.Fprintln(errOut, "Invalid trace cursor or limit.")
		return 2
	}
	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		fmt.Fprintln(errOut, "Configure the Cuan connection first.")
		return 1
	}
	endpoint := "/api/v1/chat/conversations/" + url.PathEscape(args[0]) + "/requests/" + url.PathEscape(args[1]) + "/steps?after_seq=" + strconv.Itoa(*after) + "&limit=" + strconv.Itoa(*limit)
	res, err := client.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		fmt.Fprintln(errOut, "Could not read response details.")
		return 1
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		fmt.Fprintf(errOut, "Could not read response details (HTTP %d).\n", res.StatusCode)
		return 1
	}
	var page db.InboxStepPage
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&page); err != nil {
		fmt.Fprintln(errOut, "Invalid response details.")
		return 1
	}
	if json.NewEncoder(out).Encode(page) != nil {
		return 1
	}
	return 0
}
