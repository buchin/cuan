# Walkthrough UI: Projects & Project Detail (Cuan Sprint 1 — Task T3)

Dokumen ini mendokumentasikan implementasi antarmuka pengguna (Frontend React + Vite + TypeScript + Tailwind + shadcn/ui) untuk fitur **Hubungkan Repository GitHub Existing** pada Task T3.

---

## 1. Navigasi & Struktur Tab

Antarmuka utama Cuan kini memiliki navigasi terstruktur di bawah header:
- **Projects (N):** Halaman manajemen proyek dan repositori GitHub yang terhubung.
- **Tasks Board (N):** Papan daftar task sprint (T1–T8) terproteksi.
- **System Overview:** Ringkasan kesehatan sistem (Go Chi Backend, PostgreSQL persistent volume, Single-Owner Lock, dan Proxy HTTPS).

---

## 2. Halaman Daftar Proyek (Projects List)

Saat tab **Projects** aktif dan belum ada proyek yang dipilih:
- Menampilkan judul **"Daftar Project GitHub"** dan tombol aksi **"+ Hubungkan Repository GitHub"**.
- Menampilkan kartu (Card) untuk setiap repositori yang terhubung:
  - **Nama Proyek & Repo GitHub:** misalnya `Cuan Monorepo (buchin/cuan)`.
  - **Tautan Eksternal:** Menuju repositori remote di `https://github.com/buchin/cuan` dengan ikon eksternal.
  - **Default Branch Badge:** Menampilkan branch rujukan utama dengan ikon `GitBranch` (misalnya `main`).
  - **Status Kredensial:**
    - Jika repo public tanpa token: badge netral `Public (Tanpa Token)`.
    - Jika repo dengan token: badge hijau ber-border `ghp_***5544` (Tersimpan Terenkripsi AES-256).
  - **Status Sinkronisasi:**
    - `synced`: Badge hijau dengan indikasi clone/fetch mutakhir.
    - `connected`: Badge biru saat koneksi berhasil diverifikasi.
    - `failed`: Badge merah jika remote tidak dapat dihubungi.
  - **Tombol Aksi Cepat:**
    - `Sync Remote`: Melakukan fetch instan dari remote repository.
    - `Buka Detail`: Membuka halaman Project Detail yang komprehensif.

---

## 3. Form Modal "Hubungkan Repository GitHub"

Form modal interaktif (`isConnectModalOpen`) dirancang intuitif dan informatif:
1. **URL Repository GitHub (Wajib):**
   - Mendukung format lengkap `https://github.com/buchin/cuan` atau shorthand `buchin/cuan`.
2. **Nama Project (Opsional):**
   - Default otomatis menggunakan nama repository jika dikosongkan.
3. **Default Branch (Opsional):**
   - Otomatis terdeteksi dari GitHub remote (`main` / `master`) jika tidak diisi manual.
4. **Personal Access Token (PAT) (Opsional/Wajib untuk Private):**
   - Field password terlindungi.
   - Disertai tautan interaktif **"Panduan Token & Revoke"** yang dapat dibuka tanpa meninggalkan form modal.
5. **Banner Keamanan Kredensial:**
   - Menjelaskan jaminan enkripsi database AES-256-GCM, zero-leak di log server, dan zero plain text di API.
6. **Error Handling Informatif di Form:**
   - **Kasus 404:** Alert merah menampilkan pesan: *"Repository tidak ditemukan di GitHub. Pastikan nama owner/repo sudah benar dan jika repo bersifat private, sertakan Personal Access Token (PAT)."*
   - **Kasus 401/403:** Alert merah menampilkan pesan: *"Akses ke repository GitHub ditolak (401/403). Periksa kevalidan Personal Access Token (PAT) dan izin scope 'repo'."*

---

## 4. Halaman Project Detail

Ketika pengguna mengklik kartu proyek atau tombol **"Buka Detail"**:
- **Tombol Navigasi:** `← Kembali ke Daftar Proyek` di sudut kiri atas.
- **Header Card:**
  - Nama proyek, status badge (`synced`), project ID sistem, dan tombol cepat `Lihat di GitHub`.
  - Tiga panel ringkasan:
    1. *Repository Remote:* Owner, nama repo, dan URL HTTPS.
    2. *Default Branch:* Nama cabang utama yang menjadi rujukan eksekusi task agent.
    3. *Keamanan Kredensial:* Masked preview token (`ghp_***xxxx`) dan konfirmasi enkripsi AES-256-GCM.
- **Panel Remote Branches:**
  - Memanggil endpoint `/api/v1/projects/{id}/branches` secara otomatis.
  - Menampilkan seluruh cabang remote yang aktif (misal `main`, `staging`, `feature/xyz`). Branch default ditandai secara khusus dengan highlight hijau.
- **Panel Audit Keamanan & Panduan Revoke:**
  - Tiga poin jaminan keamanan terverifikasi (Enkripsi At-Rest, Zero Log Leak, Non-Destructive Delete).
  - Tombol accordion untuk membuka panduan langkah-langkah mencabut (revoke) token di GitHub Settings kapan saja.
- **Aksi Sinkronisasi:**
  - Tombol `Sinkronisasi Sekarang` memicu fetch update terbaru dari GitHub dan memperbarui timestamp sinkronisasi.

---

## 5. Proteksi Penghapusan Proyek (Non-Destructive Delete)

Ketika pemilik menekan tombol merah **"Hapus Proyek dari Cuan"**:
- Modal konfirmasi muncul dengan peringatan jelas:
  > **Jaminan Keamanan Remote Repository:**
  > Penghapusan ini **HANYA** menghapus data proyek dan cache lokal dari instalasi Cuan Anda.
  > Repository GitHub remote Anda **TETAP UTUH DAN TIDAK AKAN DIUBAH ATAU DIHAPUS**.
- Memerlukan klik eksplisit pada tombol **"Ya, Hapus dari Cuan"**.
- Setelah dihapus, Cuan membersihkan rekaman database dan direktori cache lokal proyek, lalu kembali ke daftar proyek dengan notifikasi konfirmasi.

---

## 6. Verifikasi Build & Responsivitas

- Bundle frontend telah di-compile secara sukses menggunakan Vite & TypeScript tanpa error:
  `dist/assets/index-C37D6TV1.js (212.83 kB)` dan `dist/assets/index-CcdyimWY.css (22.50 kB)`.
- UI sepenuhnya responsif (kompatibel desktop dan mobile layar sempit).
