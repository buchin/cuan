package diagnostics

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RunTasteCLI manages only the authenticated member's personal preferences.
func RunTasteCLI(args []string, out, errOut io.Writer) int {
	const help = "Usage: cuan taste show | cuan taste config | cuan taste add --category brand|technical --topic TITLE --content TEXT\nPreferences belong to the authenticated member; they are not shared with teammates."
	if len(args) == 0 || args[0] == "--help" {
		fmt.Fprintln(out, help)
		return 0
	}
	fs := flag.NewFlagSet("taste", flag.ContinueOnError)
	fs.SetOutput(errOut)
	category := fs.String("category", "brand", "Preference category")
	topic := fs.String("topic", "", "Preference title")
	content := fs.String("content", "", "Explicit preference")
	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	method, path := http.MethodGet, "/api/v1/taste"
	var body any
	switch args[0] {
	case "show":
		if fs.NFlag() != 0 || fs.NArg() != 0 {
			fmt.Fprintln(errOut, help)
			return 2
		}
	case "config":
		if fs.NFlag() != 0 || fs.NArg() != 0 {
			fmt.Fprintln(errOut, help)
			return 2
		}
		path += "/config"
	case "add":
		if fs.NArg() != 0 || strings.TrimSpace(*content) == "" || (*category != "brand" && *category != "technical") {
			fmt.Fprintln(errOut, help)
			return 2
		}
		method, path = http.MethodPost, path+"/items"
		body = map[string]string{"category": *category, "topic": *topic, "content": *content}
	default:
		fmt.Fprintln(errOut, help)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	response, err := GetCuanClient().doRequest(ctx, method, path, body)
	if err != nil {
		fmt.Fprintln(errOut, "Preference request failed.")
		return 1
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		fmt.Fprintf(errOut, "Preference request failed (HTTP %d).\n", response.StatusCode)
		return 1
	}
	var result json.RawMessage
	if json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result) != nil {
		fmt.Fprintln(errOut, "Invalid preference response.")
		return 1
	}
	if json.NewEncoder(out).Encode(result) != nil {
		return 1
	}
	return 0
}
