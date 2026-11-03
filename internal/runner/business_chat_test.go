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
	if len(nextSteps) != 0 {
		t.Fatalf("expected no unsolicited next steps, got %d: %v", len(nextSteps), nextSteps)
	}
}

func TestFormatBusinessFriendlyMessage_CatalogTaskRecommendations(t *testing.T) {
	title := "Perbarui katalog produk mukena lebaran"
	summary := "Draf foto dan varian warna mukena telah ditambahkan ke katalog"

	msg, nextSteps := FormatBusinessFriendlyMessage(title, summary)

	if len(nextSteps) != 0 || strings.Contains(msg, "Saran Langkah Strategis") || strings.Contains(strings.ToLower(msg), "whatsapp") {
		t.Fatalf("completion must not invent promotional follow-ups: %s", msg)
	}
	if !strings.Contains(msg, summary) || !strings.Contains(msg, "percakapan ini") {
		t.Fatalf("completion must preserve the result and same-chat revision path: %s", msg)
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

func TestFormatBusinessFriendlyReviewMessage_PurgesJargonAndProvidesOptions(t *testing.T) {
	title := "Implementasi Integrasi Gateway Pembayaran"
	summary := "Evaluasi DoD menemukan gap pada penanganan callback webhook"
	gaps := []string{"Targeted tests definition of done belum 100% pass", "Working tree git commit dirty"}

	msg, opts := FormatBusinessFriendlyReviewMessage(title, summary, gaps, 3)

	forbidden := []string{"dod", "definition of done", "git commit", "pull request"}
	msgLower := strings.ToLower(msg)
	for _, term := range forbidden {
		if strings.Contains(msgLower, term) {
			t.Fatalf("found forbidden jargon %q in review message:\n%s", term, msg)
		}
	}

	if !strings.Contains(msg, "Pembaruan penting untuk toko Anda") {
		t.Fatalf("expected friendly intro in review message, got:\n%s", msg)
	}
	if !strings.Contains(msg, "Opsi Tindak Lanjut yang Dapat Anda Pilih") {
		t.Fatalf("expected follow up options section in review message, got:\n%s", msg)
	}
	if len(opts) < 2 {
		t.Fatalf("expected at least 2 strategic follow-up options, got %d", len(opts))
	}
}

func TestDefaultBusinessChatNotifier_SendTaskReviewBusinessMessage(t *testing.T) {
	var sentRoomID string
	var sentContent string

	notifier := NewDefaultBusinessChatNotifier(func(ctx context.Context, roomID, messageContent string) error {
		sentRoomID = roomID
		sentContent = messageContent
		return nil
	})

	err := notifier.SendTaskReviewBusinessMessage(context.Background(), "room_shop_review", "task_xyz", "Rekap Laporan Keuangan", "Laporan siap ditinjau", []string{"Perlu penyesuaian format angka desimal"}, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sentRoomID != "room_shop_review" {
		t.Fatalf("expected roomID room_shop_review, got %s", sentRoomID)
	}
	if !strings.Contains(sentContent, "Tinjau rincian pekerjaan") || !strings.Contains(sentContent, "/tasks/task_xyz") {
		t.Fatalf("expected task link in review content, got:\n%s", sentContent)
	}
}

func TestFormatBusinessFriendlyMessage_DynamicContextNonOlshop(t *testing.T) {
	title := "Integrasi subtitle drama korea"
	summary := "Subtitle episode 1-16 telah disinkronkan dan siap diputar"

	msg, nextSteps := FormatBusinessFriendlyMessage(title, summary, "dracin")

	if strings.Contains(strings.ToLower(msg), "toko anda") {
		t.Fatalf("non-olshop project 'dracin' must not contain 'toko Anda', got:\n%s", msg)
	}
	if !strings.Contains(msg, "Halo! Kabar baik untuk dracin") {
		t.Fatalf("expected project name greeting for dracin, got:\n%s", msg)
	}
	for _, step := range nextSteps {
		if strings.Contains(strings.ToLower(step), "toko anda") {
			t.Fatalf("next step should not contain 'toko Anda' for dracin, got: %s", step)
		}
	}
}

func TestFormatBusinessFriendlyReviewMessage_DynamicContextNonOlshop(t *testing.T) {
	title := "Konfigurasi CDN Video Streaming"
	summary := "Evaluasi menemukan performa buffering masih tinggi"
	gaps := []string{"Perlu optimasi cache rule di CDN"}

	msg, _ := FormatBusinessFriendlyReviewMessage(title, summary, gaps, 2, "dracin")

	if strings.Contains(strings.ToLower(msg), "toko anda") {
		t.Fatalf("non-olshop project 'dracin' must not contain 'toko Anda', got:\n%s", msg)
	}
	if !strings.Contains(msg, "Halo! Pembaruan penting untuk dracin") {
		t.Fatalf("expected project name review intro for dracin, got:\n%s", msg)
	}
}

func TestBuildTaskReviewEnvelope_DualChannelAndJargonPurge(t *testing.T) {
	title := "Investigasi runner heartbeat lost dan lease expired pada attempt #2"
	summary := "Evaluasi definition of done dan dod menemukan kegagalan koneksi pihak ketiga"
	gaps := []string{"Runner heartbeat lost and lease expired; marked interrupted and recoverable without auto-rerun."}

	env := BuildTaskReviewEnvelope(title, summary, gaps, 2, false, "RCA log investigasi menunjukkan timeout", "dracin")

	if env.RequiresDeploy {
		t.Fatal("expected requires_deploy to be false for investigation")
	}

	// Verify jargon purge
	forbidden := []string{"definition of done", "dod", "heartbeat lost", "lease expired", "attempt #"}
	fullText := strings.ToLower(env.Summary + " " + env.Draft + " " + env.Evidence)
	for _, f := range forbidden {
		if strings.Contains(fullText, f) {
			t.Fatalf("found forbidden technical jargon %q in envelope:\n%+v", f, env)
		}
	}

	if strings.Contains(strings.ToLower(env.Draft), "toko anda") {
		t.Fatalf("envelope draft for non-olshop 'dracin' must not contain 'toko Anda', got: %s", env.Draft)
	}

	jsonStr, err := env.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize TaskReviewEnvelope to JSON: %v", err)
	}

	if !strings.Contains(jsonStr, `"requires_deploy":false`) {
		t.Fatalf("expected 'requires_deploy:false' in JSON, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"summary"`) || !strings.Contains(jsonStr, `"draft"`) || !strings.Contains(jsonStr, `"evidence"`) {
		t.Fatalf("missing required envelope fields in JSON: %s", jsonStr)
	}
}
