package git

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
)

// An unpublished source need not exist locally to establish its absence from
// a complete target history. Shallow or broken graphs remain unknown. Stream
// IDs rather than retaining a history-sized buffer; the caller bounds runtime.
func (c *Client) completeTargetContains(ctx context.Context, root, source, target string) (bool, error) {
	shallow, err := c.snapshotGit(ctx, root, "rev-parse", "--is-shallow-repository")
	if err != nil || strings.TrimSpace(string(shallow)) != "false" {
		return false, ErrVerificationCommitUnavailable
	}
	cmd := exec.CommandContext(ctx, c.gitBinary, "-c", "core.hooksPath=/dev/null", "-c", "core.fsmonitor=false", "-c", "credential.helper=", "-C", root, "rev-list", target)
	cmd.Env = append(verificationGitEnvironment(), "GIT_NO_LAZY_FETCH=1")
	output, err := cmd.StdoutPipe()
	if err != nil {
		return false, ErrVerificationCommitUnavailable
	}
	if err := cmd.Start(); err != nil {
		return false, ErrVerificationCommitUnavailable
	}
	scanner := bufio.NewScanner(output)
	contained, seenTarget, invalid := false, false, false
	for scanner.Scan() {
		id := scanner.Text()
		if !snapshotObjectID.MatchString(id) {
			invalid = true
		}
		contained = contained || id == source
		seenTarget = seenTarget || id == target
	}
	scanErr := scanner.Err()
	runErr := cmd.Wait()
	if scanErr != nil || runErr != nil || ctx.Err() != nil || invalid || !seenTarget {
		return false, ErrVerificationCommitUnavailable
	}
	return contained, nil
}
