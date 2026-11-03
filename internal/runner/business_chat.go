package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// BusinessChatNotifier defines the contract for sending proactive zero-jargon business messages.
type BusinessChatNotifier interface {
	SendTaskCompletedBusinessMessage(ctx context.Context, roomID, taskID, title, summary string) error
	SendTaskReviewBusinessMessage(ctx context.Context, roomID, taskID, title, summary string, gaps []string, attemptNumber int) error
}

// ProjectAwareNotifier extends BusinessChatNotifier with dynamic project domain awareness.
type ProjectAwareNotifier interface {
	BusinessChatNotifier
	SendTaskCompletedBusinessMessageWithContext(ctx context.Context, roomID, taskID, title, summary, projectName string) error
	SendTaskReviewBusinessMessageWithContext(ctx context.Context, roomID, taskID, title, summary, projectName string, gaps []string, attemptNumber int) error
}

// TaskReviewEnvelope represents a structured dual-channel output envelope for task reviews.
type TaskReviewEnvelope struct {
	Summary        string `json:"summary"`
	Draft          string `json:"draft"`
	Evidence       string `json:"evidence"`
	RequiresDeploy bool   `json:"requires_deploy"`
}

// ToJSON serializes the TaskReviewEnvelope to JSON format.
func (e TaskReviewEnvelope) ToJSON() (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Technical and Scrum jargon terms that must be purged from user-facing shop messages.
var scrumJargonBlacklist = []string{
	"definition of done", "definition-of-done", "dod", "sprint review", "sprint planning",
	"scrum master", "scrum", "git commit", "pull request", "reconciler",
	"fast-forward", "worker daemon", "pty terminal", "bwrap", "cgroup",
	"attempt #", "attempt", "backlog", "in-flight", "sigterm", "sigkill",
	"heartbeat lost", "lease expired", "lease expiration",
}

var (
	attemptRegex   = regexp.MustCompile(`(?i)\battempt\s*(?:#?\s*\d+)?\b`)
	heartbeatRegex = regexp.MustCompile(`(?i)\b(?:runner\s+)?heartbeat\s+lost\b`)
	leaseRegex     = regexp.MustCompile(`(?i)\b(?:and\s+)?lease\s+expired\b`)
)

func resolveContextName(projectName ...string) (string, bool) {
	if len(projectName) > 0 {
		name := strings.TrimSpace(projectName[0])
		if name != "" && !strings.EqualFold(name, "default") {
			lower := strings.ToLower(name)
			if strings.Contains(lower, "toko") || strings.Contains(lower, "shop") || strings.Contains(lower, "store") {
				return "toko Anda", true
			}
			return name, false
		}
	}
	return "toko Anda", true
}

// FormatBusinessFriendlyMessage transforms technical task completion into warm, zero-jargon business communication.
func FormatBusinessFriendlyMessage(title, summary string, projectName ...string) (string, []string) {
	cleanTitle := sanitizeJargon(title)
	cleanSummary := sanitizeJargon(summary)
	contextName, isShop := resolveContextName(projectName...)

	if cleanTitle == "" {
		if isShop {
			cleanTitle = "Pembaruan Toko Online"
		} else {
			cleanTitle = fmt.Sprintf("Pembaruan %s", contextName)
		}
	}
	if cleanSummary == "" {
		if isShop {
			cleanSummary = "Pekerjaan sistem telah kami selesaikan dan diverifikasi dengan baik agar toko Anda dapat beroperasi lebih lancar."
		} else {
			cleanSummary = fmt.Sprintf("Pekerjaan sistem telah kami selesaikan dan diverifikasi dengan baik agar %s dapat beroperasi lebih lancar.", contextName)
		}
	}

	// Determine 1-2 strategic next steps based on task context and project
	nextSteps := determineStrategicNextSteps(title, summary, projectName...)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Halo! Kabar baik untuk %s, pekerjaan berikut telah berhasil diselesaikan dengan rapi:\n\n", contextName))
	sb.WriteString(fmt.Sprintf("**%s**\n", cleanTitle))
	sb.WriteString(fmt.Sprintf("%s\n\n", cleanSummary))

	if len(nextSteps) > 0 {
		sb.WriteString("**Saran Langkah Strategis Berikutnya:**\n")
		for i, step := range nextSteps {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Silakan tinjau hasilnya di aplikasi Cuan kapan saja. Jika ada hal lain yang ingin dikerjakan, cukup beri tahu kami di sini!")

	return sb.String(), nextSteps
}

// FormatBusinessFriendlyReviewMessage formats a warm, transparent, zero-jargon business message
// when a task reaches the retry limit or needs human review/direction.
func FormatBusinessFriendlyReviewMessage(title, summary string, gaps []string, attemptNumber int, projectName ...string) (string, []string) {
	cleanTitle := sanitizeJargon(title)
	cleanSummary := sanitizeJargon(summary)
	contextName, isShop := resolveContextName(projectName...)

	if cleanTitle == "" {
		if isShop {
			cleanTitle = "Pekerjaan Toko Online"
		} else {
			cleanTitle = fmt.Sprintf("Pekerjaan %s", contextName)
		}
	}
	if cleanSummary == "" {
		if isShop {
			cleanSummary = "Pekerjaan sistem telah kami proses dan memerlukan peninjauan lebih lanjut agar hasil akhirnya optimal untuk toko Anda."
		} else {
			cleanSummary = fmt.Sprintf("Pekerjaan sistem telah kami proses dan memerlukan peninjauan lebih lanjut agar hasil akhirnya optimal untuk %s.", contextName)
		}
	}

	followUpOptions := []string{
		"Beri arahan tambahan atau sesuaikan instruksi pekerjaan di obrolan ini agar kami dapat melanjutkannya.",
		"Tinjau perkembangan yang tersimpan di menu Tugas aplikasi Cuan dan putuskan langkah prioritas berikutnya.",
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Halo! Pembaruan penting untuk %s terkait pekerjaan:\n\n", contextName))
	sb.WriteString(fmt.Sprintf("**%s**\n", cleanTitle))
	sb.WriteString(fmt.Sprintf("%s\n\n", cleanSummary))

	if len(gaps) > 0 {
		sb.WriteString("**Catatan yang Memerlukan Perhatian:**\n")
		for i, gap := range gaps {
			cleanGap := sanitizeJargon(gap)
			if cleanGap != "" {
				sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, cleanGap))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("**Opsi Tindak Lanjut yang Dapat Anda Pilih:**\n")
	for i, opt := range followUpOptions {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, opt))
	}
	sb.WriteString("\nKami siap membantu melanjutkan kapan saja sesuai keputusan Anda!")

	return sb.String(), followUpOptions
}

// BuildTaskReviewEnvelope creates a structured dual-channel output envelope for task review.
func BuildTaskReviewEnvelope(title, summary string, gaps []string, attemptNumber int, requiresDeploy bool, evidence string, projectName ...string) TaskReviewEnvelope {
	cleanTitle := sanitizeJargon(title)
	cleanSummary := sanitizeJargon(summary)
	contextName, isShop := resolveContextName(projectName...)

	operatorSummary := cleanSummary
	if operatorSummary == "" {
		if cleanTitle != "" {
			operatorSummary = fmt.Sprintf("Pekerjaan '%s' telah diproses dan memerlukan peninjauan operator.", cleanTitle)
		} else if isShop {
			operatorSummary = "Pekerjaan sistem telah diproses dan memerlukan peninjauan operator."
		} else {
			operatorSummary = fmt.Sprintf("Pekerjaan %s telah diproses dan memerlukan peninjauan operator.", contextName)
		}
	}

	var draftVisitorReply string
	if isShop {
		draftVisitorReply = "Halo! Terkait kebutuhan toko Anda, kami telah memeriksa perkembangannya dan menyiapkan langkah terbaik untuk Anda. Silakan beri tahu kami jika ada detail yang ingin disesuaikan."
	} else {
		draftVisitorReply = fmt.Sprintf("Halo! Terkait kebutuhan %s, kami telah memeriksa perkembangannya dan menyiapkan langkah terbaik untuk Anda. Silakan beri tahu kami jika ada detail yang ingin disesuaikan.", contextName)
	}

	if len(gaps) > 0 {
		cleanGap := sanitizeJargon(gaps[0])
		if cleanGap != "" {
			if isShop {
				draftVisitorReply = fmt.Sprintf("Halo! Terkait layanan toko Anda, kami sedang memvalidasi bagian: %s. Tim kami siap membantu memastikan semuanya berjalan optimal.", cleanGap)
			} else {
				draftVisitorReply = fmt.Sprintf("Halo! Terkait layanan %s, kami sedang memvalidasi bagian: %s. Tim kami siap membantu memastikan semuanya berjalan optimal.", contextName, cleanGap)
			}
		}
	}

	cleanEvidence := sanitizeJargon(evidence)
	if cleanEvidence == "" {
		if len(gaps) > 0 {
			var cleanedGaps []string
			for _, g := range gaps {
				if cg := sanitizeJargon(g); cg != "" {
					cleanedGaps = append(cleanedGaps, cg)
				}
			}
			cleanEvidence = strings.Join(cleanedGaps, "; ")
		} else if !requiresDeploy {
			cleanEvidence = "Bukti verifikasi belum disertakan."
		} else {
			cleanEvidence = "Bukti verifikasi belum disertakan."
		}
	}

	return TaskReviewEnvelope{
		Summary:        operatorSummary,
		Draft:          draftVisitorReply,
		Evidence:       cleanEvidence,
		RequiresDeploy: requiresDeploy,
	}
}

// FormatTaskReviewEnvelopeJSON produces a formatted JSON string for the dual-channel envelope.
func FormatTaskReviewEnvelopeJSON(title, summary string, gaps []string, attemptNumber int, requiresDeploy bool, evidence string, projectName ...string) (string, error) {
	env := BuildTaskReviewEnvelope(title, summary, gaps, attemptNumber, requiresDeploy, evidence, projectName...)
	return env.ToJSON()
}

func sanitizeJargon(text string) string {
	cleaned := text
	cleaned = heartbeatRegex.ReplaceAllString(cleaned, "koneksi terputus")
	cleaned = leaseRegex.ReplaceAllString(cleaned, "waktu sesi habis")
	cleaned = attemptRegex.ReplaceAllString(cleaned, "")

	for _, jargon := range scrumJargonBlacklist {
		lower := strings.ToLower(cleaned)
		for {
			idx := strings.Index(lower, jargon)
			if idx == -1 {
				break
			}
			cleaned = cleaned[:idx] + cleaned[idx+len(jargon):]
			lower = strings.ToLower(cleaned)
		}
	}
	// Clean up double spaces or residual symbols
	cleaned = strings.ReplaceAll(cleaned, "  ", " ")
	cleaned = strings.TrimSpace(cleaned)
	return cleaned
}

func determineStrategicNextSteps(title, summary string, projectName ...string) []string {
	combined := strings.ToLower(title + " " + summary)
	contextName, isShop := resolveContextName(projectName...)

	switch {
	case strings.Contains(combined, "katalog") || strings.Contains(combined, "produk") || strings.Contains(combined, "catalog"):
		if isShop {
			return []string{
				"Periksa tampilan foto dan rincian harga produk di etalase toko Anda agar siap dipromosikan.",
				"Siapkan pesan siaran promosi singkat untuk menyapa pelanggan setia Anda melalui WhatsApp.",
			}
		}
		return []string{
			fmt.Sprintf("Periksa rincian katalog pada %s agar siap dipublikasikan.", contextName),
			fmt.Sprintf("Siapkan pesan pemberitahuan singkat untuk menyapa pengguna setia %s.", contextName),
		}
	case strings.Contains(combined, "promo") || strings.Contains(combined, "diskon") || strings.Contains(combined, "voucher"):
		if isShop {
			return []string{
				"Aktifkan kupon atau banner diskon di halaman depan toko agar langsung terlihat pengunjung.",
				"Buat postingan pengumuman promo di media sosial (Instagram/TikTok/Facebook) untuk mengundang pembeli baru.",
			}
		}
		return []string{
			fmt.Sprintf("Aktifkan fitur diskon atau banner promo di halaman depan %s.", contextName),
			fmt.Sprintf("Buat postingan pengumuman di media sosial untuk mengundang pengguna baru %s.", contextName),
		}
	case strings.Contains(combined, "pembayaran") || strings.Contains(combined, "payment") || strings.Contains(combined, "checkout"):
		if isShop {
			return []string{
				"Lakukan uji coba transaksi pembelian kecil untuk memastikan alur checkout pelanggan berjalan mulus.",
				"Pastikan nomor rekening atau instruksi QRIS toko sudah tertera dengan jelas di faktur pesanan.",
			}
		}
		return []string{
			fmt.Sprintf("Lakukan uji coba alur pembayaran pada %s untuk memastikan transaksi berjalan mulus.", contextName),
			fmt.Sprintf("Pastikan instruksi pembayaran %s sudah tertera dengan jelas.", contextName),
		}
	case strings.Contains(combined, "laporan") || strings.Contains(combined, "analisis") || strings.Contains(combined, "rekap"):
		if isShop {
			return []string{
				"Tinjau tren produk paling diminati minggu ini untuk mempersiapkan stok barang lebih awal.",
				"Catat pelanggan paling loyal untuk diberikan apresiasi atau penawaran khusus.",
			}
		}
		return []string{
			fmt.Sprintf("Tinjau tren penggunaan dan performa paling aktif minggu ini pada %s.", contextName),
			fmt.Sprintf("Catat pengguna paling aktif pada %s untuk diberikan apresiasi atau peningkatan layanan.", contextName),
		}
	default:
		if isShop {
			return []string{
				"Tinjau tampilan dan fungsionalitas terbaru toko Anda secara langsung melalui browser atau smartphone.",
				"Bagikan kabar pembaruan layanan ini kepada pelanggan setia toko Anda untuk meningkatkan kepuasan belanja.",
			}
		}
		return []string{
			fmt.Sprintf("Tinjau tampilan dan fungsionalitas terbaru %s secara langsung melalui browser atau smartphone.", contextName),
			fmt.Sprintf("Bagikan kabar pembaruan layanan ini kepada pengguna setia %s untuk meningkatkan kepuasan layanan.", contextName),
		}
	}
}

// DefaultBusinessChatNotifier sends business-friendly messages via a supplied sender callback.
type DefaultBusinessChatNotifier struct {
	Sender func(ctx context.Context, roomID, messageContent string) error
}

func NewDefaultBusinessChatNotifier(sender func(ctx context.Context, roomID, messageContent string) error) *DefaultBusinessChatNotifier {
	return &DefaultBusinessChatNotifier{
		Sender: sender,
	}
}

func (n *DefaultBusinessChatNotifier) SendTaskCompletedBusinessMessage(ctx context.Context, roomID, taskID, title, summary string) error {
	if n.Sender == nil || roomID == "" {
		return nil
	}
	content, _ := FormatBusinessFriendlyMessage(title, summary)
	taskLink := fmt.Sprintf("\n\n[Lihat rincian pekerjaan](/tasks/%s)", taskID)
	return n.Sender(ctx, roomID, content+taskLink)
}

func (n *DefaultBusinessChatNotifier) SendTaskCompletedBusinessMessageWithContext(ctx context.Context, roomID, taskID, title, summary, projectName string) error {
	if n.Sender == nil || roomID == "" {
		return nil
	}
	content, _ := FormatBusinessFriendlyMessage(title, summary, projectName)
	taskLink := fmt.Sprintf("\n\n[Lihat rincian pekerjaan](/tasks/%s)", taskID)
	return n.Sender(ctx, roomID, content+taskLink)
}

func (n *DefaultBusinessChatNotifier) SendTaskReviewBusinessMessage(ctx context.Context, roomID, taskID, title, summary string, gaps []string, attemptNumber int) error {
	if n.Sender == nil || roomID == "" {
		return nil
	}
	content, _ := FormatBusinessFriendlyReviewMessage(title, summary, gaps, attemptNumber)
	taskLink := fmt.Sprintf("\n\n[Tinjau rincian pekerjaan](/tasks/%s)", taskID)
	return n.Sender(ctx, roomID, content+taskLink)
}

func (n *DefaultBusinessChatNotifier) SendTaskReviewBusinessMessageWithContext(ctx context.Context, roomID, taskID, title, summary, projectName string, gaps []string, attemptNumber int) error {
	if n.Sender == nil || roomID == "" {
		return nil
	}
	content, _ := FormatBusinessFriendlyReviewMessage(title, summary, gaps, attemptNumber, projectName)
	taskLink := fmt.Sprintf("\n\n[Tinjau rincian pekerjaan](/tasks/%s)", taskID)
	return n.Sender(ctx, roomID, content+taskLink)
}
