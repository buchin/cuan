package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/buchin/cuan/internal/taskprocess"
)

// AcquireVerificationLease coordinates runners with generated-dependency
// maintenance. The inode stays in .git, outside source fingerprints, and must
// never be unlinked: waiting processes must all lock the same inode.
func AcquireVerificationLease(ctx context.Context, root string) (*os.File, error) {
	real, err := filepath.EvalSymlinks(root)
	if err != nil || real != filepath.Clean(root) || !filepath.IsAbs(root) {
		return nil, errors.New("verification lease requires a private snapshot")
	}
	metadata := filepath.Join(root, ".git")
	info, err := os.Lstat(metadata)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("verification lease requires private repository metadata")
	}
	file, err := taskprocess.OpenLock(filepath.Join(metadata, "cuan-verification.lock"))
	if err != nil {
		return nil, err
	}
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		file.Close()
		return nil, errors.New("verification lease is not a regular file")
	}
	for {
		if err := ctx.Err(); err != nil {
			file.Close()
			return nil, err
		}
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			file.Close()
			return nil, err
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			file.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
