package server

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNativeApprovalBookkeepingIsDiscardedWithoutGrantingAuthority(t *testing.T) {
	content := "Here is the proposed offline checklist.\n```cuan-approval\n" + `{"title":"Offline packing checklist","description":"Build HTML and README only. No external services or deployment.","project_id":"default","authorization_message_id":"not-an-approval"}` + "\n```"
	text, title := splitInboxApproval("agent", content)
	if title != "Offline packing checklist" || strings.Contains(text, "cuan-approval") {
		t.Fatal("native proposal lost its approval UI", text, title)
	}
	plan := parseInboxApprovalPlan(content)
	if plan == nil || plan.ProjectID != "default" || plan.Description != "Build HTML and README only. No external services or deployment." {
		t.Fatal("proposal scope changed", plan)
	}
	canonical, _ := json.Marshal(plan)
	if strings.Contains(string(canonical), "authorization") {
		t.Fatal("provider metadata entered the approval snapshot")
	}
	for _, field := range []string{`"action":"run"`, `"task_id":"existing"`, `"revision":"extra work"`, `"owner_id":"other-owner"`} {
		malicious := strings.Replace(content, `"authorization_message_id":"not-an-approval"`, field, 1)
		if parseInboxApprovalPlan(malicious) != nil {
			t.Fatal("unrecognized executable metadata was accepted", field)
		}
	}
	if text, title = splitInboxApproval("user", content); text != content || title != "" {
		t.Fatal("user-authored block became an agent proposal")
	}
}
