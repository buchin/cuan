package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestVerificationLeaseSerializesMaintenanceAndPreservesInode(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	first, err := AcquireVerificationLease(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	original, _ := first.Stat()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if other, err := AcquireVerificationLease(ctx, root); !errors.Is(err, context.DeadlineExceeded) || other != nil {
		t.Fatal("another reader bypassed the active snapshot lease", err)
	}
	first.Close()
	next, err := AcquireVerificationLease(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	current, _ := next.Stat()
	if !os.SameFile(original, current) {
		t.Fatal("snapshot lease inode was replaced")
	}
}

func TestVerificationLeaseRejectsLinkedAndNonregularMetadata(t *testing.T) {
	for _, name := range []string{"root link", "metadata link", "gate link", "gate directory"} {
		t.Run(name, func(t *testing.T) {
			root, outside := t.TempDir(), t.TempDir()
			if name == "root link" {
				link := filepath.Join(root, "linked")
				os.Symlink(outside, link)
				root = link
			} else if name == "metadata link" {
				os.Symlink(outside, filepath.Join(root, ".git"))
			} else {
				os.Mkdir(filepath.Join(root, ".git"), 0700)
				gate := filepath.Join(root, ".git/cuan-verification.lock")
				if name == "gate link" {
					file := filepath.Join(outside, "preserve")
					os.WriteFile(file, []byte("saved"), 0600)
					os.Symlink(file, gate)
				} else {
					os.Mkdir(gate, 0700)
				}
			}
			lease, err := AcquireVerificationLease(context.Background(), root)
			if err == nil || lease != nil {
				t.Fatal("unsafe snapshot lease accepted")
			}
		})
	}
}
