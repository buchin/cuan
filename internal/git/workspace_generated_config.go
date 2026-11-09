package git

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/buchin/cuan/internal/agent"
)

// A clean published checkout may contain Cuan's exact regenerable MCP defaults.
// Never ignore whole directories: extra files or edited settings retain the repo.
func reconstructibleWorkspaceStatus(root string, status []byte) bool {
	if len(bytes.TrimSpace(status)) == 0 {
		return true
	}
	for _, entry := range strings.Split(strings.TrimSuffix(string(status), "\n"), "\n") {
		var relative, expected string
		switch entry {
		case "!! .agents/":
			relative, expected = ".agents", ".agents/mcp_config.json"
		case "!! .gemini/":
			relative, expected = ".gemini", ".gemini/config/mcp_config.json"
		default:
			return false
		}
		found := false
		err := filepath.WalkDir(filepath.Join(root, relative), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if rel != relative && !(relative == ".gemini" && rel == filepath.Join(".gemini", "config")) {
					return fs.ErrInvalid
				}
				return nil
			}
			if filepath.ToSlash(rel) != expected || entry.Type()&os.ModeSymlink != 0 {
				return fs.ErrInvalid
			}
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
				return fs.ErrInvalid
			}
			data, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(data, agent.GeneratedWorkspaceMCPConfig()) {
				return fs.ErrInvalid
			}
			found = true
			return nil
		})
		if err != nil || !found {
			return false
		}
	}
	return true
}
