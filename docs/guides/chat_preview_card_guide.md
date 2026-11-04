# Panduan Teknis: Chat Preview Card & Deep Link Ruang Obrolan (/inbox/:id & /chat/:id)

## 1. Ikhtisar Fitur
Fitur **ChatPreviewCard** (komponen `RoomCard` / `ChatPreviewCard`) memberikan kemampuan pratinjau interaktif cerdas untuk tautan percakapan internal di antarmuka web Cuan. Ketika agen AI, sistem, atau pengguna membagikan tautan ruang obrolan internal, tautan tersebut secara otomatis dirender sebagai kartu pratinjau kaya informasi (bukan teks mentah), memungkinkan navigasi cepat tanpa kehilangan konteks.

---

## 2. Arsitektur Komponen & Ruang Lingkup

### A. Parser Tautan & Deteksi Host Internal (`web/src/features/inbox/room-link.ts`)
- **Deteksi Format Tautan**:
  - Format rute relatif: `/inbox/:id` dan `/chat/:id`
  - Format rute query / hash: `/inbox/:id?filter=all#bottom`
  - URL absolut pada domain Cuan (`cuan.dojo.cc`, `*.cuan.id`, `localhost`, `127.0.0.1`, `window.location.origin`)
- **Penyaringan Keamanan URL Eksternal**:
  - URL eksternal (seperti `https://google.com/chat/...` atau domain penyerang) tidak diperlakukan sebagai tautan internal dan tidak memicu pembuatan preview card.

### B. Integrasi Chat Markdown & Navigation (`ChatMarkdown.tsx` & `MessageContent.tsx`)
- **Autolinking Cerdas (`autolinkConversationPaths`)**:
  - Mengonversi path `/inbox/:id` dan `/chat/:id` di dalam pesan teks mentah menjadi tautan markdown `[path](path)`.
  - Mengabaikan inline code (`` `code` ``) dan blok kode (```` ```code``` ````).
- **Renderer Tautan Kustom**:
  - Komponen `a` kustom di `ChatMarkdown.tsx` memeriksa `extractConversationRoomId(href)`. Bila cocok, komponen mengganti anchor HTML standar dengan `<RoomCard roomId={roomId} className="my-2" />`.
  - Fungsi `messageNavigation` pada `MessageContent.tsx` mendeteksi tautan navigasi di akhir pesan dan merender `<RoomCard key={action.href} roomId={roomId} />`.
- **Dukungan Sinkronisasi Rute (`App.tsx`)**:
  - Rute `/chat/:roomId` disinkronkan langsung ke alur `/inbox/:roomId` agar pengguna selalu diarahkan ke tampilan chat yang tepat.

### C. Komponen Visual & Informasi Ruang Obrolan (`RoomCard.tsx` / `ChatPreviewCard.tsx`)
- **Indikator Jenis Ruang**:
  - Badge `Customer Room` (Biru / icon `MessageSquare`) untuk pelanggan toko.
  - Badge `Agent Room` (Ungu / icon `Bot`) untuk sesi agen internal.
- **Indikator Status & Waktu**:
  - Status online real-time (`Online` dengan pulsing green dot).
  - Relative time stamp (misal: "Baru saja", "5 mnt lalu").
- **Ringkasan Percakapan**:
  - Menampilkan nama kontak / judul ruang obrolan.
  - Menampilkan snippet pesan terakhir (`last_message`).
  - Menampilkan nama peserta / agen terkait (`agentDisplayName`).

---

## 3. Standar Aksesibilitas WCAG & Keamanan Mobile (360px–390px)

1. **Target Sentuh Minimum WCAG 2.1 AA (≥ 44x44px)**:
   - Kontainer kartu memiliki kelas `min-h-[44px]`.
   - Tombol aksi tombol "Salin tautan" (`Copy link`) dan "Buka ruang" (`Open room`) memiliki dimensi sentuh minimal `min-h-[44px] min-w-[44px]`.
2. **Keamanan Viewport Mobile (360px–390px)**:
   - Kontainer menggunakan kombinasi `w-full max-w-full min-w-0` untuk mencegah *horizontal layout shift* atau *horizontal scrollbar blowout*.
   - Tipografi menggunakan `break-words` dan `line-clamp-2` untuk menangani nama atau pesan yang sangat panjang.
   - Header dan footer kartu menggunakan `flex-wrap` untuk adaptasi layar sempit.

---

## 4. Isolasi Event Bubbling (Double Navigation Prevention)

Untuk mencegah konflik antara klik kartu (yang menavigasi ke ruang obrolan) dan klik tombol aksi internal (seperti menyalin tautan):
- **Tombol Salin Tautan (`handleCopyLink`)**:
  - Memanggil `e.preventDefault()` dan `e.stopPropagation()`.
  - Menulis URL ke clipboard tanpa memicu navigasi kartu induk.
- **Tombol Buka Ruang (`handleOpenRoom`)**:
  - Memanggil `e.preventDefault()` dan `e.stopPropagation()`.
  - Melakukan navigasi tunggal secara deterministik.
- **Navigasi Keyboard**:
  - `handleKeyDown` memvalidasi `if (e.target !== e.currentTarget) return` agar fokus keyboard pada tombol dalam tidak memicu event ganda.
  - Tombol-tombol internal mengikat `onKeyDown={(e) => e.stopPropagation()}`.

---

## 5. Verifikasi Otomatis

Verifikasi spesifikasi ini tercakup dalam unit test:
```bash
# Menjalankan pengujian spesifik deep link dan event isolation
cd web && node --test tests/chat-preview-card-deep-link.test.mjs tests/clickable-cards-event-isolation.test.mjs
```
Seluruh 12 skenario pengujian mencakup deteksi regex, isolasi DOM event, target sentuh WCAG, responsivitas mobile, dan kompatibilitas lokalisasi bahasa Indonesia.
