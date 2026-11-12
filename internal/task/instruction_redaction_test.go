package task

import (
	"context"
	"strings"
	"testing"
)

// Task instructions are persisted, replayed to a provider and shown in the UI,
// so credential material must be redacted at the creation boundary regardless
// of which caller produced the text.
func TestCreateTaskRedactsCredentialsInInstructions(t *testing.T) {
	svc := NewService(NewMemoryStore(), nil)
	instr := "Deploy the app with these settings:\n" +
		"EXAMPLE_SERVICE_PASSWORD=fixture-only-secret\n" +
		"CUAN_PUBLICATION_SSH_HOST_KEY=ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFixtureKeyMaterial root@fixture\n" +
		"EXAMPLE_SERVICE_PUBLIC_IP=203.0.113.10\n"

	item, err := svc.CreateTask(context.Background(), "owner", CreateTaskRequest{Title: "Deploy", Instructions: instr})
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"fixture-only-secret", "AAAAC3NzaC1lZDI1NTE5AAAAIFixtureKeyMaterial"} {
		if strings.Contains(item.Instructions, secret) {
			t.Fatalf("credential survived task creation: %s", item.Instructions)
		}
	}
	if !strings.Contains(item.Instructions, "[REDACTED]") {
		t.Fatal("no redaction marker recorded")
	}
	// Operational context stays readable so the saved task is still diagnosable.
	if !strings.Contains(item.Instructions, "203.0.113.10") {
		t.Fatalf("non-secret setting was removed: %s", item.Instructions)
	}
}
