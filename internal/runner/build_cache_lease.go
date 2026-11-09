package runner

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Maintenance takes this same inode exclusively. Keep the shared lease for the
// complete execution, including native verification after the model returns.
func acquireBuildCacheLease(ctx context.Context, cache string) (*os.File, error) {
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
		return nil, errors.New("build cache lease must be a regular file")
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

// releaseBuildCacheLease records the accurate completion timestamp of native
// execution into the lease metadata and releases the file descriptor.
// It verifies regular file status, single hardlink, non-symlink path and
// ownership before writing, protecting foreign files and linked structures.
func releaseBuildCacheLease(f *os.File, cache string) error {
	return releaseBuildCacheLeaseTime(f, cache, time.Now())
}

func releaseBuildCacheLeaseTime(f *os.File, cache string, at time.Time) error {
	if f == nil {
		return nil
	}
	defer f.Close()
	if filepath.Base(cache) != "go-build" {
		return nil
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	sys, ok := info.Sys().(*syscall.Stat_t)
	if !ok || sys.Nlink != 1 || sys.Uid != uint32(os.Geteuid()) {
		return nil
	}
	path := f.Name()
	linfo, err := os.Lstat(path)
	if err != nil || linfo.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	lsys, ok := linfo.Sys().(*syscall.Stat_t)
	if !ok || lsys.Ino != sys.Ino || lsys.Dev != sys.Dev || lsys.Nlink != 1 || lsys.Uid != uint32(os.Geteuid()) {
		return nil
	}

	data, err := json.Marshal(map[string]any{
		"version":      1,
		"last_used_at": at.Unix(),
	})
	if err != nil {
		return nil
	}
	if _, err := f.Seek(0, 0); err == nil {
		if err := f.Truncate(0); err == nil {
			_, _ = f.Write(data)
			_ = f.Sync()
		}
	}
	return nil
}
