package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/buchin/cuan/internal/executionpolicy"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
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
	_, reconstructionRecord := s.reconstructionPaths(key)
	if _, e := os.Lstat(reconstructionRecord); e == nil {
		work, e := s.reconstructedWorkspace(ctx, taskID, source, key)
		if e != nil {
			return "", e
		}
		if e = syncWorkspaceEnvironment(work, SettingsFromMetadata(p.Metadata).Environment); e != nil {
			return "", e
		}
		return work, nil
	} else if !os.IsNotExist(e) {
		return "", e
	}
	root := filepath.Join(s.baseDir, "task-workspaces")
	if err = os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	relativeDestination := filepath.Join(root, key)
	destination, err := filepath.Abs(relativeDestination)
	if err != nil {
		return "", err
	}
	// Git resolves a relative worktree destination from the repository passed
	// with -C, not from Cuan's working directory. Reuse the exact legacy path
	// when an older release created it there; the checks below still verify
	// its repository and task branch before accepting retained files.
	if !filepath.IsAbs(relativeDestination) {
		if _, e := os.Lstat(destination); os.IsNotExist(e) {
			legacy, e := filepath.Abs(filepath.Join(source, relativeDestination))
			if e == nil {
				if _, e := os.Lstat(legacy); e == nil {
					destination = legacy
				}
			}
		}
	}
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
		preflightWorkspaceRebase(ctx, destination, source, p.DefaultBranch)
		settings := SettingsFromMetadata(p.Metadata)
		s.configureWorkspaceDeployBranch(ctx, destination, p)
		_ = syncWorkspaceEnvironment(destination, settings.Environment)
		return destination, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	base := "refs/remotes/origin/" + p.DefaultBranch
	if _, err = run("-C", source, "rev-parse", "--verify", base+"^{commit}"); err != nil {
		if _, localErr := run("-C", source, "rev-parse", "--verify", "refs/heads/"+p.DefaultBranch+"^{commit}"); localErr == nil {
			base = "refs/heads/" + p.DefaultBranch
		} else {
			// Repair an empty/stale cache before reserving any execution attempt. Existing
			// worktrees return above and never fetch or change their original base.
			current, e := s.store.GetProject(ctx, p.ID)
			if e != nil {
				return "", errors.New("Project connection is unavailable. Your work is saved.")
			}
			token, e := s.repositoryToken(ctx, current.EncryptedToken)
			if e != nil {
				return "", e
			}
			fetch, cancel := context.WithTimeout(ctx, 20*time.Second)
			e = s.gitClient.FetchBoundBranch(fetch, source, p.RepoURL, p.DefaultBranch, token)
			cancel()
			if e != nil {
				if _, fallbackErr := run("-C", source, "rev-parse", "--verify", "HEAD^{commit}"); fallbackErr == nil {
					base = "HEAD"
				} else {
					return "", e
				}
			} else if _, e = run("-C", source, "rev-parse", "--verify", base+"^{commit}"); e != nil {
				if _, fallbackErr := run("-C", source, "rev-parse", "--verify", "HEAD^{commit}"); fallbackErr == nil {
					base = "HEAD"
				} else {
					return "", errors.New("Project files could not be prepared. Your request is saved.")
				}
			}
		}
	}
	settings := SettingsFromMetadata(p.Metadata)
	if _, err = run("-C", source, "show-ref", "--verify", "refs/heads/"+branch); err == nil {
		if err := s.restoreRetiredWorkspace(ctx, key, taskID, source, branch, destination); err != nil {
			return "", err
		}
		_ = syncWorkspaceEnvironment(destination, settings.Environment)
		return destination, nil
	}
	if _, err = run("-C", source, "worktree", "add", "-b", branch, destination, base); err != nil {
		return "", errors.New("Could not prepare the isolated task workspace")
	}
	s.configureWorkspaceDeployBranch(ctx, destination, p)
	_ = syncWorkspaceEnvironment(destination, settings.Environment)
	return destination, nil
}

