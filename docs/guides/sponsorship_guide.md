# Panduan Operasional Sponsorship & Kurasi Anti-Judol Cuan

Dokumen ini merupakan panduan operasional komprehensif bagi pemilik instance (Owner) dan tim pengelola proyek Cuan mengenai tata cara kurasi sponsor, penegakan etika anti-judol, manajemen batch crowdfunding, serta prosedur sinkronisasi leaderboard dan arsip abadi repositori GitHub.

---

## 1. Filosofi & Gambaran Umum

Cuan menerapkan model *Public Crowdfunding Engine* berbasis **Pay-What-You-Want (PWYW)** dengan nominal minimal **Rp 10.000** melalui payment gateway Dojo (QRIS, Virtual Account, E-Wallet).

### Prinsip Utama Kemitraan Backer
1. **Zero-Friction Intake:** Calon sponsor tidak dibebani proses pendaftaran akun rumit. Cukup mengisi profil brand dan langsung diarahkan ke checkout terintegrasi Dojo.
2. **Keadilan Ranking Leaderboard:** Urutan peringkat ditentukan secara adil dan matematis: nominal kontribusi tertinggi menempati posisi teratas (`ORDER BY amount_idr DESC`), dan apabila nominal sama, sponsor yang menyelesaikan pembayaran lebih awal mendapatkan posisi lebih tinggi (`paid_at ASC`).
3. **Karantina Anti-Judol & Anti-Scam Ketat:** Cuan memberlakukan kebijakan **Zero-Tolerance** terhadap situs judi online, pornografi, dan skema penipuan. Status awal seluruh order terbayar adalah `pending_review`. Tidak ada profil sponsor yang dapat tayang di publik sebelum disetujui secara eksplisit oleh Owner melalui panel kurasi.
4. **Arsip Abadi (*Perpetual Proof of Backing*):** Setiap sponsor yang disetujui diabadikan dalam riwayat Git repositori GitHub (`README.md` dan direktori `sponsors/`) atas nama human author `Buchin <buchin@dropsugar.com>`.

---

## 2. Diagram Alur Kerja End-to-End (E2E Architecture)

Berikut adalah diagram arsitektur alur data dari pendaftaran waiting list hingga publikasi sponsor:

```d2
direction: right

visitor: Pengunjung / Backer {
  shape: person
}

landing: Landing Page PAS (/) {
  style.fill: "#f8fafc"
}

intake: Showcase & Intake (/sponsor) {
  style.fill: "#f8fafc"
}

dojo: Dojo Payment Gateway {
  style.fill: "#ecfdf5"
}

cuan_backend: Backend Go (cuan-server) {
  style.fill: "#eff6ff"
}

quarantine: Antrean Karantina (pending_review) {
  style.fill: "#fef3c7"
}

owner_panel: Panel Kurasi Owner (/admin/sponsors) {
  shape: rectangle
  style.fill: "#f3e8ff"
}

live_public: Leaderboard Live (/sponsor & API) {
  style.fill: "#dcfce7"
}

git_sync: Runner Git Sync (README.md & sponsors/v1.0.md) {
  style.fill: "#e0e7ff"
}

visitor -> landing: Isi email Waiting List
landing -> cuan_backend: POST /api/waiting-list
cuan_backend -> dojo: Webhook Trigger Autoresponder

visitor -> intake: Unggah profil & nominal PWYW
intake -> cuan_backend: POST /api/sponsors/checkout
cuan_backend -> dojo: Buat Order Dojo & Redirect QRIS
dojo -> visitor: Selesaikan pembayaran

dojo -> cuan_backend: Webhook POST /api/webhooks/dojo
cuan_backend -> quarantine: Verifikasi Secret & Masuk Karantina

quarantine -> owner_panel: Tinjau Brand & Preview Logo
owner_panel -> live_public: 1-Klik Approve (Atomic Live)
owner_panel -> git_sync: Sinkronisasi Otomatis README & Arsip
```

---

## 3. Tata Cara Kurasi Owner di Dashboard (`/admin/sponsors`)

Panel kurasi sponsor hanya dapat diakses oleh akun Pemilik Tunggal (*Single-Owner*) yang terautentikasi.

### Langkah-Langkah Kurasi:
1. **Akses Dashboard:**
   - Masuk ke dashboard internal Cuan di `/admin` atau langsung navigasi ke `/admin/sponsors`.
   - Pilih tab navigasi **Sponsors**.
