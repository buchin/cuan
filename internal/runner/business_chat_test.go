package runner

import (
	"context"
	"strings"
	"testing"
)

func TestFormatBusinessFriendlyMessage_PurgesJargon(t *testing.T) {
	title := "S55-T3: Autonomous Worker Daemon & DoD Passed with Git Commit"
	summary := "Sprint review: Completed definition of done checklist items and fast-forward pull request"

	msg, nextSteps := FormatBusinessFriendlyMessage(title, summary)

	// Ensure no forbidden technical/Scrum terms leak into message
	forbidden := []string{"dod", "definition of done", "sprint review", "pull request", "git commit", "worker daemon"}
	msgLower := strings.ToLower(msg)
	for _, term := range forbidden {
		if strings.Contains(msgLower, term) {
			t.Fatalf("found forbidden technical jargon %q in business chat message:\n%s", term, msg)
		}
	}

	// Ensure friendly greeting and structure
	if !strings.Contains(msg, "Halo! Kabar baik untuk toko Anda") {
		t.Fatalf("expected warm greeting in message, got:\n%s", msg)
	}
	if len(nextSteps) < 1 || len(nextSteps) > 2 {
		t.Fatalf("expected 1-2 strategic next steps, got %d: %v", len(nextSteps), nextSteps)
	}
}

func TestFormatBusinessFriendlyMessage_CatalogTaskRecommendations(t *testing.T) {
	title := "Perbarui katalog produk mukena lebaran"
	summary := "Draf foto dan varian warna mukena telah ditambahkan ke katalog"

	msg, nextSteps := FormatBusinessFriendlyMessage(title, summary)

	if len(nextSteps) != 2 {
		t.Fatalf("expected 2 recommendations, got %d", len(nextSteps))
	}
	combined := strings.ToLower(nextSteps[0] + " " + nextSteps[1])
	if !strings.Contains(combined, "etalase") && !strings.Contains(combined, "whatsapp") {
		t.Fatalf("expected catalog-related strategic next steps, got: %v", nextSteps)
	}
	if !strings.Contains(msg, "Saran Langkah Strategis Berikutnya") {
		t.Fatalf("expected next steps header in message, got:\n%s", msg)
	}
}

func TestDefaultBusinessChatNotifier_SendTaskCompletedBusinessMessage(t *testing.T) {
	var sentRoomID string
	var sentContent string

	notifier := NewDefaultBusinessChatNotifier(func(ctx context.Context, roomID, messageContent string) error {
		sentRoomID = roomID
		sentContent = messageContent
		return nil
	})

	err := notifier.SendTaskCompletedBusinessMessage(context.Background(), "room_shop_123", "task_abc", "Atur Diskon Akhir Bulan", "Voucher diskon 20% siap digunakan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sentRoomID != "room_shop_123" {
		t.Fatalf("expected roomID room_shop_123, got %s", sentRoomID)
	}
	if !strings.Contains(sentContent, "Lihat rincian pekerjaan") || !strings.Contains(sentContent, "/tasks/task_abc") {
		t.Fatalf("expected task link in sent content, got:\n%s", sentContent)
	}
}
