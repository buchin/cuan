package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/buchin/cuan/internal/taskprocess"
)

func TestResolvedCopyDiscardPreservesProjectAndChatAttachments(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "project")
	createTestBaseRepo(t, source)
	copies := filepath.Join(root, "workspaces")
	copy := filepath.Join(copies, "task")
	retentionGit(t, source, "clone", "--local", source, copy)
	attachment := filepath.Join(root, "chat-attachment")
	if err := os.WriteFile(attachment, []byte("delivered result"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(attachment, filepath.Join(copy, "external-result")); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(copy, "README.md"), []byte("discarded local edit"), 0600)
	before := retentionGit(t, source, "rev-parse", "HEAD")
	c := NewClient()
	if _, err := c.DiscardResolvedRepoCopy(context.Background(), copies, source); err == nil {
		t.Fatal("project repository was eligible for deletion")
	}
	lock, err := taskprocess.Acquire(filepath.Join(copy, ".git", "cuan-verification.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.DiscardResolvedRepoCopy(context.Background(), copies, copy); err == nil {
		t.Fatal("active snapshot lease ignored")
	}
	lock.Close()
	bytes, err := c.DiscardResolvedRepoCopy(context.Background(), copies, copy)
	if err != nil || bytes <= 0 {
		t.Fatal(bytes, err)
	}
	if _, err := os.Stat(copy); !os.IsNotExist(err) {
		t.Fatal("repo copy not discarded")
	}
	if content, err := os.ReadFile(attachment); err != nil || string(content) != "delivered result" {
		t.Fatal("chat attachment changed", err)
	}
	if retentionGit(t, source, "rev-parse", "HEAD") != before {
		t.Fatal("project source changed")
	}
	if _, err := os.Stat(filepath.Join(copies, "task.tar.gz")); !os.IsNotExist(err) {
		t.Fatal("unrequested archive created")
	}
	if bytes, err := c.DiscardResolvedRepoCopy(context.Background(), copies, copy); err != nil || bytes != 0 {
		t.Fatal("cleanup is not idempotent", bytes, err)
	}
}
