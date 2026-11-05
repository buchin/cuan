package agent

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
)

var ErrAccountBusy = errors.New("All configured session slots for this account are in use")

// ProviderAccountSessionLimit permits independent native sessions to share one
// account. They still share provider quota. Limits bound local concurrency only.
func ProviderAccountSessionLimit() int {
	n, err := strconv.Atoi(os.Getenv("CUAN_ACCOUNT_MAX_SESSIONS"))
	if err != nil || n < 1 || n > 16 {
		return 4
	}
	return n
}
func accountLeaseFile(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}
func accountLeaseDirectory(owner, id string) (string, error) {
	profile, err := GetProviderAccount(owner, id)
	if err != nil || profile.Legacy {
		return "", ErrAccountNotFound
	}
	return accountDirectory(id)
}
func releaseAccountFiles(files ...*os.File) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			for _, f := range files {
				syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
				f.Close()
			}
		})
	}
}

// Sessions share the existing gate, then claim one cross-process slot. Keeping
// the old gate path also prevents overlap with old exclusive-lease releases.
func TryLeaseProviderAccount(owner, id string) (func(), error) {
	return tryLeaseProviderAccount(owner, id, false)
}

// Long tasks leave interactive capacity available. Both classes still use the
// same cross-process locks and provider quota; no account limit is increased.
func TryLeaseProviderAccountForTask(owner, id string) (func(), error) {
	return tryLeaseProviderAccount(owner, id, true)
}

func providerTaskSessionLimit() int {
	limit := ProviderAccountSessionLimit()
	reserved := 1
	if raw := os.Getenv("CUAN_ACCOUNT_CHAT_RESERVED_SESSIONS"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
			reserved = n
		}
	}
	if reserved >= limit {
		reserved = limit - 1
	}
	return limit - reserved
}

func tryLeaseProviderAccount(owner, id string, task bool) (func(), error) {
	if id == "" {
		return func() {}, nil
	}
	dir, err := accountLeaseDirectory(owner, id)
	if err != nil {
		return nil, err
	}
	gate, err := accountLeaseFile(filepath.Join(dir, ".execution.lock"))
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(gate.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
		gate.Close()
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return nil, err
		}
		return nil, ErrAccountBusy
	}
	limit := ProviderAccountSessionLimit()
	if task {
		limit = providerTaskSessionLimit()
	}
	for offset := 0; offset < limit; offset++ {
		i := offset
		if !task {
			i = limit - 1 - offset
		}
		slot, err := accountLeaseFile(filepath.Join(dir, ".session-"+strconv.Itoa(i)+".lock"))
		if err != nil {
			releaseAccountFiles(gate)()
			return nil, err
		}
		err = syscall.Flock(int(slot.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return releaseAccountFiles(slot, gate), nil
		}
		slot.Close()
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			releaseAccountFiles(gate)()
			return nil, err
		}
	}
	releaseAccountFiles(gate)()
	return nil, ErrAccountBusy
}

// Backend token refresh remains exclusive: it must not rewrite credentials
// while an active native session is using the profile.
func TryLeaseProviderAccountRefresh(owner, id string) (func(), error) {
	if id == "" {
		return nil, ErrAccountNotFound
	}
	dir, err := accountLeaseDirectory(owner, id)
	if err != nil {
		return nil, err
	}
	gate, err := accountLeaseFile(filepath.Join(dir, ".execution.lock"))
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(gate.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		gate.Close()
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return nil, err
		}
		return nil, ErrAccountBusy
	}
	return releaseAccountFiles(gate), nil
}