// TaskExecutionWorkspace preserves pre-isolation task bindings. Only newly
// snapshotted tasks opt in, so an upgrade cannot abandon older pending edits.
func (s *Service) TaskExecutionWorkspace(ctx context.Context, taskID, instructions string, p *ProjectResponse) (string, error) {
	if executionpolicy.StandaloneArtifactWork(instructions) {
		return s.artifactWorkspace(taskID)
	}
	if p == nil {
		return "", errors.New("Project workspace is unavailable")
	}
	at, markerLen := findSnapshotMarker(instructions)
	if at < 0 {
		return p.LocalPath, nil
	}
	var snapshot taskProjectSnapshot
	if json.Unmarshal([]byte(snapshotBody(instructions, at, markerLen)), &snapshot) != nil {
		return "", errors.New("Task project context is invalid")
	}
	if !snapshot.IsolatedRepository {
		return p.LocalPath, nil
	}
	return s.ExecutionWorkspace(ctx, taskID, p)
}

// ProjectTerminalWorkspace prepares a dedicated, isolated Git worktree for interactive project terminal sessions.
// Uncommitted edits, scratch scripts, and local work are preserved and isolated from runner tasks.
func (s *Service) ProjectTerminalWorkspace(ctx context.Context, projectID string, p *ProjectResponse) (string, error) {
	if p == nil || p.LocalPath == "" {
		return "", errors.New("Project workspace is unavailable")
	}
	if p.RepoURL == "" {
		return p.LocalPath, nil
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return "", errors.New("Project identity is required")
	}

	executionWorkspaceMu.Lock()
	defer executionWorkspaceMu.Unlock()

	source, err := filepath.EvalSymlinks(p.LocalPath)
	if err != nil {
		return "", errors.New("Project repository is unavailable")
	}

	digest := sha256.Sum256([]byte("terminal:" + projectID + "\x00" + source))
	key := hex.EncodeToString(digest[:])

	root := filepath.Join(s.baseDir, "terminal-workspaces")
	if err = os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	destination, err := filepath.Abs(filepath.Join(root, key))
	if err != nil {
		return "", err
	}

	branch := "cuan/terminal-" + projectID
	run := func(args ...string) (string, error) {
		out, err := exec.CommandContext(ctx, "git", args...).Output()
		return strings.TrimSpace(string(out)), err
	}

	// 1. If worktree directory already exists, verify its integrity and reuse it directly
	if _, err = os.Lstat(destination); err == nil {
		info, e := os.Lstat(destination)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("Project terminal workspace is invalid")
		}
		common, e := run("-C", destination, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if e != nil {
			return "", errors.New("Project terminal workspace Git state is unavailable")
		}
		expected, e := run("-C", source, "rev-parse", "--path-format=absolute", "--git-common-dir")
		if e != nil || common != expected {
			return "", errors.New("Project terminal workspace belongs to another repository")
		}
		current, e := run("-C", destination, "symbolic-ref", "--short", "HEAD")
		if e == nil && current == branch {
			settings := SettingsFromMetadata(p.Metadata)
			s.configureWorkspaceDeployBranch(ctx, destination, p)
			_ = syncWorkspaceEnvironment(destination, settings.Environment)
			return destination, nil
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}

	// 2. Determine base commit
	base := "refs/remotes/origin/" + p.DefaultBranch
	if _, err = run("-C", source, "rev-parse", "--verify", base+"^{commit}"); err != nil {
		if _, localErr := run("-C", source, "rev-parse", "--verify", "refs/heads/"+p.DefaultBranch+"^{commit}"); localErr == nil {
			base = "refs/heads/" + p.DefaultBranch
		} else {
			base = "HEAD"
		}
	}

	// 3. Add worktree
	if _, err = run("-C", source, "show-ref", "--verify", "refs/heads/"+branch); err == nil {
		if _, addErr := run("-C", source, "worktree", "add", destination, branch); addErr != nil {
			_, _ = run("-C", source, "worktree", "prune")
			if _, retryErr := run("-C", source, "worktree", "add", destination, branch); retryErr != nil {
				return "", fmt.Errorf("could not attach project terminal worktree: %w", retryErr)
			}
		}
	} else {
		if _, addErr := run("-C", source, "worktree", "add", "-b", branch, destination, base); addErr != nil {
			return "", fmt.Errorf("could not prepare isolated project terminal workspace: %w", addErr)
		}
	}

	// Configure upstream tracking to origin/<defaultBranch>
	if p.DefaultBranch != "" {
		_, _ = run("-C", destination, "branch", "--set-upstream-to=origin/"+p.DefaultBranch)
	}

	settings := SettingsFromMetadata(p.Metadata)
	s.configureWorkspaceDeployBranch(ctx, destination, p)
	_ = syncWorkspaceEnvironment(destination, settings.Environment)
	return destination, nil
}

func preflightWorkspaceRebase(ctx context.Context, destination, source, defaultBranch string) {
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	out, err := exec.CommandContext(ctx, "git", "-C", destination, "status", "--porcelain").Output()
	if err != nil || len(strings.TrimSpace(string(out))) > 0 {
		return
	}
	base := "refs/remotes/origin/" + defaultBranch
	if _, err := exec.CommandContext(ctx, "git", "-C", source, "rev-parse", "--verify", base+"^{commit}").Output(); err == nil {
		_ = exec.CommandContext(ctx, "git", "-C", destination, "rebase", base).Run()
	} else if _, err := exec.CommandContext(ctx, "git", "-C", source, "rev-parse", "--verify", "refs/heads/"+defaultBranch+"^{commit}").Output(); err == nil {
		_ = exec.CommandContext(ctx, "git", "-C", destination, "rebase", "refs/heads/"+defaultBranch).Run()
	}
}

func (s *Service) configureWorkspaceDeployBranch(ctx context.Context, destination string, p *ProjectResponse) {
	if p == nil {
		return
	}
	settings := SettingsFromMetadata(p.Metadata)
	deployBranch := strings.TrimSpace(settings.DeployBranch)
	if deployBranch == "" {
		deployBranch = strings.TrimSpace(p.DefaultBranch)
	}
	if deployBranch != "" {
		_ = exec.CommandContext(ctx, "git", "-C", destination, "config", "cuan.deployBranch", deployBranch).Run()
	}
}

func syncWorkspaceEnvironment(destDir string, envText string) error {
	if strings.TrimSpace(envText) == "" {
		return nil
	}
	envPath := filepath.Join(destDir, ".env")
	envExamplePath := filepath.Join(destDir, ".env.example")

	parsedNew := ParseEnvironment(envText)
	if len(parsedNew) == 0 {
		return nil
	}

	// If .env doesn't exist but .env.example exists, seed from .env.example
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		if exampleData, err := os.ReadFile(envExamplePath); err == nil {
			_ = os.WriteFile(envPath, exampleData, 0600)
		}
	}

	var existingContent string
	if data, err := os.ReadFile(envPath); err == nil {
		existingContent = string(data)
	}

	lines := strings.Split(existingContent, "\n")
	updatedKeys := make(map[string]bool)
	var newLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			newLines = append(newLines, line)
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			if newVal, exists := parsedNew[k]; exists {
				if strings.ContainsAny(newVal, " \t\"'") {
					newLines = append(newLines, fmt.Sprintf("%s=%q", k, newVal))
				} else {
					newLines = append(newLines, fmt.Sprintf("%s=%s", k, newVal))
				}
				updatedKeys[k] = true
				continue
			}
		}
		newLines = append(newLines, line)
	}

	for k, v := range parsedNew {
		if !updatedKeys[k] {
			if strings.ContainsAny(v, " \t\"'") {
				newLines = append(newLines, fmt.Sprintf("%s=%q", k, v))
			} else {
				newLines = append(newLines, fmt.Sprintf("%s=%s", k, v))
			}
		}
	}

	finalContent := strings.Join(newLines, "\n")
	if !strings.HasSuffix(finalContent, "\n") {
		finalContent += "\n"
	}

	return os.WriteFile(envPath, []byte(finalContent), 0600)
}
