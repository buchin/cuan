package terminal

import (
	"errors"
	"github.com/buchin/cuan/internal/taskprocess"
	"testing"
)

func TestRetentionLeaseExcludesExecution(t *testing.T) {
	// Exercise the same durable task ownership used by the process supervisor.
	manager := NewManager(ManagerConfig{WorkspacesDir: t.TempDir()})
	path, e := manager.taskLockPath("retention-task")
	if e != nil {
		t.Fatal(e)
	}
	first, e := manager.AcquireTaskExecutionLease("retention-task")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = taskprocess.Acquire(path); !errors.Is(e, taskprocess.ErrActive) {
		t.Fatal("second owner acquired live execution lease", e)
	}
	first.Close()
	second, e := manager.AcquireTaskExecutionLease("retention-task")
	if e != nil {
		t.Fatal("closed lease not released", e)
	}
	second.Close()
}
