package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
)

var ErrSharedBuildOutput = errors.New("workspace build output is shared; existing files were preserved")

// ValidatePrivateBuildOutput never follows a web/dist symlink. Read-only
// verifiers must refuse shared outputs rather than rendering another source or
// allowing a build to modify another workspace or the active release.
func ValidatePrivateBuildOutput(root string) error {
	if root == "" {
		return nil // No repository assigned; do not inspect the process directory.
	}
	web := filepath.Join(root, "web")
	info, err := os.Lstat(web)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || !info.IsDir() {
		return errors.New("workspace web directory is unavailable or shared")
	}
	info, err = os.Lstat(filepath.Join(web, "dist"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return ErrSharedBuildOutput
	}
	if !info.IsDir() {
		return errors.New("workspace build output is not a directory")
	}
	return nil
}

func resolvedBuildPath(path string) string {
	path, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return filepath.Clean(path)
}

// Only remove an untracked link created by the former dependency linker. Never
// remove its target, tracked links, arbitrary user links or ordinary artifacts.
func (c *Client) isolateBuildOutput(ctx context.Context, root, source string) error {
	err := ValidatePrivateBuildOutput(root)
	if !errors.Is(err, ErrSharedBuildOutput) {
		return err
	}
	path := filepath.Join(root, "web", "dist")
	target, err := os.Readlink(path)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, c.gitBinary, "-C", root, "ls-files", "--error-unmatch", "--", "web/dist")
	if err := cmd.Run(); err == nil {
		return ErrSharedBuildOutput
	} else if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		return ErrSharedBuildOutput
	}
	resolvedTarget := target
	if !filepath.IsAbs(target) {
		resolvedTarget = filepath.Join(filepath.Dir(path), target)
	}
	for _, known := range []string{filepath.Join(source, "web", "dist"), os.Getenv("CUAN_STATIC_DIR"), "/opt/cuan/current/web/dist", "/opt/cuan/data/projects/default/web/dist"} {
		if known != "" && (target == known || resolvedBuildPath(resolvedTarget) == resolvedBuildPath(known)) {
			return os.Remove(path) // Unlink only; the shared target is untouched.
		}
	}
	return ErrSharedBuildOutput
}