2. **Pemeriksaan Antrean Moderasi (`pending_review`):**
   - Sistem menampilkan badge penghitung jumlah order yang membutuhkan perhatian:
     - `Pending Review`: Order terbayar yang menunggu keputusan Owner.
     - `Approved`: Sponsor aktif yang sedang tayang di leaderboard publik.
     - `Rejected`: Order yang ditolak karena pelanggaran etika/kebijakan.
3. **Verifikasi Profil & Anti-XSS Check:**
   - Periksa **Nama Brand**, **Slogan**, dan **URL Website**. Pastikan URL diawali dengan `https://` atau `http://` yang sah.
   - Periksa pratinjau **Logo**: Parser Cuan telah membersihkan skrip berbahaya dan tag berbahaya SVG (skrip, `onload`, `<foreignObject>`). Pastikan logo tidak memuat konten ofensif atau visual judol.
   - Periksa nominal pembayaran IDR yang telah diverifikasi lunas oleh Dojo gateway.
4. **Eksekusi Keputusan (1-Klik):**
   - **Tombol Approve:** Klik tombol hijau *Approve*. Status sponsor seketika berubah menjadi `approved`, langsung muncul pada API publik `/api/sponsors/current`, live di leaderboard `/sponsor`, serta memicu runner sinkronisasi `README.md`.
   - **Tombol Reject:** Klik tombol merah *Reject*. Dialog konfirmasi akan meminta alasan penolakan (misal: `"Pelanggaran etika anti-judol Cuan"`). Status berubah menjadi `rejected` dan sponsor dikarantina secara permanen dari publik.

---

## 4. Kebijakan Anti-Judol, Pornografi & Scam

Cuan adalah perangkat lunak open-source profesional yang digunakan oleh developer, agensi, dan perusahaan. Reputasi proyek dan para sponsor sah dijaga melalui protokol berikut:

### Kriteria Konten Terlarang
- **Judi Online / Kasino / Slot / Togel:** Segala situs, aplikasi, atau layanan yang menawarkan taruhan uang riil, permainan slot gacor, kasino daring, dan promosi perjudian.
- **Pornografi / Konten Dewasa:** Situs bermuatan pornografi, webcam dewasa, atau layanan esek-esek.
- **Skema Penipuan (Scam / Phishing):** Investasi bodong, penggandaan uang, skema piramida terlarang, atau malware.

### Kebijakan No-Refund Pelanggaran Etika
Syarat dan ketentuan checkout sponsor Cuan menyatakan secara tegas bahwa pendaftaran yang sengaja mempromosikan konten terlarang (judol/pornografi) akan ditolak tanpa pengembalian dana (*no-refund policy*). Dana tersebut disita sebagai kompensasi biaya administratif penanganan moderasi dan investigasi keamanan.

### Audit Activity Log
Setiap tindakan Owner dicatat dalam tabel `sponsorship_activities` dengan mencatat:
- Stempel waktu UTC (`created_at`)
- Identitas pelaku (`actor`, contoh: `owner@cuan.dev`)
- Jenis tindakan (`sponsor_approved`, `sponsor_rejected`, `order_created`, `order_paid`)
- Rincian alasan penolakan secara transparan.

---

## 5. Manajemen Batch Rilis (*Founding Batches*)

Sponsorship Cuan dikelompokkan dalam batch rilis untuk menciptakan urgensi pendanaan dan mempertahankan nilai historis bagi para pendukung awal.

### Struktur Batch
- **Batch v1.0 ("v1.0 Founding Backers"):**
  - Slug: `v1.0`
  - Kuota rilis awal menuju peluncuran Cuan General Availability.
  - Ditandai dengan badge khusus `🌟 Platinum Founding Backer`, `🥇 Gold Founding Backer`, `🥈 Silver Founding Backer`, dan `🥉 Bronze Founding Backer`.
- **Batch Masa Depan:**
  - Misalnya `v2.0` saat pengembangan fitur besar berikutnya dimulai.

### Penutupan & Pembukaan Batch via Database
Untuk menutup batch yang telah selesai dan membuka batch baru:
```sql
-- Tutup batch v1.0
UPDATE sponsorship_batches 
SET status = 'closed', closed_at = NOW() 
WHERE batch_slug = 'v1.0';

-- Buka batch v2.0
INSERT INTO sponsorship_batches (id, batch_slug, name, status, created_at)
VALUES ('batch_' || substr(md5(random()::text), 1, 12), 'v2.0', 'v2.0 Strategic Supporters', 'open', NOW());
```

