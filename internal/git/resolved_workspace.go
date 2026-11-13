package git

import (
	"context"
	"errors"
	"github.com/buchin/cuan/internal/taskprocess"
	"os"
	"path/filepath"
	"strings"
)

// DiscardResolvedRepoCopy removes a disposable standalone Git checkout, without
// archiving it. The caller must hold task execution ownership and freshly verify
// explicit conversation resolution. base must be a dedicated workspace root,
// never the project repository or chat attachment store. Bytes are logical file
// bytes removed, not a claim about device space (objects may be hardlinked).
func (c *Client) DiscardResolvedRepoCopy(ctx context.Context, base, path string) (int64, error) {
	base, err := filepath.Abs(base)
	if err != nil {
		return 0, err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return 0, err
	}
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return 0, errors.New("resolved copy is outside workspace storage")
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	real, err := filepath.EvalSymlinks(path)
	if err != nil || !info.IsDir() {
		return 0, errors.New("resolved copy is not a private directory")
	}
	parentResolved, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil || real != filepath.Join(parentResolved, filepath.Base(path)) {
		return 0, errors.New("resolved copy is not a private directory")
	}
	metadata, err := os.Lstat(filepath.Join(path, ".git"))
	if err != nil || !metadata.IsDir() || metadata.Mode()&os.ModeSymlink != 0 {
		return 0, errors.New("resolved copy is not a standalone Git checkout")
	}
	lease, err := taskprocess.Acquire(filepath.Join(path, ".git", "cuan-verification.lock"))
	if err != nil {
		return 0, err
	}
	defer lease.Close()
	root, err := c.snapshotGit(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil || !equalSourcePaths(strings.TrimSpace(string(root)), path) {
		return 0, errors.New("resolved copy repository identity is unavailable")
	}
	var bytes int64
	err = filepath.WalkDir(path, func(name string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			bytes += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := os.RemoveAll(path); err != nil {
		return 0, err
	}
	return bytes, nil
}
