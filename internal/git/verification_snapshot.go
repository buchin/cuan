package git

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var verificationSnapshotGate = make(chan struct{}, 1)
var literalVerificationPath = regexp.MustCompile(`^[A-Za-z0-9_./-]+$`)
var snapshotObjectID = regexp.MustCompile(`^[a-f0-9]{40}$`)
var ErrVerificationCommitUnavailable = errors.New("verification source or target commit is unavailable in the project cache")

type verificationSnapshotBinding struct {
	Source       string `json:"source"`
	SourceCommit string `json:"source_commit"`
	TargetCommit string `json:"target_commit"`
}

type snapshotOutput struct {
	data     []byte
	overflow bool
}

func (o *snapshotOutput) Write(data []byte) (int, error) {
	n := len(data)
	remaining := 64*1024 - len(o.data)
	if len(data) > remaining {
		o.overflow = true
		data = data[:remaining]
	}
	o.data = append(o.data, data...)
	return n, nil
}

func verificationGitEnvironment() []string {
	var environment []string
	for _, key := range []string{"PATH", "LANG", "LC_ALL", "LC_CTYPE", "TMPDIR", "TMP", "TEMP", "LD_LIBRARY_PATH", "SystemRoot"} {
		if value, ok := os.LookupEnv(key); ok {
			environment = append(environment, key+"="+value)
		}
	}
	return append(environment, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_REPLACE_OBJECTS=1", "GIT_GRAFT_FILE=/dev/null")
}

func (c *Client) snapshotGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	command := []string{"-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false"}
	if root != "" {
		command = append(command, "-C", root)
	}
	cmd := exec.CommandContext(ctx, c.gitBinary, append(command, args...)...)
	cmd.Env = verificationGitEnvironment()
	var output snapshotOutput
	cmd.Stdout = &output
	err := cmd.Run()
	if output.overflow {
		return nil, errors.New("verification snapshot output exceeded its inspection budget")
	}
	return output.data, err
}

// Resolve the nearest existing parent before creating anything. A missing
// destination below a symlink must not create directories inside the source.
func snapshotDestination(source, destination string) (string, error) {
	parent, suffix := filepath.Dir(destination), ""
	for {
		if _, err := os.Lstat(parent); os.IsNotExist(err) {
			suffix = filepath.Join(filepath.Base(parent), suffix)
			next := filepath.Dir(parent)
			if next == parent {
				return "", errors.New("verification snapshot parent is unavailable")
			}
			parent = next
			continue
		} else if err != nil {
			return "", err
		}
		real, err := filepath.EvalSymlinks(parent)
		if err != nil {
			return "", err
		}
		result := filepath.Join(real, suffix, filepath.Base(destination))
		if result == source || strings.HasPrefix(result, source+string(filepath.Separator)) {
			return "", errors.New("verification snapshot must be separate from its source")
		}
		return result, nil
	}
}

