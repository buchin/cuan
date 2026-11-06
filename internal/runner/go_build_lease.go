package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Maintenance takes this same inode exclusively. Keep the shared lease for the
// complete execution, including native verification after the model returns.
func acquireGoBuildLease(ctx context.Context, cache string) (*os.File, error) {
	if cache == "" {
		return nil, nil
	}
	f, err := os.OpenFile(filepath.Join(filepath.Dir(cache), ".build-cache.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, errors.New("Go build cache lease must be a regular file")
	}
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_SH|syscall.LOCK_NB)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			f.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			f.Close()
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}
