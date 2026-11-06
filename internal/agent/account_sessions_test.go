package agent

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestTaskSessionsLeaveChatCapacityWithoutOversubscribingAccount(t *testing.T) {
	for _, chatFirst := range []bool{false, true} {
		t.Run(fmt.Sprint(chatFirst), func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CUAN_ACCOUNT_MAX_SESSIONS", "4")
			t.Setenv("CUAN_ACCOUNT_CHAT_RESERVED_SESSIONS", "1")
			a, err := CreateProviderAccount("owner", "fixture")
			if err != nil {
				t.Fatal(err)
			}
			var releaseChat func()
			if chatFirst {
				releaseChat, err = TryLeaseProviderAccount("owner", a.ID)
				if err != nil {
					t.Fatal(err)
				}
				defer releaseChat()
			}
			for i := 0; i < 3; i++ {
				release, err := TryLeaseProviderAccountForTask("owner", a.ID)
				if err != nil {
					t.Fatal("chat consumed task reservation unexpectedly", i, err)
				}
				defer release()
			}
			if release, err := TryLeaseProviderAccountForTask("owner", a.ID); !errors.Is(err, ErrAccountBusy) {
				if release != nil {
					release()
				}
				t.Fatal("tasks consumed interactive reservation", err)
			}
			if !chatFirst {
				releaseChat, err = TryLeaseProviderAccount("owner", a.ID)
				if err != nil {
					t.Fatal("chat blocked by long tasks", err)
				}
				defer releaseChat()
			}
			if release, err := TryLeaseProviderAccount("owner", a.ID); !errors.Is(err, ErrAccountBusy) {
				if release != nil {
					release()
				}
				t.Fatal("account capacity exceeded", err)
			}
		})
	}
}

func TestTaskReservationKeepsSingleSessionAccountsUsable(t *testing.T) {
	t.Setenv("CUAN_ACCOUNT_MAX_SESSIONS", "1")
	t.Setenv("CUAN_ACCOUNT_CHAT_RESERVED_SESSIONS", "1")
	if providerTaskSessionLimit() != 1 {
		t.Fatal("single-session tasks starved")
	}
	t.Setenv("CUAN_ACCOUNT_MAX_SESSIONS", "4")
	t.Setenv("CUAN_ACCOUNT_CHAT_RESERVED_SESSIONS", "0")
	if providerTaskSessionLimit() != 4 {
		t.Fatal("explicit reservation override ignored")
	}
}

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

