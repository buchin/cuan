package git

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const verificationColdMarker = "cuan-verification-cold.json"

// Called only while holding the persistent verification lease. A cold snapshot
// retains its independent Git objects; restoration never fetches from its source,
// resets a branch, cleans unknown files or changes the original task checkout.
func (c *Client) restoreColdVerificationSnapshot(ctx context.Context, root string) error {
	marker := filepath.Join(root, ".git", verificationColdMarker)
	info, err := os.Lstat(marker)
	if os.IsNotExist(err) {
		return nil
	}
	bad := errors.New("cold verification snapshot could not be restored; existing files were preserved")
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return bad
	}
	data, err := os.ReadFile(marker)
	var cold, original verificationSnapshotBinding
	if err != nil || json.Unmarshal(data, &cold) != nil || !snapshotObjectID.MatchString(cold.SourceCommit) || !snapshotObjectID.MatchString(cold.TargetCommit) {
		return bad
	}
	receipt := filepath.Join(root, ".git", "cuan-verification.json")
	info, err = os.Lstat(receipt)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return bad
	}
	data, err = os.ReadFile(receipt)
	if err != nil || json.Unmarshal(data, &original) != nil || cold != original {
		return bad
	}
	if _, err = os.Lstat(filepath.Join(root, ".git", "objects", "info", "alternates")); !os.IsNotExist(err) {
		return bad
	}
	if c.verifySnapshotCommitState(ctx, root, cold.SourceCommit, cold.TargetCommit) != nil {
		return bad
	}
	status, err := c.snapshotGit(ctx, root, "status", "--porcelain=v1", "--ignored")
	if err != nil || !reconstructibleWorkspaceStatus(root, status) {
		return bad
	}
	sparse, configErr := c.snapshotGit(ctx, root, "config", "--bool", "--get", "core.sparseCheckout")
	if configErr != nil && len(sparse) != 0 {
		return bad
	}
	if strings.TrimSpace(string(sparse)) == "true" {
		// A fully cold tree contains no tracked files. Never overwrite a file
		// someone added while the tree was cold, even if skip-worktree hides it
		// from git status. Unknown ignored files were rejected above too.
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			rel, e := filepath.Rel(root, path)
			if e != nil {
				return e
			}
			if rel == ".git" && entry.IsDir() {
				return filepath.SkipDir
			}
			if entry.IsDir() {
				return nil
			}
			if filepath.ToSlash(rel) == ".agents/mcp_config.json" && reconstructibleWorkspaceStatus(root, []byte("!! .agents/")) {
				return nil
			}
			if filepath.ToSlash(rel) == ".gemini/config/mcp_config.json" && reconstructibleWorkspaceStatus(root, []byte("!! .gemini/")) {
				return nil
			}
			return bad
		})
		if err != nil {
			return bad
		}
		if _, err = c.snapshotGit(ctx, root, "sparse-checkout", "disable"); err != nil {
			return bad
		}
	} else if configErr != nil {
		// git config exits1 for an absent optional setting. Other failures are
		// not evidence that a partially changed snapshot is safe to expose.
		var exit interface{ ExitCode() int }
		if !errors.As(configErr, &exit) || exit.ExitCode() != 1 {
			return bad
		}
	}
	if c.verifySnapshotCommitState(ctx, root, cold.SourceCommit, cold.TargetCommit) != nil {
		return bad
	}
	if err = os.Remove(marker); err != nil {
		return bad
	}
	return nil
}
