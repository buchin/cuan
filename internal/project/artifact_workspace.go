package project

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// artifactWorkspace is separate from every registered project checkout. It uses
// only an application task identity and retains that task's own outputs on retry.
func (s *Service) artifactWorkspace(taskID string) (string, error) {
	if strings.TrimSpace(taskID) == "" {
		return "", fmt.Errorf("Task identity is required")
	}
	base, err := filepath.Abs(filepath.Join(s.baseDir, "artifact-workspaces"))
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(base, 0700); err != nil {
		return "", err
	}
	info, err := os.Lstat(base)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("Artifact workspace root is unavailable")
	}
	path := filepath.Join(base, fmt.Sprintf("%x", sha256.Sum256([]byte(taskID))))
	if err = os.Mkdir(path, 0700); err != nil && !os.IsExist(err) {
		return "", err
	}
	info, err = os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("Artifact workspace is unavailable")
	}
	return path, nil
}