---

## 6. Sinkronisasi Manual & Operasi CLI

Secara default, setiap aksi **Approve** di panel Owner otomatis menjalankan sinkronisasi ke berkas `README.md` dan direktori arsip abadi `sponsors/`. Namun, Owner juga dapat menjalankan sinkronisasi sewaktu-waktu secara manual.

### 1. Sinkronisasi via Web UI Dashboard
Pada panel `/admin/sponsors`, klik tombol **Sinkronkan README Sekarang**. Backend akan membaca data leaderboard terkini dari database dan memperbarui berkas secara idempotent.

### 2. Sinkronisasi via CLI Server
Jalankan flag `--sync-sponsors` pada binary `cuan-server`:
```bash
./bin/cuan-server --sync-sponsors
```
Output yang diharapkan:
```text
✓ Sinkronisasi selesai: README.md dan arsip sponsors/ berhasil diperbarui (sponsors: 12, files modified: 2)
```

### 3. Konfigurasi Environment Sinkronisasi
Variabel lingkungan berikut dapat disesuaikan pada berkas `.env`:

| Variabel | Default | Keterangan |
| --- | --- | --- |
| `CUAN_README_PATH` | `README.md` | Lokasi berkas README master repositori |
| `CUAN_SPONSORS_ARCHIVE_DIR` | `sponsors` | Direktori ledger arsip permanen Markdown |
| `CUAN_AUTO_SYNC_README` | `true` | Otomatis sinkron saat kurasi disetujui |
| `CUAN_SYNC_GIT_COMMIT` | `false` | Jalankan git commit otomatis pada environment production |
| `GIT_AUTHOR_NAME` | `Buchin` | Nama author human untuk riwayat commit |
| `GIT_AUTHOR_EMAIL` | `buchin@dropsugar.com` | Email author human |

### 4. Format Penanda Idempotent pada `README.md`
Sinkronisasi Cuan menggunakan penanda komentar HTML khusus agar tidak mengganggu seksi dokumentasi lain:
```markdown
## Backers & Sponsors
<!-- SPONSORS:START -->
| Peringkat | Brand & Tautan | Slogan | Kontribusi | Tier |
| :---: | :--- | :--- | :---: | :--- |
| 🥇 #1 | [**Kopi Code Studio**](https://kopicode.dev) | Solusi Deploy Mandiri Anti-Pusing | Rp 250.000 | 🌟 Platinum Founding Backer |
<!-- SPONSORS:END -->
```
Konten di luar penanda `<!-- SPONSORS:START -->` dan `<!-- SPONSORS:END -->` dijamin 100% aman dan tidak akan tertimpa.

---

## 7. Prosedur Tanggap Darurat & Troubleshooting

### Kasus 1: Webhook Dojo Tidak Diterima / Status Order Tidak Berubah
1. Periksa log server:
   ```bash
   grep "dojo" /var/log/cuan/server.log
   ```
2. Pastikan `DOJO_WEBHOOK_SECRET` pada `.env` Cuan identik dengan secret di dashboard merchant Dojo.
3. Cuan mendukung autentikasi via header `X-Dojo-Secret`, `X-Webhook-Secret`, atau query parameter `?secret=...`.
4. Jika order sukses terbayar di Dojo namun webhook gagal terkirim, Owner dapat menyetujui langsung melalui panel admin dengan mencocokkan `dojo_order_id`.

### Kasus 2: Percobaan Infiltrasi Judol Massal
1. Buka tab **Pending Review** pada `/admin/sponsors`.
2. Klik **Reject** pada setiap entri pelanggaran dengan menyertakan alasan.
3. Catat IP address pengirim dari log akses server (`/api/sponsors/intake`).
4. Blokir IP address penyerang pada level firewall (UFW / Cloudflare / Caddy).

### Kasus 3: Logo SVG Tidak Tampil di Browser Pengunjung
1. Cuan menyajikan logo melalui endpoint `/uploads/sponsors/logos/*`.
2. Pastikan file tersimpan di direktori `data/sponsors/logos/`.
3. Endpoint ini secara otomatis menyertakan security header:
   ```http
   X-Content-Type-Options: nosniff
   Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'
   ```
   Hal ini menjamin SVG tampil dengan aman tanpa risiko eksekusi JavaScript berbahaya.

---
*Panduan ini disusun sebagai bagian dari Acceptance Gate Sprint 2 Cuan v1.0.*
