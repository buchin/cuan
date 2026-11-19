package db

import (
	"testing"
	"time"
)

// The provider stops itself at its print timeout. If the request window is not
// strictly longer, the request deadline always wins the race and the answer is
// destroyed: production evidence showed a stage still reading files at 346.8s
// of a 346.97s budget.
func TestInboxRequestWindowOutlastsProviderPrintTimeout(t *testing.T) {
	const providerPrintTimeout = 900 * time.Second
	if InboxRequestWindow <= providerPrintTimeout {
		t.Fatalf("request window %v must exceed the provider print timeout %v", InboxRequestWindow, providerPrintTimeout)
	}
	if InboxRequestWindow-providerPrintTimeout < 30*time.Second {
		t.Fatalf("request window leaves too little room (%v) for the provider to report", InboxRequestWindow-providerPrintTimeout)
	}
}
