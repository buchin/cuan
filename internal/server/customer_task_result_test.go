package server

import (
	"strings"
	"testing"

	"github.com/buchin/cuan/internal/task"
)

func TestAcceptedCustomerAuditDoesNotInventRepairOrReplyDraft(t *testing.T) {
	d := &task.TaskDetailResponse{Task: task.Task{ID: "audit", Title: "Audit provider outage", Summary: "Provider outage confirmed; no service changes were made."}}
	result := acceptedCustomerTaskResult(d, "Audit selesai. Gangguan berasal dari provider dan belum pulih.")
	for _, invented := range []string{"berhasil kami perbaiki", "bisa diakses kembali", "video drama", "Customer reply draft", "Halo!"} {
		if strings.Contains(result, invented) {
			t.Fatal("invented customer claim", invented)
		}
	}
	if !strings.Contains(result, "belum pulih") || !strings.Contains(result, "/tasks/audit") {
		t.Fatal(result)
	}
}

func TestAcceptedCustomerResultUsesSavedDraftOnce(t *testing.T) {
	d := &task.TaskDetailResponse{Task: task.Task{ID: "audit", Summary: "Audit complete", CompleteNote: "Verified provider response.\n\n**Draf Balasan:**\n> Gangguan provider masih berlangsung.\n> Kami akan menyampaikan pembaruannya.\n\n[View work](/tasks/private)"}}
	result := acceptedCustomerTaskResult(d, "Audit selesai.")
	report, draft := splitCustomerReplyDraft(result)
	if strings.Count(result, "Gangguan provider masih berlangsung.") != 1 || draft != "Gangguan provider masih berlangsung.\nKami akan menyampaikan pembaruannya." {
		t.Fatal(result)
	}
	if strings.Contains(draft, "/tasks/") || !strings.Contains(report, "/tasks/audit") {
		t.Fatal("internal link mixed into visitor draft")
	}
}
