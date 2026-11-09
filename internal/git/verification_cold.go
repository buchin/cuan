package git

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
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
		// Only exact tracked source may already be present after an interrupted
		// transition. Preserve unique writes hidden by skip-worktree as well as
		// the unknown ignored files rejected above.
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
			if c.coldTrackedFileMatches(ctx, root, rel, entry) {
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

// An interrupted cold/restore transition may leave some exact source files
// present. Distinguish those from unique writes hidden by skip-worktree rather
// than either blocking forever or overwriting them blindly.
func (c *Client) coldTrackedFileMatches(ctx context.Context, root, relative string, entry fs.DirEntry) bool {
	data, err := c.snapshotGit(ctx, root, "ls-files", "--stage", "-z", "--", filepath.ToSlash(relative))
	if err != nil {
		return false
	}
	parts := strings.Split(string(data), "\x00")
	if len(parts) != 2 || parts[1] != "" {
		return false
	}
	header, path, ok := strings.Cut(parts[0], "\t")
	fields := strings.Fields(header)
	if !ok || path != filepath.ToSlash(relative) || len(fields) != 3 || fields[2] != "0" || !snapshotObjectID.MatchString(fields[1]) {
		return false
	}
	info, err := entry.Info()
	if err != nil {
		return false
	}
	if fields[0] == "120000" && info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(filepath.Join(root, relative))
		if err != nil {
			return false
		}
		digest := sha1.New()
		fmt.Fprintf(digest, "blob %d\x00%s", len(target), target)
		return fmt.Sprintf("%x", digest.Sum(nil)) == fields[1]
	}
	if !info.Mode().IsRegular() || (fields[0] != "100644" && fields[0] != "100755") || (info.Mode().Perm()&0111 != 0) != (fields[0] == "100755") {
		return false
	}
	actual, err := c.snapshotGit(ctx, root, "hash-object", "--no-filters", "--", filepath.ToSlash(relative))
	return err == nil && strings.TrimSpace(string(actual)) == fields[1]
}
