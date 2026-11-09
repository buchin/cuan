package server

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/buchin/cuan/internal/browserfetch"
)

// prepareInboxBrowserMounts selects the service runtime, never the AI account's
// ambient browser. Only a verified optional installation adds MCP capability.
func prepareInboxBrowserMounts(home, mcpFile string) ([]string, error) {
	selected := os.Getenv("CUAN_CHAT_BROWSER_MANIFEST")
	if selected == "" {
		serviceHome, err := os.UserHomeDir()
		if err != nil {
			return nil, nil
		}
		selected = filepath.Join(serviceHome, ".local", "share", "cuan-chat-browser", "runtime.json")
	}
	manifest, err := browserfetch.LoadManifest(selected)
	if err != nil {
		return nil, nil
	} // Optional failure does not disable HTTP/chat.
	projected := *manifest
	projected.Runtime = browserfetch.Runtime{Binary: "/cuan-chat-browser/obscura", Sandbox: "/cuan-chat-browser/bwrap", LockFile: "/cuan-chat-browser/slot.lock"}
	mounts := []string{"--dir", "/cuan-chat-browser", "--ro-bind", manifest.Binary, projected.Binary, "--ro-bind", manifest.Sandbox, projected.Sandbox, "--bind", manifest.LockFile, projected.LockFile}
	if manifest.Loader != "" {
		projected.Loader = "/cuan-chat-browser/ld.so"
		projected.LibraryDir = "/cuan-chat-browser/libraries"
		mounts = append(mounts, "--ro-bind", manifest.Loader, projected.Loader, "--ro-bind", manifest.LibraryDir, projected.LibraryDir)
	}
	raw, err := json.Marshal(projected)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(filepath.Dir(mcpFile), "browser_runtime.json")
	relative, err := filepath.Rel(home, path)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(home)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if err := writeNativeRuntimeFile(root, relative, raw, 0600); err != nil {
		return nil, err
	}
	target := "/cuan-chat-browser/runtime.json"
	mounts = append(mounts, "--ro-bind", path, target, "--setenv", "CUAN_CHAT_BROWSER_MANIFEST", target)
	return mounts, nil
}
