package db

import "testing"

func TestInboxReconciliationSurvivesSubsequentCapacityFailure(t *testing.T) {
	q := InboxRequest{State: "failed", SafeError: "Unknown result"}
	child := retryRequest(q)
	if !InboxRetryNeedsReconciliation(child) {
		t.Fatal("unknown result replayed")
	}
	child.State = "failed"
	child.SafeError = "Provider usage limit"
	child.Steps = append(child.Steps, InboxStep{Kind: "recovery_safe", Text: "No tool was called"})
	if InboxRecoveryAction(child) != "check_usage" {
		t.Fatal("fixture must be a safe capacity failure")
	}
	grandchild := retryRequest(child)
	if !InboxRetryNeedsReconciliation(grandchild) {
		t.Fatal("retry lost the original uncertainty")
	}
}
