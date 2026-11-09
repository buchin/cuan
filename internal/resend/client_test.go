package resend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_ValidateAPIKey(t *testing.T) {
	t.Run("valid api key", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api-keys" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer re_valid_12345" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":"key_1","name":"Production"}]}`))
		}))
		defer ts.Close()

		c := NewClient("re_valid_12345", WithBaseURL(ts.URL))
		err := c.ValidateAPIKey(context.Background())
		if err != nil {
			t.Fatalf("expected valid key, got error: %v", err)
		}
	})

	t.Run("invalid api key returns ErrInvalidAPIKey", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"statusCode":401,"message":"API key is invalid","name":"validation_error"}`))
		}))
		defer ts.Close()

		c := NewClient("re_invalid_999", WithBaseURL(ts.URL))
		err := c.ValidateAPIKey(context.Background())
		if err == nil || err != ErrInvalidAPIKey {
			t.Fatalf("expected ErrInvalidAPIKey, got: %v", err)
		}
	})

	t.Run("empty api key returns ErrEmptyAPIKey", func(t *testing.T) {
		c := NewClient("")
		err := c.ValidateAPIKey(context.Background())
		if err != ErrEmptyAPIKey {
			t.Fatalf("expected ErrEmptyAPIKey, got: %v", err)
		}
	})
}

func TestClient_SendEmail(t *testing.T) {
	t.Run("successful email send", func(t *testing.T) {
		var receivedReq EmailRequest
		var key string
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/emails" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer re_test_key" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			key = r.Header.Get("Idempotency-Key")
			_ = json.NewDecoder(r.Body).Decode(&receivedReq)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"email_msg_123456789"}`))
		}))
		defer ts.Close()

		c := NewClient("re_test_key", WithBaseURL(ts.URL))
		resp, err := c.SendEmail(context.Background(), EmailRequest{
			IdempotencyKey: "cuan-idle/room/message",
			From:           "Cuan Support <support@cuan.id>",
			To:             []string{"owner@tokocuan.com"},
			Subject:        "Notifikasi Live Chat",
			HTML:           "<p>Halo ada pesan baru</p>",
			Text:           "Halo ada pesan baru",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if key != "cuan-idle/room/message" {
			t.Fatalf("missing idempotency key: %q", key)
		}
		if resp.ID != "email_msg_123456789" {
			t.Fatalf("expected id email_msg_123456789, got %s", resp.ID)
		}
		if receivedReq.From != "Cuan Support <support@cuan.id>" {
			t.Errorf("expected sender %s, got %s", "Cuan Support <support@cuan.id>", receivedReq.From)
		}
	})

	t.Run("validation errors for send payload", func(t *testing.T) {
		c := NewClient("re_test_key")

		// Empty sender
		_, err := c.SendEmail(context.Background(), EmailRequest{
			To:      []string{"test@cuan.id"},
			Subject: "Sub",
			Text:    "Txt",
		})
		if err != ErrEmptySender {
			t.Errorf("expected ErrEmptySender, got %v", err)
		}

		// Empty recipient
		_, err = c.SendEmail(context.Background(), EmailRequest{
			From:    "from@cuan.id",
			Subject: "Sub",
			Text:    "Txt",
		})
		if err != ErrEmptyRecipients {
			t.Errorf("expected ErrEmptyRecipients, got %v", err)
		}

		// Empty subject
		_, err = c.SendEmail(context.Background(), EmailRequest{
			From: "from@cuan.id",
			To:   []string{"to@cuan.id"},
			Text: "Txt",
		})
		if err != ErrEmptySubject {
			t.Errorf("expected ErrEmptySubject, got %v", err)
		}

		// Empty content
		_, err = c.SendEmail(context.Background(), EmailRequest{
			From:    "from@cuan.id",
			To:      []string{"to@cuan.id"},
			Subject: "Sub",
		})
		if err != ErrEmptyContent {
			t.Errorf("expected ErrEmptyContent, got %v", err)
		}
	})
}

func TestBuildChatIdleTranscript(t *testing.T) {
	params := ChatIdleTemplateParams{
		StoreName:       "Sepatu Barokah",
		CustomerName:    "Ahmad Budi",
		CustomerEmail:   "budi@example.com",
		RoomID:          "room-abc-123",
		ConversationURL: "https://cuan.dojo.cc/inbox/room-abc-123",
		IdleMinutes:     7,
		Messages: []ChatMessageItem{
			{
				SenderName: "Ahmad Budi",
				SenderRole: "customer",
				Content:    "Halo, apakah sepatu ukuran 42 warna hitam masih ready stock?",
				SentAt:     time.Date(2026, 10, 31, 14, 30, 0, 0, time.UTC),
			},
			{
				SenderName: "Ahmad Budi",
				SenderRole: "customer",
				Content:    "Bisa kirim sameday hari ini juga kak?",
				SentAt:     time.Date(2026, 10, 31, 14, 31, 0, 0, time.UTC),
			},
		},
	}

	htmlContent := BuildChatIdleTranscriptHTML(params)
	if !strings.Contains(htmlContent, "Sepatu Barokah") {
		t.Errorf("html should contain store name")
	}
	if !strings.Contains(htmlContent, "Ahmad Budi") {
		t.Errorf("html should contain customer name")
	}
	if !strings.Contains(htmlContent, "budi@example.com") {
		t.Errorf("html should contain customer email")
	}
	if !strings.Contains(htmlContent, "sepatu ukuran 42") {
		t.Errorf("html should contain message content")
	}
	if !strings.Contains(htmlContent, "https://cuan.dojo.cc/inbox/room-abc-123") {
		t.Errorf("html should contain conversation link")
	}
	if !strings.Contains(htmlContent, "7 menit") {
		t.Errorf("html should mention idle minutes")
	}

	textContent := BuildChatIdleTranscriptText(params)
	if !strings.Contains(textContent, "Sepatu Barokah") || !strings.Contains(textContent, "Ahmad Budi") {
		t.Errorf("text should contain store and customer name")
	}
	if !strings.Contains(textContent, "sepatu ukuran 42") {
		t.Errorf("text should contain message content")
	}
}
