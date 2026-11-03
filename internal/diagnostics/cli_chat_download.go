package diagnostics

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
)

var attachmentIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,160}$`)

// Download through the same authenticated owner endpoint as Inbox. Never echo
// response bodies, overwrite existing files or save partial responses as results.
func runChatDownload(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) == 0 || !attachmentIDPattern.MatchString(args[0]) {
		fmt.Fprintln(errOut, "Usage: cuan chat download ATTACHMENT_ID --output PATH")
		return 2
	}
	flags := flag.NewFlagSet("chat download", flag.ContinueOnError)
	flags.SetOutput(errOut)
	destination := flags.String("output", "", "Save attachment to a new local file")
	if err := flags.Parse(args[1:]); err != nil || *destination == "" || flags.NArg() != 0 {
		fmt.Fprintln(errOut, "Specify --output PATH for the downloaded attachment.")
		return 2
	}
	client := GetCuanClient()
	if client == nil || !client.IsConfigured() {
		fmt.Fprintln(errOut, "Configure the Cuan connection first.")
		return 1
	}
	response, err := client.doRequest(ctx, http.MethodGet, "/api/v1/chat/attachments/"+url.PathEscape(args[0]), nil)
	if err != nil {
		fmt.Fprintln(errOut, "Could not download the attachment.")
		return 1
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		fmt.Fprintf(errOut, "Attachment download failed (HTTP %d).\n", response.StatusCode)
		return 1
	}
	file, err := os.CreateTemp(filepath.Dir(*destination), ".cuan-attachment-*")
	if err != nil {
		fmt.Fprintln(errOut, "Could not create the local output file.")
		return 1
	}
	defer os.Remove(file.Name())
	defer file.Close()
	const limit = 25 << 20
	size, err := io.Copy(file, io.LimitReader(response.Body, limit+1))
	if err != nil || size > limit {
		fmt.Fprintln(errOut, "Attachment was incomplete or exceeded25MB. No result was saved.")
		return 1
	}
	if err = file.Sync(); err == nil {
		err = file.Close()
	}
	if err == nil {
		err = os.Link(file.Name(), *destination)
	}
	if err != nil {
		fmt.Fprintln(errOut, "Could not save attachment. Choose a new writable output path; existing files are preserved.")
		return 1
	}
	fmt.Fprintln(out, *destination)
	return 0
}
