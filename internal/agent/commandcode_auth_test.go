package agent

import "testing"

func TestCommandCodeAuthLifecycleIsIdempotent(t *testing.T) {
	owner := "owner-lifecycle"
	first := StartCommandCodeAuth(owner)
	second := StartCommandCodeAuth(owner)
	if first.ID != second.ID || first.Status != "pending" {
		t.Fatalf("expected one resumable pending session: %#v %#v", first, second)
	}
	if _, err := CompleteCommandCodeAuth(first.ID, owner, true, true, "connected"); err != nil {
		t.Fatal(err)
	}
	third := StartCommandCodeAuth(owner)
	if third.ID == first.ID {
		t.Fatal("authenticated session must not be reused as pending")
	}
	if err := CancelCommandCodeAuth(third.ID, owner); err != nil {
		t.Fatal(err)
	}
	if err := CancelCommandCodeAuth(third.ID, owner); err != nil {
		t.Fatalf("cancel must be idempotent: %v", err)
	}
	got, err := GetCommandCodeAuth(third.ID, owner)
	if err != nil || got.Status != "cancelled" {
		t.Fatalf("expected cancelled session, got %#v %v", got, err)
	}
}
