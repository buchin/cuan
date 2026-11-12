package server

import (
	"strings"
	"testing"
)

// agy ends its own turn on a deadline and reports DEADLINE_EXCEEDED with partial
// output. Production treated that as a hard provider timeout and discarded the
// user's message even though Cuan's own context was still alive.
func TestProviderTurnDeadlineIsClassifiedAsRetryableCapacity(t *testing.T) {
	cases := []inboxAgyResult{
		{Status: "ERROR", Reason: "print_timeout", ShortError: "print timeout after 120s with turn in progress"},
		{Status: "ERROR", ShortError: "error: print timeout after 360s with the turn still in progress"},
		{Status: "DEADLINE_EXCEEDED"},
	}
	for _, result := range cases {
		if !providerTurnDeadlineExceeded(result, "") {
			t.Fatalf("provider deadline was not recognised: %+v", result)
		}
	}
}

// A genuine application error must stay a provider failure, not be retried as a
// capacity wait, so a broken reply does not loop through every account.
func TestProviderTurnDeadlineIgnoresUnrelatedFailures(t *testing.T) {
	for _, result := range []inboxAgyResult{
		{Status: "ERROR", ShortError: "invalid api key"},
		{Status: "ERROR", Reason: "bad_request"},
		{Status: "ERROR", ShortError: "model not found"},
	} {
		if providerTurnDeadlineExceeded(result, "") {
			t.Fatalf("unrelated failure was misread as a provider deadline: %+v", result)
		}
	}
}

// The capacity sentinel is a routing signal. It must never reach the owner as
// the visible failure text.
func TestCapacityWaitSentinelIsNeverUserFacing(t *testing.T) {
	if !strings.HasPrefix(inboxProviderCapacityWait, "\x00") {
		t.Fatal("capacity sentinel must stay distinguishable from real messages")
	}
	if strings.Contains(inboxQuotaNoBackup, inboxProviderCapacityWait) {
		t.Fatal("capacity wait must not leak into the user-facing message")
	}
	if inboxQuotaNoBackup == inboxProviderCapacityWait {
		t.Fatal("capacity sentinel must map to a human message")
	}
}
