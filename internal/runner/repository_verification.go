package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/buchin/cuan/internal/executionpolicy"
	"github.com/buchin/cuan/internal/git"
)

// RepositoryVerification is runner evidence, not an agent's completion claim.
// Hashes cover HEAD, refs, local Git config, index/worktree differences and
// nonignored untracked files.
type RepositoryVerification struct {
	Baseline string `json:"baseline"`
	Final    string `json:"final"`
}

// ObserveRepositoryScope is shared by native and local terminal execution.
// It reads existing Git state; it never prepares or repairs a repository.
func ObserveRepositoryScope(ctx context.Context, root string) (string, error) {
	if err := git.ValidatePrivateBuildOutput(root); err != nil {
		return "", err
	}
	if _, err := os.Lstat(filepath.Join(root, ".git")); os.IsNotExist(err) {
		return directoryScopeFingerprint(ctx, root)
	}
	return repositoryFingerprint(ctx, root)
}

// Projects without Git still need native proof that an audit preserved files.
// WalkDir never follows symlinks; bounds prevent hashing arbitrary large trees.
func directoryScopeFingerprint(ctx context.Context, root string) (string, error) {
	digest := sha256.New()
	count, remaining := 0, int64(16<<20)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		count++
		if count > 4096 {
			return fmt.Errorf("directory verification budget exceeded")
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		fmt.Fprintf(digest, "%d:%s:%s:", len(relative), relative, info.Mode())
		if entry.IsDir() {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			fmt.Fprintf(digest, "%d:%s", len(target), target)
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported directory entry")
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		fmt.Fprintf(digest, "%d:", info.Size())
		n, copyErr := io.Copy(digest, io.LimitReader(file, remaining+1))
		closeErr := file.Close()
		remaining -= n
		if remaining < 0 {
			return fmt.Errorf("directory verification budget exceeded")
		}
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func (v *RepositoryVerification) Unchanged() bool {
	if v == nil || len(v.Baseline) != 64 || v.Baseline != v.Final {
		return false
	}
	_, err := hex.DecodeString(v.Baseline)
	return err == nil
}

type boundedFingerprintOutput struct {
	data     []byte
	overflow bool
}

func (b *boundedFingerprintOutput) Write(p []byte) (int, error) {
	const maxOutput = 8 << 20
	n := len(p)
	remaining := maxOutput - len(b.data)
	if len(p) > remaining {
		b.overflow = true
		p = p[:remaining]
	}
	b.data = append(b.data, p...)
	return n, nil
}
func fingerprintGit(ctx context.Context, root string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"--no-optional-locks", "-C", root}, args...)...)
	var out boundedFingerprintOutput
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil || out.overflow {
		return nil, fmt.Errorf("repository state unavailable or exceeds verification budget")
	}
	return out.data, nil
}
func repositoryFingerprint(parent context.Context, root string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("repository workspace required")
	}
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	digest := sha256.New()
	hasHead := true
	if _, err := fingerprintGit(ctx, root, "rev-parse", "--verify", "HEAD"); err != nil {
		hasHead = false
	}
	var commandList [][]string
	if hasHead {
		commandList = [][]string{
			{"rev-parse", "--verify", "HEAD"},
			{"for-each-ref", "--format=%(refname)%00%(objectname)"},
			{"config", "--local", "--null", "--list"},
			{"diff", "--no-ext-diff", "--no-textconv", "--binary", "HEAD", "--"},
			{"status", "--porcelain=v1", "-z", "--untracked-files=all"},
		}
	} else {
		commandList = [][]string{
			{"for-each-ref", "--format=%(refname)%00%(objectname)"},
			{"config", "--local", "--null", "--list"},
			{"status", "--porcelain=v1", "-z", "--untracked-files=all"},
		}
	}
	for _, args := range commandList {
		data, err := fingerprintGit(ctx, root, args...)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(digest, "%d:", len(data))
		digest.Write(data)
	}
	untracked, err := fingerprintGit(ctx, root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return "", err
	}
	var total int64
	count := 0
	for _, name := range strings.Split(string(untracked), "\x00") {
		if name == "" {
			continue
		}
		count++
		clean := filepath.Clean(name)
		if count > 4096 || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("untracked verification boundary exceeded")
		}
		path := filepath.Join(root, clean)
		info, err := os.Lstat(path)
		if err != nil {
			return "", fmt.Errorf("untracked state unavailable")
		}
		fmt.Fprintf(digest, "%d:%s:%d:", len(name), name, info.Mode())
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return "", fmt.Errorf("symlink state unavailable")
			}
			fmt.Fprintf(digest, "%d:%s", len(target), target)
			continue
		}
		total += info.Size()
		if !info.Mode().IsRegular() || info.Size() > 32<<20 || total > 64<<20 {
			return "", fmt.Errorf("untracked verification budget exceeded")
		}
		f, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("untracked file unavailable")
		}
		fmt.Fprintf(digest, "%d:", info.Size())
		n, copyErr := io.Copy(digest, io.LimitReader(f, info.Size()+1))
		f.Close()
		if copyErr != nil || n != info.Size() || ctx.Err() != nil {
			return "", fmt.Errorf("untracked file changed during verification")
		}
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
func verifyTaskRepository(ctx context.Context, root, branch string, policy executionpolicy.PublicationPolicy, baseline string, configuredTargets ...string) (verificationResult, *RepositoryVerification) {
	if err := git.ValidatePrivateBuildOutput(root); err != nil {
		return verificationResult{Error: "Repository build output is shared or unavailable; existing files were preserved."}, nil
	}
	if !policy.ReadOnlyRepository {
		var result verificationResult
		if len(configuredTargets) > 0 && strings.TrimSpace(configuredTargets[0]) != "" {
			result = runRegisteredProjectVerification(ctx, root, configuredTargets[0])
		} else {
			result = runProjectVerification(ctx, root, branch)
		}
		if result.Passed && git.ValidatePrivateBuildOutput(root) != nil {
			return verificationResult{Error: "Project verification left a shared build output; existing files were preserved."}, nil
		}
		return result, nil
	}
	final, err := ObserveRepositoryScope(ctx, root)
	evidence := &RepositoryVerification{Baseline: baseline, Final: final}
	if err != nil || !evidence.Unchanged() {
		return verificationResult{Error: "Read-only scope verification failed: repository state changed or could not be verified. Preserve existing work; do not repair unrelated tests or publish changes."}, evidence
	}
	return verificationResult{Passed: true, Summary: "Read-only repository scope verified against the runner baseline: HEAD, tracked/index changes and nonignored untracked files unchanged. Project implementation tests are outside this audit; task report evidence still requires acceptance."}, evidence
}

// Supplemental verification never prepares, checks out or repairs a workspace.
func verifySourceRepository(parent context.Context, root, commit string) error {
	ctx, cancel := context.WithTimeout(parent, 20*time.Second)
	defer cancel()
	if err := git.ValidatePrivateBuildOutput(root); err != nil {
		return err
	}
	head, err := fingerprintGit(ctx, root, "rev-parse", "--verify", "HEAD")
	if err != nil || strings.TrimSpace(string(head)) != commit {
		return fmt.Errorf("verification source commit differs from the approved implementation")
	}
	status, err := fingerprintGit(ctx, root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil || len(status) != 0 {
		return fmt.Errorf("verification requires the unchanged clean implementation workspace; saved work was preserved")
	}
	return nil
}
func verificationDeadline(existing, started time.Time, enabled bool) time.Time {
	if enabled && started.Add(25*time.Minute).Before(existing) {
		return started.Add(25 * time.Minute)
	}
	return existing
}
