# Notifications

Open the bell to see approvals, work updates, reply problems and customer messages.
Each row shows the relevant conversation, project or contact, what happened, and
where it opens. Rows are grouped by day; hover over the short time for the exact
date. On a phone, opening the row shows the full conversation or work record.

**Unread** shows unread updates that have not already been handled. **All** keeps
recent history, including **Approved**, **Saved for later**, **Recovering** and
**Recovered** updates. A recovery in progress does not mean the work succeeded.
When work advances, an older request for attention becomes an informational
history item. The feed retains up to100 recent items.

Opening a notification marks it read when the connection allows it. If saving the
read flag fails, the destination still opens; the item remains unread and can be
retried later. Opening a row does not approve a proposal or restart work—those
actions remain in the relevant conversation.

The top-right menu offers **Mark all as read** and **Notification settings**.
Settings controls categories, quiet hours and browser push. Push text on a locked
device is generic; project and conversation context is shown inside Cuan.

## Quick view

On desktop and tablet, select the bell to preview recent notifications without
leaving the current page. On mobile, the bell opens the full notification page
directly; use Back to return to the page you came from.
Use **Mark all as read** directly in its header, or **View all notifications** for
the full page. Escape, the close button or clicking outside dismisses the panel.
On the full page, **Mark all as read** sits on the right of **All / Unread**.
Phones show the ✓✓ icon; desktop also shows its label. It is disabled when no
unread items need attention. If saving read state
fails, retry; opening the related conversation or task still works.

---

## Integrasi Email Resend & Transkrip Chat Idle

Untuk mencegah potensi kehilangan calon pembeli (*leads*) saat staf atau pemilik toko sedang tidak membuka antarmuka web Cuan, sistem dilengkapi dengan integrasi **Resend Email & Idle Chat Detector**:

### 1. Cara Kerja Deteksi Chat Idle
1. **Debounce Timer Otomatis**: Saat pengunjung mengirim pesan di widget Live Chat (`POST /api/chat/messages`), backend secara otomatis menjadwalkan timer deteksi idle untuk ruang percakapan tersebut.
2. **Ambang Batas Waktu**: Pilihan fleksibel mulai dari 1 menit (testing), 5 menit (rekomendasi toko aktif), 10 menit (standar bisnis), hingga 15-30 menit.
3. **Penyaring Cerdas & Pembatalan Otomatis**:
   - Jika operator atau AI membalas pesan pengunjung, timer idle otomatis dibatalkan (`CancelChatIdleTimer`).
   - Jika staf membaca percakapan di Inbox, timer idle disinkronkan dan tidak akan memicu email.
4. **Scheduler Background Cron (`chat_idle_email_digest`)**:
   - Job background berjalan setiap 1 menit untuk memeriksa ruangan yang memiliki pesan belum terbaca melebihi ambang batas waktu.
   - Mengirimkan transkrip percakapan lengkap dalam format email HTML responsif dan teks polos ke email staf/pemilik toko.
   - Disertai tautan langsung (1-klik) menuju percakapan di antarmuka Inbox Cuan.
5. **Perlindungan Anti-Spam / Cooldown**:
   - Menggunakan mekanisme cooldown minimal 5 menit per ruangan agar tidak membanjiri kotak masuk email penerima (*anti-inbox bombing*).

### 2. Konfigurasi di Pengaturan
Buka **Settings > Notifications** (`/settings/notifications`) atau kartu **Resend Email** di **Settings > Integrations**:
- **API Key Resend**: Masukkan API key dari dashboard Resend (diawali dengan `re_`). Kunci disimpan terenkripsi dengan AES-256-GCM pada vault server dan selalu dimasking (`re_...xxxx`) di UI.
- **Validasi Key**: Klik tombol **Validasi Key** untuk memverifikasi keabsahan API key langsung ke endpoint `/api-keys` Resend tanpa menyimpan kunci tidak valid.
- **Nama & Email Pengirim**: Tentukan nama toko dan alamat pengirim (contoh: `notif@toko.com` atau default `onboarding@resend.dev` untuk testing).
- **Email Penerima**: Alamat email staf yang bertugas menerima transkrip chat idle.
- **Uji Coba Pengiriman**: Klik **Kirim Uji Coba** untuk memverifikasi pengiriman email riil via Resend API.

