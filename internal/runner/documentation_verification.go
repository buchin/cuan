package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	cuangit "github.com/buchin/cuan/internal/git"
)

// Only proven Markdown-only diffs take this path. Unknown git state or mixed
// code changes fall back to project verification; task acceptance still applies.
func verifyDocumentationChanges(ctx context.Context, root, branch string) *verificationResult {
	if root == "" || branch == "" {
		return nil
	}
	base := ""
	refs := []string{"refs/remotes/origin/" + branch, "refs/heads/" + branch}
	if strings.HasPrefix(branch, "refs/") || cuangit.IsCommitHash(branch) {
		refs = []string{branch}
	}
	for _, ref := range refs {
		cmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "--end-of-options", ref+"^{commit}")
		cmd.Dir = root
		if output, err := cmd.Output(); err == nil {
			base = strings.TrimSpace(string(output))
			break
		}
	}
	if base == "" {
		return nil
	}
	paths := map[string]bool{}
	for _, args := range [][]string{{"diff", "--no-renames", "--name-only", "-z", base, "--"}, {"ls-files", "--others", "--exclude-standard", "-z"}} {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir = root
		output, err := cmd.Output()
		if err != nil || len(output) > 65536 {
			return nil
		}
		for _, name := range strings.Split(string(output), "\x00") {
			if name != "" {
				paths[name] = true
			}
		}
	}
	if len(paths) == 0 || len(paths) > 100 {
		return nil
	}
	for name := range paths {
		if !strings.EqualFold(filepath.Ext(name), ".md") {
			return nil
		}
	}
	checked, deleted := 0, 0
	for name := range paths {
		path := filepath.Join(root, name)
		info, err := os.Lstat(path)
		if os.IsNotExist(err) {
			deleted++
			continue
		}
		if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
			return &verificationResult{Error: "Documentation verification failed: unreadable or unsafe Markdown file"}
		}
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return &verificationResult{Error: "Documentation verification failed: unresolved file"}
		}
		rel, err := filepath.Rel(root, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return &verificationResult{Error: "Documentation verification failed: file outside workspace"}
		}
		data, err := os.ReadFile(path)
		if err != nil || !utf8.Valid(data) || strings.IndexByte(string(data), 0) >= 0 || len(strings.TrimSpace(string(data))) == 0 {
			return &verificationResult{Error: "Documentation verification failed: empty or invalid text"}
		}
		checked++
	}
	return &verificationResult{Passed: true, Summary: fmt.Sprintf("Markdown-only diff verified: %d readable nonempty UTF-8 file(s), %d deletion(s). No code/config changes detected against %s. Task-specific content and delivery criteria still require evidence.", checked, deleted, branch)}
}