// PrepareVerificationSnapshot creates a separate detached local clone. It never
// checks out, resets, cleans, commits or changes refs/files in the source repo.
// The caller selects a separate owned audit directory. Existing destinations
// are validated, never repaired or replaced.
// Source commits must already be present; credentialed fetching belongs to the
// project's normal repository connection, outside this local snapshot operation.
func (c *Client) PrepareVerificationSnapshot(ctx context.Context, source, destination, sourceCommit, targetCommit string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if !snapshotObjectID.MatchString(sourceCommit) || !snapshotObjectID.MatchString(targetCommit) || source == "" || destination == "" {
		return "", errors.New("verification snapshot requires exact source and target commits")
	}
	source, err := filepath.EvalSymlinks(source)
	if err != nil {
		return "", errors.New("verification source repository is unavailable")
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return "", err
	}
	destination, err = filepath.Abs(destination)
	if err != nil || destination == source || strings.HasPrefix(destination, source+string(filepath.Separator)) {
		return "", errors.New("verification snapshot must be separate from its source")
	}
	realDestination, err := snapshotDestination(source, destination)
	if err != nil {
		return "", err
	}
	select {
	case verificationSnapshotGate <- struct{}{}:
		defer func() { <-verificationSnapshotGate }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	for _, commit := range []string{sourceCommit, targetCommit} {
		if _, err := c.snapshotGit(ctx, source, "cat-file", "-e", commit+"^{commit}"); err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "", ErrVerificationCommitUnavailable
		}
	}
	if _, err := c.snapshotGit(ctx, source, "merge-base", "--is-ancestor", sourceCommit, targetCommit); err != nil {
		return "", errors.New("verification target does not contain the task source commit")
	}
	binding := verificationSnapshotBinding{Source: source, SourceCommit: sourceCommit, TargetCommit: targetCommit}
	if info, err := os.Lstat(destination); err == nil {
		if !info.IsDir() {
			return "", errors.New("saved verification snapshot is not an owned directory")
		}
		if err := c.validateVerificationSnapshot(ctx, destination, binding); err != nil {
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
	if err != nil {
		return "", err
	}
	if filepath.Join(realParent, filepath.Base(destination)) != realDestination {
		return "", errors.New("verification snapshot parent changed; existing files were preserved")
	}
	temporary, err := os.MkdirTemp(parent, ".verification-preparing-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temporary)
	distanceRaw, err := c.snapshotGit(ctx, source, "rev-list", "--count", sourceCommit+".."+targetCommit)
	if err != nil {
		return "", errors.New("verification history could not be bounded")
	}
	distance, err := strconv.Atoi(strings.TrimSpace(string(distanceRaw)))
	if err != nil || distance < 0 || distance == int(^uint(0)>>1) {
		return "", errors.New("verification history could not be bounded")
	}
	if _, err := c.snapshotGit(ctx, "", "init", "--quiet", temporary); err != nil {
		return "", errors.New("verification snapshot could not be created")
	}
	// Transfer only the target history needed to reach the source, rather than
	// copying all cache branches, dangling objects and obsolete build artifacts.
	for _, args := range [][]string{{"config", "core.hooksPath", "/dev/null"}, {"fetch", "--no-tags", "--no-write-fetch-head", "--no-auto-maintenance", "--recurse-submodules=no", "--depth=" + strconv.Itoa(distance+1), "--", source, targetCommit}, {"checkout", "--detach", targetCommit}} {
		if _, err := c.snapshotGit(ctx, temporary, args...); err != nil {
			return "", errors.New("verification snapshot could not be prepared")
		}
	}
	raw, _ := json.Marshal(binding)
	receipt, err := os.OpenFile(filepath.Join(temporary, ".git", "cuan-verification.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	if _, err = receipt.Write(raw); err == nil {
		err = receipt.Sync()
	}
	closeErr := receipt.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if err := c.validateVerificationSnapshot(ctx, temporary, binding); err != nil {
		return "", err
	}
	return c.publishVerificationSnapshot(ctx, temporary, destination, binding)
}

func (c *Client) publishVerificationSnapshot(ctx context.Context, temporary, destination string, binding verificationSnapshotBinding) (string, error) {
	if err := os.Rename(temporary, destination); err != nil {
		// API and chat workers have independent process-local gates. Another
		// process may have published the identical snapshot while we prepared
		// ours. Reuse only a fully verified winner; never replace existing work.
		if info, statErr := os.Lstat(destination); statErr == nil && info.IsDir() && c.validateVerificationSnapshot(ctx, destination, binding) == nil {
			return destination, nil
		}
		return "", errors.New("verification snapshot could not be published; existing files were preserved")
	}
	return destination, nil
}

func (c *Client) validateVerificationSnapshot(ctx context.Context, root string, expected verificationSnapshotBinding) error {
	bad := errors.New("saved verification snapshot changed or could not be verified; existing files were preserved")
	info, err := os.Lstat(filepath.Join(root, ".git"))
	if err != nil || !info.IsDir() {
		return bad
	}
	if _, err := os.Lstat(filepath.Join(root, ".git", "objects", "info", "alternates")); !os.IsNotExist(err) {
		return bad
	}
	path := filepath.Join(root, ".git", "cuan-verification.json")
	info, err = os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return bad
	}
	raw, err := os.ReadFile(path)
	var binding verificationSnapshotBinding
	if err != nil || json.Unmarshal(raw, &binding) != nil || binding != expected {
		return bad
	}
	if c.VerifySnapshotCommit(ctx, root, expected.SourceCommit, expected.TargetCommit) != nil {
		return bad
	}
	return nil
}

// VerifySnapshotCommit checks actual code/ancestry without repairing the clone.
func (c *Client) VerifySnapshotCommit(ctx context.Context, root, sourceCommit, targetCommit string) error {
	bad := errors.New("verification snapshot commit, ancestry or clean state was not verified")
	if !snapshotObjectID.MatchString(sourceCommit) || !snapshotObjectID.MatchString(targetCommit) {
		return bad
	}
	head, err := c.snapshotGit(ctx, root, "rev-parse", "--verify", "HEAD")
	if err != nil || strings.TrimSpace(string(head)) != targetCommit {
		return bad
	}
	if _, err := c.snapshotGit(ctx, root, "merge-base", "--is-ancestor", sourceCommit, targetCommit); err != nil {
		return bad
	}
	status, err := c.snapshotGit(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil || len(status) != 0 || ValidatePrivateBuildOutput(root) != nil {
		return bad
	}
	return nil
}

// VerificationPathsChanged checks literal paths only, against two immutable
// commits in a clean snapshot. A deployment alone is not a reason to retry work.
func (c *Client) VerificationPathsChanged(parent context.Context, root, source, target string, paths []string) (bool, error) {
	if len(paths) == 0 || len(paths) > 16 {
		return false, errors.New("review paths unavailable")
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	if err := c.VerifySnapshotCommit(ctx, root, source, target); err != nil {
		return false, err
	}
	return c.CommittedPathsChanged(ctx, root, source, target, paths)
}

// CommitContained reads real commit ancestry, ignoring replace refs and grafts.
// It never requires or changes the working tree, index, HEAD or refs.
func (c *Client) CommitContained(parent context.Context, root, source, target string) (bool, error) {
	if !snapshotObjectID.MatchString(source) || !snapshotObjectID.MatchString(target) {
		return false, errors.New("publication commit identity unavailable")
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	for _, sha := range []string{source, target} {
		if _, err := c.snapshotGit(ctx, root, "cat-file", "-e", sha+"^{commit}"); err != nil {
			return false, ErrVerificationCommitUnavailable
		}
	}
	_, err := c.snapshotGit(ctx, root, "merge-base", "--is-ancestor", source, target)
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, errors.New("publication ancestry could not be verified")
}

// CommittedPathsChanged reads immutable objects without requiring or changing
// the project cache's working tree, index, HEAD or refs. It creates no snapshot.
func (c *Client) CommittedPathsChanged(parent context.Context, root, source, target string, paths []string) (bool, error) {
	if !snapshotObjectID.MatchString(source) || !snapshotObjectID.MatchString(target) || len(paths) == 0 || len(paths) > 16 {
		return false, errors.New("review source identity unavailable")
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	for _, sha := range []string{source, target} {
		if _, err := c.snapshotGit(ctx, root, "cat-file", "-e", sha+"^{commit}"); err != nil {
			return false, ErrVerificationCommitUnavailable
		}
	}
	if _, err := c.snapshotGit(ctx, root, "merge-base", "--is-ancestor", source, target); err != nil {
		return false, errors.New("reviewed revision is not an ancestor of the changed result")
	}
	args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-only", "-z", source, target, "--"}
	for _, name := range paths {
		if name == "" || filepath.IsAbs(name) || filepath.Clean(name) != name || strings.HasPrefix(name, "../") || !literalVerificationPath.MatchString(name) {
			return false, errors.New("invalid literal review path")
		}
		args = append(args, name)
	}
	output, err := c.snapshotGit(ctx, root, args...)
	if err != nil {
		return false, err
	}
	return len(output) > 0, nil
}
