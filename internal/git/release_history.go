package git

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Release activation needs complete commit-parent evidence, whereas verifier
// snapshots deliberately keep shallow history. Import only reachable commit
// objects; historical trees/blobs and unrelated refs remain excluded.
func (c *Client) completeReleaseCommitHistory(ctx context.Context, source, root, target string) error {
	shallow, err := c.snapshotGit(ctx, root, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return errors.New("release history state unavailable")
	}
	if strings.TrimSpace(string(shallow)) == "false" {
		_, err = c.snapshotGit(ctx, root, "rev-list", "--count", target)
		return err
	}
	sourceShallow, err := c.snapshotGit(ctx, source, "rev-parse", "--is-shallow-repository")
	if err != nil || strings.TrimSpace(string(sourceShallow)) != "false" {
		return errors.New("release requires complete source commit history")
	}
	file, err := os.CreateTemp(filepath.Join(root, ".git"), ".release-commits-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	command := func(directory string, args ...string) *exec.Cmd {
		all := append([]string{"--no-replace-objects", "-c", "core.hooksPath=/dev/null", "-C", directory}, args...)
		cmd := exec.CommandContext(ctx, c.gitBinary, all...)
		cmd.Env = verificationGitEnvironment()
		return cmd
	}
	list := command(source, "rev-list", target)
	objects, err := list.StdoutPipe()
	if err != nil {
		return err
	}
	defer objects.Close()
	if err = list.Start(); err != nil {
		return err
	}
	pack := command(source, "pack-objects", "--stdout")
	pack.Stdin, pack.Stdout = objects, file
	packErr := pack.Run()
	_ = objects.Close()
	listErr := list.Wait()
	if packErr != nil || listErr != nil {
		return errors.New("complete release commit graph could not be packed")
	}
	info, err := file.Stat()
	if err != nil || info.Size() == 0 || info.Size() > 64<<20 {
		return errors.New("release commit graph exceeds activation budget")
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	index := command(root, "index-pack", "--stdin")
	index.Stdin = file
	if index.Run() != nil {
		return errors.New("complete release commit graph could not be imported")
	}
	boundary := filepath.Join(root, ".git", "shallow")
	info, err = os.Lstat(boundary)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("release shallow boundary changed")
	}
	if err = os.Remove(boundary); err != nil {
		return err
	}
	if _, err = c.snapshotGit(ctx, root, "rev-list", "--count", target); err != nil {
		return errors.New("release commit-parent graph remains incomplete")
	}
	return nil
}