func TestBackgroundReviewsLeaveCapacityForTasksAndChat(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CUAN_ACCOUNT_MAX_SESSIONS", "4")
	t.Setenv("CUAN_ACCOUNT_CHAT_RESERVED_SESSIONS", "1")
	a, err := CreateProviderAccount("owner", "review-capacity-fixture")
	if err != nil {
		t.Fatal(err)
	}
	review, err := TryLeaseProviderAccountForReview("owner", a.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer review()
	if release, err := TryLeaseProviderAccountForReview("owner", a.ID); !errors.Is(err, ErrAccountBusy) {
		if release != nil {
			release()
		}
		t.Fatal("reviews filled another slot", err)
	}
	for i := 0; i < 2; i++ {
		release, err := TryLeaseProviderAccountForTask("owner", a.ID)
		if err != nil {
			t.Fatal("background review starved tasks", err)
		}
		defer release()
	}
	chat, err := TryLeaseProviderAccount("owner", a.ID)
	if err != nil {
		t.Fatal("review/tasks starved interactive chat", err)
	}
	defer chat()
	if release, err := TryLeaseProviderAccount("owner", a.ID); !errors.Is(err, ErrAccountBusy) {
		if release != nil {
			release()
		}
		t.Fatal("total account limit changed", err)
	}
}

func TestBackgroundReviewUsesFreeSlotAfterTask(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CUAN_ACCOUNT_MAX_SESSIONS", "4")
	t.Setenv("CUAN_ACCOUNT_CHAT_RESERVED_SESSIONS", "1")
	account, err := CreateProviderAccount("owner", "review-after-task")
	if err != nil {
		t.Fatal(err)
	}
	task, err := TryLeaseProviderAccountForTask("owner", account.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer task()
	review, err := TryLeaseProviderAccountForReview("owner", account.ID)
	if err != nil {
		t.Fatal("idle account slots stranded behind first task", err)
	}
	defer review()
	if release, err := TryLeaseProviderAccountForReview("owner", account.ID); !errors.Is(err, ErrAccountBusy) {
		if release != nil {
			release()
		}
		t.Fatal("second review admitted", err)
	}
	secondTask, err := TryLeaseProviderAccountForTask("owner", account.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer secondTask()
	chat, err := TryLeaseProviderAccount("owner", account.ID)
	if err != nil {
		t.Fatal("review consumed reserved chat capacity", err)
	}
	defer chat()
	if release, err := TryLeaseProviderAccountForTask("owner", account.ID); !errors.Is(err, ErrAccountBusy) {
		if release != nil {
			release()
		}
		t.Fatal("account oversubscribed", err)
	}
	review()
	review() // releasing a review remains idempotent and frees both locks.
	next, err := TryLeaseProviderAccountForReview("owner", account.ID)
	if err != nil {
		t.Fatal("review gate leaked", err)
	}
	next()
}

func TestFailedReviewAdmissionDoesNotRetainClassGate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CUAN_ACCOUNT_MAX_SESSIONS", "2")
	t.Setenv("CUAN_ACCOUNT_CHAT_RESERVED_SESSIONS", "1")
	account, err := CreateProviderAccount("owner", "review-admission")
	if err != nil {
		t.Fatal(err)
	}
	task, err := TryLeaseProviderAccountForTask("owner", account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if release, err := TryLeaseProviderAccountForReview("owner", account.ID); !errors.Is(err, ErrAccountBusy) {
		if release != nil {
			release()
		}
		t.Fatal("reserved chat slot used", err)
	}
	task()
	review, err := TryLeaseProviderAccountForReview("owner", account.ID)
	if err != nil {
		t.Fatal("failed admission retained review lock", err)
	}
	defer review()
	chat, err := TryLeaseProviderAccount("owner", account.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer chat()
	if release, err := TryLeaseProviderAccountForReview("owner", account.ID); !errors.Is(err, ErrAccountBusy) {
		if release != nil {
			release()
		}
		t.Fatal("review class limit changed", err)
	}
}

func TestReviewClassGateCrossProcess(t *testing.T) {
	if id := os.Getenv("CUAN_REVIEW_LEASE_FIXTURE"); id != "" {
		release, err := TryLeaseProviderAccountForReview("owner", id)
		if errors.Is(err, ErrAccountBusy) {
			os.Exit(3)
		}
		if err != nil {
			os.Exit(4)
		}
		release()
		os.Exit(0)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CUAN_ACCOUNT_MAX_SESSIONS", "4")
	account, err := CreateProviderAccount("owner", "cross-process-review")
	if err != nil {
		t.Fatal(err)
	}
	task, err := TryLeaseProviderAccountForTask("owner", account.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer task()
	review, err := TryLeaseProviderAccountForReview("owner", account.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer review()
	probe := func(want int) {
		t.Helper()
		cmd := exec.Command(os.Args[0], "-test.run=^TestReviewClassGateCrossProcess$")
		cmd.Env = append(os.Environ(), "CUAN_REVIEW_LEASE_FIXTURE="+account.ID)
		err := cmd.Run()
		got := 0
		if err != nil {
			exit, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatal(err)
			}
			got = exit.ExitCode()
		}
		if got != want {
			t.Fatalf("review process exit=%d want=%d", got, want)
		}
	}
	probe(3)
	review()
	probe(0) // the task still owns slot0, but review may use another idle slot.
}
