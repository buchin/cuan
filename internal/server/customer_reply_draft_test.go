package server

import (
	"strings"
	"testing"
)

func TestCustomerReplyDraftSeparatesInternalReportAndLinks(t *testing.T) {
	for _, marker := range []string{"**Draf Balasan untuk Pengunjung:**", "**Customer reply draft:**"} {
		raw := "Internal verified report.\n\n" + marker + "\n> \"Terima kasih sudah menunggu.\n> Kami sedang memeriksa kendalanya.\"\n\n[Tinjau rincian pekerjaan](/tasks/private-task)\n\n**Next steps:**\nKeep monitoring."
		report, draft := splitCustomerReplyDraft(raw)
		if draft != "Terima kasih sudah menunggu.\nKami sedang memeriksa kendalanya." {
			t.Fatal(draft)
		}
		if strings.Contains(report, "Terima kasih") || !strings.Contains(report, "/tasks/private-task") || !strings.Contains(report, "Keep monitoring") {
			t.Fatal(report)
		}
	}
	_, draft := splitCustomerReplyDraft("**Draf Balasan:**\n> Balasan. [Task internal](/tasks/private-task)")
	if draft != "Balasan." {
		t.Fatal("internal link leaked", draft)
	}
	raw := "Ordinary customer message without a draft."
	if report, draft := splitCustomerReplyDraft(raw); report != raw || draft != "" {
		t.Fatal(report, draft)
	}
}
