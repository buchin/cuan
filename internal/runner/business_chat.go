package runner

import (
	"context"
	"fmt"
	"strings"
)

// BusinessChatNotifier defines the contract for sending proactive zero-jargon business messages.
type BusinessChatNotifier interface {
	SendTaskCompletedBusinessMessage(ctx context.Context, roomID, taskID, title, summary string) error
}

// Technical and Scrum jargon terms that must be purged from user-facing shop messages.
var scrumJargonBlacklist = []string{
	"definition of done", "dod", "sprint review", "sprint planning",
	"scrum master", "scrum", "git commit", "pull request", "reconciler",
	"fast-forward", "worker daemon", "pty terminal", "bwrap", "cgroup",
	"attempt", "backlog", "in-flight", "sigterm", "sigkill", "lease expiration",
}

// FormatBusinessFriendlyMessage transforms technical task completion into warm, zero-jargon business communication.
func FormatBusinessFriendlyMessage(title, summary string) (string, []string) {
	cleanTitle := sanitizeJargon(title)
	cleanSummary := sanitizeJargon(summary)

	if cleanTitle == "" {
		cleanTitle = "Pembaruan Toko Online"
	}
	if cleanSummary == "" {
		cleanSummary = "Pekerjaan sistem telah kami selesaikan dan diverifikasi dengan baik agar toko Anda dapat beroperasi lebih lancar."
	}

	// Determine 1-2 strategic next steps based on task context
	nextSteps := determineStrategicNextSteps(title, summary)

	var sb strings.Builder
	sb.WriteString("Halo! Kabar baik untuk toko Anda, pekerjaan berikut telah berhasil diselesaikan dengan rapi:\n\n")
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

func sanitizeJargon(text string) string {
	cleaned := text
	for _, jargon := range scrumJargonBlacklist {
		// Case-insensitive replacement
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

func determineStrategicNextSteps(title, summary string) []string {
	combined := strings.ToLower(title + " " + summary)

	switch {
	case strings.Contains(combined, "katalog") || strings.Contains(combined, "produk") || strings.Contains(combined, "catalog"):
		return []string{
			"Periksa tampilan foto dan rincian harga produk di etalase toko Anda agar siap dipromosikan.",
			"Siapkan pesan siaran promosi singkat untuk menyapa pelanggan setia Anda melalui WhatsApp.",
		}
	case strings.Contains(combined, "promo") || strings.Contains(combined, "diskon") || strings.Contains(combined, "voucher"):
		return []string{
			"Aktifkan kupon atau banner diskon di halaman depan toko agar langsung terlihat pengunjung.",
			"Buat postingan pengumuman promo di media sosial (Instagram/TikTok/Facebook) untuk mengundang pembeli baru.",
		}
	case strings.Contains(combined, "pembayaran") || strings.Contains(combined, "payment") || strings.Contains(combined, "checkout"):
		return []string{
			"Lakukan uji coba transaksi pembelian kecil untuk memastikan alur checkout pelanggan berjalan mulus.",
			"Pastikan nomor rekening atau instruksi QRIS toko sudah tertera dengan jelas di faktur pesanan.",
		}
	case strings.Contains(combined, "laporan") || strings.Contains(combined, "analisis") || strings.Contains(combined, "rekap"):
		return []string{
			"Tinjau tren produk paling diminati minggu ini untuk mempersiapkan stok barang lebih awal.",
			"Catat pelanggan paling loyal untuk diberikan apresiasi atau penawaran khusus.",
		}
	default:
		return []string{
			"Tinjau tampilan dan fungsionalitas terbaru toko Anda secara langsung melalui browser atau smartphone.",
			"Bagikan kabar pembaruan layanan ini kepada pelanggan setia toko Anda untuk meningkatkan kepuasan belanja.",
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
