package server

import (
	"context"
	"testing"
	"time"
)

func TestReviewBudgetDoesNotInheritDirectRecoveryConfiguration(t *testing.T) {
	t.Setenv("INBOX_CLI_DIRECT_RECOVERY_TIMEOUT", "17s")
	t.Setenv("INBOX_REVIEW_TIMEOUT", "")
	review := context.WithValue(context.Background(), inboxTaskReviewKey{}, true)
	if inboxToolFreeBudget(review) != 240*time.Second || inboxToolFreeBudget(context.Background()) != 17*time.Second {
		t.Fatal("review and direct chat share a time budget")
	}
	delivery := context.WithValue(review, inboxCustomerReplyKey{}, true)
	if inboxToolFreeBudget(delivery) != 17*time.Second {
		t.Fatal("customer draft received the longer review budget")
	}
	for _, value := range []string{"-1s", "0s", "bad", "241s", "1h"} {
		t.Setenv("INBOX_REVIEW_TIMEOUT", value)
		if inboxToolFreeBudget(review) != 240*time.Second {
			t.Fatal("invalid configuration bypassed bounded review", value)
		}
	}
	t.Setenv("INBOX_REVIEW_TIMEOUT", "180s")
	if inboxToolFreeBudget(review) != 180*time.Second {
		t.Fatal("review configuration ignored")
	}
}

func TestReviewBudgetKeepsParentDeadlineAndCancellation(t *testing.T) {
	t.Setenv("INBOX_REVIEW_TIMEOUT", "")
	root := context.WithValue(context.Background(), inboxTaskReviewKey{}, true)
	bounded, stop := inboxToolFreeContext(root)
	defer stop()
	deadline, ok := bounded.Deadline()
	if !ok || time.Until(deadline) < 244*time.Second || time.Until(deadline) > 245*time.Second {
		t.Fatal("native review has no bounded cleanup allowance")
	}
	parent, cancel := context.WithTimeout(root, time.Second)
	defer cancel()
	child, finish := inboxToolFreeContext(parent)
	defer finish()
	want, _ := parent.Deadline()
	got, _ := child.Deadline()
	if !got.Equal(want) {
		t.Fatal("review renewed the durable request deadline")
	}
	cancel()
	if child.Err() != context.Canceled {
		t.Fatal("review ignored request cancellation")
	}
}
