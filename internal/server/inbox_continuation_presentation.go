package server

import "strings"

func continuationPresentation(remaining string) (title, description string) {
	switch remaining {
	case "verification":
		return "Lanjutkan pemeriksaan", "Lengkapi pemeriksaan agar hasil pekerjaan bisa dipastikan."
	case "implementation":
		return "Lanjutkan perbaikan", "Perbaiki bagian yang masih bermasalah, lalu periksa hasilnya."
	default:
		return "Lanjutkan pekerjaan", "Lanjutkan pekerjaan ini dari hasil terakhir."
	}
}

func continuationDescriptionMatches(plan *inboxApprovalPlan, assessment inboxReviewEvaluation) bool {
	c := plan.Continuation
	if c == nil {
		return false
	}
	// Exact old proposals stay approvable. Rendering them never changes the
	// persisted scope selected by the approval endpoint.
	if c.RemainingWork == "" && plan.Description == c.Revision {
		return true
	}
	title, description := continuationPresentation(assessment.RemainingWork)
	return c.RemainingWork == assessment.RemainingWork && plan.Title == title && plan.Description == description
}

func presentContinuationPlan(plan *inboxApprovalPlan) *inboxApprovalPlan {
	if plan == nil || plan.Continuation == nil {
		return plan
	}
	visible := *plan
	visible.Title, visible.Description = continuationPresentation(plan.Continuation.RemainingWork)
	return &visible
}

func presentContinuationText(content string, plan *inboxApprovalPlan) string {
	if plan == nil || plan.Continuation == nil {
		return content
	}
	// Presentation-only compatibility for the old system-generated review footer.
	// Stored chat history and its immutable approval scope are never rewritten.
	content = strings.ReplaceAll(content, "Hasil kerja belum terverifikasi, termasuk status rilisnya.", "Hasil perubahan belum bisa dipastikan sudah diterapkan.")
	lines := strings.Split(content, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if strings.TrimSpace(line) == "Batas perbaikan otomatis sudah tercapai; hasil sementara dan bukti tetap tersimpan." {
			continue
		}
		kept = append(kept, line)
	}
	result := strings.Join(kept, "\n")
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(result)
}
