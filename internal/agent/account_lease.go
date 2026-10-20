package agent

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// Cross-process lease spans an Inbox turn or an entire task session. A backup
// cannot be selected while its account is already executing another operation.
func TryLeaseProviderAccount(owner, id string) (func(), error) {
	if id == "" {
		return func() {}, nil
	}
	profile, err := GetProviderAccount(owner, id)
	if err != nil || profile.Legacy {
		return nil, ErrAccountNotFound
	}
	dir, err := accountDirectory(id)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, ".execution.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New("Account is already in use")
	}
	var once sync.Once
	return func() { once.Do(func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }) }, nil
}
