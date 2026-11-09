package diagnostics

import (
	"context"
	"errors"
	"os"

	"github.com/buchin/cuan/internal/browserfetch"
)

func configuredChatBrowser() (*browserfetch.Manifest, error) {
	path := os.Getenv("CUAN_CHAT_BROWSER_MANIFEST")
	if path == "" {
		return nil, errors.New("chat browser runtime is not configured")
	}
	return browserfetch.LoadManifest(path)
}

func publicChatTool(name string) bool {
	if name == "web_fetch" || name == "http_request" {
		return true
	}
	if name != "browser_fetch" {
		return false
	}
	_, err := configuredChatBrowser()
	return err == nil
}

// BrowserFetch uses only the application-selected verified anonymous runtime.
// Providers cannot supply executable paths, script expressions or browser flags.
func BrowserFetch(ctx context.Context, rawURL string) (*browserfetch.Result, error) {
	selected, err := configuredChatBrowser()
	if err != nil {
		return nil, err
	}
	return selected.Runtime.Fetch(ctx, rawURL)
}
