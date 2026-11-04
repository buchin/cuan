package runner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVerificationSourceRequiresExactCleanCommit(t *testing.T) {
	root := documentationFixture(t)
	ctx := context.Background()
	head, err := fingerprintGit(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	commit := strings.TrimSpace(string(head))
	if err := verifySourceRepository(ctx, root, commit); err != nil {
		t.Fatal(err)
	}
	if err := verifySourceRepository(ctx, root, strings.Repeat("0", 40)); err == nil {
		t.Fatal("accepted wrong commit")
	}
	if err := os.WriteFile(filepath.Join(root, "saved-work.txt"), []byte("preserve me"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifySourceRepository(ctx, root, commit); err == nil {
		t.Fatal("accepted dirty source")
	}
	if data, err := os.ReadFile(filepath.Join(root, "saved-work.txt")); err != nil || string(data) != "preserve me" {
		t.Fatal("modified saved work")
	}
}
func TestVerificationDeadlineNeverExtendsExecution(t *testing.T) {
	start := time.Now()
	for _, c := range []struct {
		existing time.Duration
		enabled  bool
		want     time.Duration
	}{{time.Hour, true, 10 * time.Minute}, {time.Minute, true, time.Minute}, {time.Hour, false, time.Hour}} {
		if got := verificationDeadline(start.Add(c.existing), start, c.enabled); !got.Equal(start.Add(c.want)) {
			t.Fatalf("deadline %v", got)
		}
	}
}
