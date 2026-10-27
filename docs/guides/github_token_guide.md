# Panduan Konfigurasi & Pencabutan (Revoke) GitHub Personal Access Token (PAT)

Dokumen ini memandu pemilik instance Cuan (single-owner) dalam membuat, mengonfigurasi, dan mencabut (revoke) Personal Access Token (PAT) GitHub untuk menghubungkan repository existing ke Cuan.

---

## 1. Kapan Personal Access Token (PAT) Dibutuhkan?

- **Repository Public:** PAT bersifat **opsional**. Cuan dapat melakukan clone, fetch, dan inspect branch publik tanpa token. Namun, memberikan PAT tetap dianjurkan untuk menghindari rate-limit GitHub API anonim (60 request/jam).
- **Repository Private:** PAT **wajib disertakan** agar Cuan dapat memvalidasi repositori, memeriksa remote branches, dan melakukan clone/fetch workspace.

---

## 2. Cara Membuat Personal Access Token di GitHub

Anda dapat menggunakan **Classic Token** (paling mudah) atau **Fine-grained Token** (akses tersegmentasi).

### Opsi A: Personal Access Token (Classic) — Direkomendasikan
1. Masuk ke akun GitHub Anda, buka [GitHub Personal Access Tokens](https://github.com/settings/tokens).
2. Klik tombol **Generate new token** -> **Generate new token (classic)**.
3. Beri nama token pada kolom **Note**, misalnya: `Cuan Runner - Dogfooding`.
4. Pilih masa berlaku (**Expiration**), misalnya `30 days` atau `90 days`.
5. Centang scope yang diperlukan:
   - `repo` (Full control of private repositories) — diperlukan untuk clone, fetch, branch checkout, dan pembuatan branch/PR pada task runner.
6. Klik **Generate token** di bagian bawah halaman.
7. Salin token (berawalan `ghp_...`). **Catatan:** Token ini hanya ditampilkan satu kali oleh GitHub.

### Opsi B: Fine-grained Personal Access Token
1. Buka [GitHub Fine-grained Tokens](https://github.com/settings/tokens?type=beta).
2. Klik **Generate new token**.
3. Pilih **Resource owner** (akun personal Anda atau organisasi yang menaungi repo).
4. Pada **Repository access**, pilih *Only select repositories* dan pilih repository yang akan dihubungkan ke Cuan (misal `cuan`).
5. Pada **Permissions** -> **Repository permissions**:
   - *Contents*: `Read and Write`
   - *Pull requests*: `Read and Write`
   - *Metadata*: `Read-only` (otomatis terpilih)
6. Klik **Generate token** dan salin token (berawalan `github_pat_...`).

---

## 3. Menghubungkan Repository ke Cuan

1. Buka dashboard Cuan, masuk ke menu **Projects**.
2. Klik tombol **Hubungkan Repository GitHub**.
3. Masukkan URL repository, misalnya: `https://github.com/buchin/cuan` atau `buchin/cuan`.
4. Tempelkan token PAT pada kolom **Personal Access Token (PAT)**.
5. Klik **Hubungkan Repository**. Cuan akan:
   - Memvalidasi eksistensi dan izin repository ke GitHub API.
   - Mendeteksi default branch (misal `main`).
   - Mengenkripsi token menggunakan **AES-256-GCM** sebelum disimpan ke database PostgreSQL.
   - Menginisialisasi clone/fetch lokal di bawah direktori data Cuan.

---

## 4. Cara Mencabut (Revoke) Token Kapan Saja

Anda memegang kendali penuh atas kredensial Anda. Kapan saja Anda ingin memutuskan akses Cuan ke akun GitHub Anda:

1. Buka halaman [GitHub Personal Access Tokens](https://github.com/settings/tokens).
2. Cari token yang Anda buat untuk Cuan (misal `Cuan Runner - Dogfooding`).
3. Klik tombol **Delete** atau **Revoke**.
4. Konfirmasi penghapusan token.
5. Seketika itu juga, semua akses clone/fetch remote menggunakan token tersebut akan ditolak oleh GitHub (401/403).
6. Di dashboard Cuan, status koneksi akan menampilkan indikator `failed / access denied` secara informatif tanpa menyebabkan server crash.

---

## 5. Jaminan Keamanan Kredensial di Cuan

1. **Enkripsi Database (At-Rest Encryption):**
   - Kredensial token tidak pernah disimpan dalam bentuk plain text di database.
   - Enkripsi menggunakan algoritma standar industri **AES-256-GCM** dengan salt dan derived key berbasis `CUAN_AUTH_SECRET`.
2. **Zero Plaintext Exposure di API:**
   - Endpoint API Cuan (`GET /api/v1/projects`, `GET /api/v1/projects/{id}`) tidak pernah mengirimkan token dalam bentuk plain text.
   - Respons API hanya memuat preview tersamar (misal `ghp_***4a2b`) dan flag boolean `has_token: true`.
3. **Penyaringan Log (Zero Log Leak):**
   - Runner dan Git client Cuan dilengkapi modul `agent.MaskSecrets`.
   - Output log dari eksekusi perintah `git clone`, `git fetch`, dan `git ls-remote` secara otomatis menyaring format token dan string kredensial sebelum dicatat ke log server.
4. **Proteksi Non-Destructive:**
   - Menghapus project dari database Cuan hanya membersihkan data lokal Cuan.
   - Cuan **tidak pernah** menjalankan perintah penghapusan atau perusakan terhadap remote repository di GitHub.
