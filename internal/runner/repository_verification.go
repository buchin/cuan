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
)

// RepositoryVerification is runner evidence, not an agent's completion claim.
// Hashes cover HEAD, refs, local Git config, index/worktree differences and
// nonignored untracked files.
type RepositoryVerification struct {
	Baseline string `json:"baseline"`
	Final    string `json:"final"`
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
	for _, args := range [][]string{{"rev-parse", "--verify", "HEAD"}, {"for-each-ref", "--format=%(refname)%00%(objectname)"}, {"config", "--local", "--null", "--list"}, {"diff", "--no-ext-diff", "--no-textconv", "--binary", "HEAD", "--"}, {"status", "--porcelain=v1", "-z", "--untracked-files=all"}} {
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
func verifyTaskRepository(ctx context.Context, root, branch string, policy executionpolicy.PublicationPolicy, baseline string) (verificationResult, *RepositoryVerification) {
	if !policy.ReadOnlyRepository {
		return runProjectVerification(ctx, root, branch), nil
	}
	final, err := repositoryFingerprint(ctx, root)
	evidence := &RepositoryVerification{Baseline: baseline, Final: final}
	if err != nil || !evidence.Unchanged() {
		return verificationResult{Error: "Read-only scope verification failed: repository state changed or could not be verified. Preserve existing work; do not repair unrelated tests or publish changes."}, evidence
	}
	return verificationResult{Passed: true, Summary: "Read-only repository scope verified against the runner baseline: HEAD, tracked/index changes and nonignored untracked files unchanged. Project implementation tests are outside this audit; task report evidence still requires acceptance."}, evidence
}
