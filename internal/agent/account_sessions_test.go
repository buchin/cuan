package agent

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func init() {
	if len(os.Args) > 1 && os.Args[1] == "--test-account-slot" {
		release, e := TryLeaseProviderAccount("owner", os.Args[2])
		if e != nil {
			os.Exit(4)
		}
		defer release()
		os.WriteFile(os.Args[3], []byte("ready"), 0600)
		var b [1]byte
		os.Stdin.Read(b[:])
		os.Exit(0)
	}
}
func TestAccountSessionsBoundedConcurrentAndRefreshExclusive(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CUAN_ACCOUNT_MAX_SESSIONS", "4")
	a, e := CreateProviderAccount("owner", "fixture")
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var releases []func()
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := TryLeaseProviderAccount("owner", a.ID)
			if e == nil {
				mu.Lock()
				releases = append(releases, r)
				mu.Unlock()
			} else if !errors.Is(e, ErrAccountBusy) {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if len(releases) != 4 {
		t.Fatal("wrong session capacity", len(releases))
	}
	if r, e := TryLeaseProviderAccountRefresh("owner", a.ID); e == nil {
		r()
		t.Fatal("refresh overlapped sessions")
	}
	if r, e := TryLeaseProviderAccount("other", a.ID); e == nil {
		r()
		t.Fatal("cross-owner session")
	}
	for _, r := range releases {
		r()
		r()
	}
	refresh, e := TryLeaseProviderAccountRefresh("owner", a.ID)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := TryLeaseProviderAccount("owner", a.ID); e == nil {
		r()
		t.Fatal("session overlapped refresh")
	}
	refresh()
	r, e := TryLeaseProviderAccount("owner", a.ID)
	if e != nil {
		t.Fatal("slots not released", e)
	}
	r()
}
func TestAccountSessionSlotReleasedAfterProcessDeath(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CUAN_ACCOUNT_MAX_SESSIONS", "2")
	a, e := CreateProviderAccount("owner", "fixture")
	if e != nil {
		t.Fatal(e)
	}
	held, e := TryLeaseProviderAccount("owner", a.ID)
	if e != nil {
		t.Fatal(e)
	}
	defer held()
	executable, _ := os.Executable()
	ready := filepath.Join(t.TempDir(), "ready")
	child := exec.Command(executable, "--test-account-slot", a.ID, ready)
	input, e := child.StdinPipe()
	if e != nil {
		t.Fatal(e)
	}
	defer input.Close()
	if e = child.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { child.Process.Kill(); child.Wait() }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, e = os.Stat(ready); e == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child did not claim slot")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if r, e := TryLeaseProviderAccount("owner", a.ID); e == nil {
		r()
		t.Fatal("cross-process cap bypassed")
	}
	child.Process.Kill()
	child.Wait()
	r, e := TryLeaseProviderAccount("owner", a.ID)
	if e != nil {
		t.Fatal("dead process retained slot", e)
	}
	r()
}
