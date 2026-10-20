package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

var executionWorkspaceMu sync.Mutex

// ExecutionWorkspace gives each repository task a stable Git worktree. A resumed
// task reuses its own files and branch instead of starting its implementation again.
func (s *Service) ExecutionWorkspace(ctx context.Context, taskID string, p *ProjectResponse) (string, error) {
	if p == nil || p.LocalPath == "" {
		return "", errors.New("Project workspace is unavailable")
	}
	if p.RepoURL == "" {
		return p.LocalPath, nil
	}
	if strings.TrimSpace(taskID) == "" {
		return "", errors.New("Task identity is required")
	}
	executionWorkspaceMu.Lock()
	defer executionWorkspaceMu.Unlock()
	source, err := filepath.EvalSymlinks(p.LocalPath)
	if err != nil {
		return "", errors.New("Project repository is unavailable")
	}
	digest := sha256.Sum256([]byte(taskID + "\x00" + source))
	key := hex.EncodeToString(digest[:])
	root := filepath.Join(s.baseDir, "task-workspaces")
	if err = os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	destination := filepath.Join(root, key)
	branch := "cuan/task-" + key
	run := func(args ...string) (string, error) {
		out, err := exec.CommandContext(ctx, "git", args...).Output()
		return strings.TrimSpace(string(out)), err
	}
	if _, err = os.Lstat(destination); err == nil {
		// Verify both repository identity and branch before reusing existing files.
		info, e := os.Lstat(destination)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("Task workspace is invalid")
		}
		common, e := run("-C", destination, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if e != nil {
			return "", errors.New("Task workspace Git state is unavailable")
		}
		expected, e := run("-C", source, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if e != nil || common != expected {
			return "", errors.New("Task workspace belongs to another repository")
		}
		current, e := run("-C", destination, "symbolic-ref", "--short", "HEAD")
		if e != nil || current != branch {
			return "", errors.New("Task workspace branch changed; retained work needs review")
		}
		return destination, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	base := "refs/remotes/origin/" + p.DefaultBranch
	if _, err = run("-C", source, "rev-parse", "--verify", base+"^{commit}"); err != nil {
		return "", errors.New("Project default branch is unavailable. Sync the project first.")
	}
	if _, err = run("-C", source, "show-ref", "--verify", "refs/heads/"+branch); err == nil {
		if err := s.restoreRetiredWorkspace(ctx, key, taskID, source, branch, destination); err != nil {
			return "", err
		}
		return destination, nil
	}
	if _, err = run("-C", source, "worktree", "add", "-b", branch, destination, base); err != nil {
		return "", errors.New("Could not prepare the isolated task workspace")
	}
	return destination, nil
}

// TaskExecutionWorkspace preserves pre-isolation task bindings. Only newly
// snapshotted tasks opt in, so an upgrade cannot abandon older pending edits.
func (s *Service) TaskExecutionWorkspace(ctx context.Context, taskID, instructions string, p *ProjectResponse) (string, error) {
	if p == nil {
		return "", errors.New("Project workspace is unavailable")
	}
	at := strings.LastIndex(instructions, snapshotMarker)
	if at < 0 {
		return p.LocalPath, nil
	}
	var snapshot taskProjectSnapshot
	if json.Unmarshal([]byte(strings.TrimSuffix(instructions[at+len(snapshotMarker):], "\n```")), &snapshot) != nil {
		return "", errors.New("Task project context is invalid")
	}
	if !snapshot.IsolatedRepository {
		return p.LocalPath, nil
	}
	return s.ExecutionWorkspace(ctx, taskID, p)
}
