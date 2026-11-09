package git

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type releaseWorkspaceBinding struct {
	Source       string `json:"source"`
	SourceCommit string `json:"source_commit"`
	TargetCommit string `json:"target_commit"`
	Branch       string `json:"branch"`
}

// PrepareReleaseWorkspace creates a private clone on the frozen release branch.
// It transfers immutable source history only; existing work is validated, never
// reset or cleaned. Credentialed remote fetch/merge belongs to the caller.
func (c *Client) PrepareReleaseWorkspace(ctx context.Context, source, destination, sourceCommit, targetCommit, branch string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if branch == "" || strings.HasPrefix(branch, "-") || !snapshotObjectID.MatchString(sourceCommit) || !snapshotObjectID.MatchString(targetCommit) {
		return "", errors.New("release workspace identity unavailable")
	}
	if _, err := c.snapshotGit(ctx, "", "check-ref-format", "--branch", branch); err != nil {
		return "", errors.New("invalid release branch")
	}
	source, err := filepath.EvalSymlinks(source)
	if err != nil {
		return "", errors.New("release source unavailable")
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return "", err
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	realDestination, err := snapshotDestination(source, destination)
	if err != nil {
		return "", err
	}
	binding := releaseWorkspaceBinding{Source: source, SourceCommit: sourceCommit, TargetCommit: targetCommit, Branch: branch}
	validate := func(root string) error { return c.validateReleaseWorkspace(ctx, root, binding) }
	if info, err := os.Lstat(destination); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("release workspace is not a private directory")
		}
		if err := validate(destination); err != nil {
			return "", err
		}
		if err := c.completeReleaseCommitHistory(ctx, source, destination, targetCommit); err != nil {
			return "", err
		}
		return destination, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", err
	}
	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil || filepath.Join(realParent, filepath.Base(destination)) != realDestination {
		return "", errors.New("release workspace parent changed")
	}
	staging, err := os.MkdirTemp(parent, ".release-preparing-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)
	prepared, err := c.PrepareVerificationSnapshot(ctx, source, filepath.Join(staging, "checkout"), sourceCommit, targetCommit)
	if err != nil {
		return "", err
	}
	if _, err := c.snapshotGit(ctx, prepared, "checkout", "-b", branch, targetCommit); err != nil {
		return "", errors.New("release branch preparation failed")
	}
	raw, _ := json.Marshal(binding)
	if err := os.WriteFile(filepath.Join(prepared, ".git", "cuan-release-workspace.json"), raw, 0600); err != nil {
		return "", err
	}
	if err := c.completeReleaseCommitHistory(ctx, source, prepared, targetCommit); err != nil {
		return "", err
	}
	if err := validate(prepared); err != nil {
		return "", err
	}
	if current, err := snapshotDestination(source, destination); err != nil || current != realDestination {
		return "", errors.New("release destination changed")
	}
	if err := os.Rename(prepared, destination); err != nil {
		if info, statErr := os.Lstat(destination); statErr == nil && info.IsDir() && validate(destination) == nil {
			return destination, nil
		}
		return "", errors.New("release workspace publication failed; existing work preserved")
	}
	return destination, nil
}

func (c *Client) validateReleaseWorkspace(ctx context.Context, root string, expected releaseWorkspaceBinding) error {
	bad := errors.New("release workspace changed; existing work preserved")
	receipt := filepath.Join(root, ".git", "cuan-release-workspace.json")
	info, err := os.Lstat(receipt)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return bad
	}
	raw, err := os.ReadFile(receipt)
	var saved releaseWorkspaceBinding
	if err != nil || json.Unmarshal(raw, &saved) != nil || saved != expected {
		return bad
	}
	if err := c.validateVerificationSnapshot(ctx, root, verificationSnapshotBinding{Source: expected.Source, SourceCommit: expected.SourceCommit, TargetCommit: expected.TargetCommit}); err != nil {
		return bad
	}
	branch, err := c.snapshotGit(ctx, root, "symbolic-ref", "--short", "HEAD")
	if err != nil || strings.TrimSpace(string(branch)) != expected.Branch {
		return bad
	}
	return nil
}
