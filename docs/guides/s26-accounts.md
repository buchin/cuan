# Akun provider Cuan — pratinjau S26.1

## Status

Tersedia di staging https://cuan-s25-dojo.fural.space/team. Production tetap pada rilis S25. Fitur pemilihan akun per agen belum diaktifkan karena gate login akun kedua belum terpenuhi; tombol Tambah agen lama masih menggunakan akun server lama. Jangan menganggap tersimpannya label akun sebagai bukti Google login atau provider siap.

## Menambahkan akun Google lain

1. Masuk ke Cuan staging, buka Team → Akun provider.
2. Isi nama akun sebagai label yang mudah dikenali, misalnya Google kerja. Ini bukan email yang sudah diverifikasi.
3. Klik Tambah akun Google. Akun dan profil privatnya disimpan; login baru tidak memakai kredensial akun lama dan tidak memerlukan akun lama lolos verifikasi kuota.
4. Buka login Google. Google diminta menampilkan pilihan akun; pilih akun lain yang memang ingin dihubungkan, kemudian selesaikan consent.
5. Tempel kode otorisasi hanya pada kolom kode dalam Cuan, lalu Simpan dan verifikasi login. Jangan kirim kode di chat/terminal.
6. Login dan ketersediaan provider diperiksa terpisah. Kuota/gangguan provider tidak menghapus hasil login. Jika kredensial sudah ada, gunakan Verifikasi akun; untuk akun Google lain buat akun baru.

Batalkan login menghentikan sesi tersebut tanpa menghapus akun/kredensial akun lain. Sesi OAuth berlaku lima menit; setelah kedaluwarsa klik Login Google pada akun yang sama. Nama akun dan profil tetap ada setelah reload/restart. Belum ada fitur hapus akun. Beberapa agen yang kelak memilih akun sama berbagi kuota; menambah label/agen bukan menambah kuota.

## Status dan batas pratinjau

Perlu login berarti kredensial belum tersedia. Belum diverifikasi berarti tidak ada bukti koneksi terkini. Terverifikasi memerlukan probe provider nyata dan menjadi kedaluwarsa setelah lima menit. Kuota dibatasi dan Provider belum tersedia mempertahankan kredensial; retry login bukan pemulihan kuota. Identitas email/Google tidak ditampilkan jika belum diketahui secara faktual. Setelah kode berhasil dipertukarkan, kegagalan provider harus dibaca terpisah dari penyimpanan kredensial.

## API pemilik

Semua endpoint memerlukan sesi owner melalui Bearer/cookie yang sudah didukung Cuan. Account ID adalah opaque; path/owner/kredensial tidak disertakan dalam DTO.

| Metode dan path | Kontrak |
| --- | --- |
| GET /api/v1/provider-accounts | `{accounts: [...]}`; hanya akun owner dan representasi akun server lama yang sudah ada. |
| POST /api/v1/provider-accounts | `{label}` → 201 `{account}`; label wajib, maksimal 100 karakter. |
| POST /api/v1/provider-accounts/:id/auth-init | `{account_id, session_id, auth_url, expires_at, authenticated:false, verified:false}`. Sesi baru hanya untuk profil tanpa kredensial; akun yang sudah login tidak ditimpa. |
| POST /api/v1/provider-accounts/:id/auth-code | `{session_id, code}` → `{account, credentials_saved}` setelah hasil login disimpan dan probe dilakukan. Quota/unavailable tetap respons akun dengan kredensial tersimpan, bukan keberhasilan provider palsu. |
| POST /api/v1/provider-accounts/:id/auth-code?cancel=true | `{session_id, code:""}` → `{status:"cancelled"}`. |
| POST /api/v1/provider-accounts/:id/verify | `{account}` dengan status faktual dan waktu pemeriksaan. |

401 meminta login owner; 404 menyamarkan akun/sesi yang tidak dimiliki; 400 input/kode tidak valid; 409 memulai OAuth ditolak/tidak tersedia; 500 penyimpanan/status gagal. Error provider disanitasi, bukan raw CLI output. Endpoint OAuth lama tidak boleh menerima atau membatalkan sesi akun baru.

## Penyimpanan dan operasi

Akun baru menggunakan profil privat di service HOME/.local/share/cuan/accounts/acct_<opaque>/home, dengan metadata account.json di direktori induk. Root/profile 0700; metadata 0600 ditulis atomik melalui temporary file + fsync/rename. Kredensial tetap CLI-owned di profil; tidak masuk DB DTO, web, evidence atau git. Root di service HOME berada di luar direktori release sehingga aplikasi rollback tidak menghapus akun. Pertahankan/backup direktori privat ini dengan kontrol akses yang sama seperti kredensial lama sebelum release S26; pg_dump sendiri tidak mencakup file tersebut.

Metadata menggunakan file konfigurasi privat, bukan kolom DB yang tidak terkait. S26.1 tidak membutuhkan migrasi baru; 19 migrasi lama tetap utuh. Konfigurasi HOME/XDG/token/profile diberikan hanya kepada child process, tanpa mengubah HOME global. Child environment menyaring secret organisasi dan provider lain. OAuth/probe profil baru dimulai dari direktori profil untuk menghindari konfigurasi proyek lain. Pembatalan/timeout menghentikan process tree profil. Akun server lama tidak dipindah/dihapus; override path yang sudah dikonfigurasi hanya dipertahankan untuk akun lama.

Staging diterbitkan memakai fural-agent sandboxes serve ke sandbox yang sudah ada, slug cuan-s25, port 8081; directory overlay tidak berisi credential. Jangan restart staging saat owner sedang consent. Production tidak boleh dirilis sebelum login kedua nyata, isolasi eksekusi terpilih, dan gate S26 lain lulus. Jika perlu rollback pratinjau, publish source/bundle versi sebelumnya melalui mekanisme staging yang sama; biarkan file akun privat tetap ada. Tidak ada restore DB atau down migration.

## Verifikasi

Bukti ada di docs/sprints/evidence/S26/: targeted.txt, real-isolated-oauth.txt, frontend-build.txt, staging-accounts-browser.json dan continuity.json. Dua profil CLI nyata menghasilkan URL resmi/sesi berbeda, tetapi tidak ada consent/kode/provider reply kedua yang diklaim. Browser Chromium 390px membuktikan form, link, cancel, reload, akun lama tetap ada, dan tanpa overflow horizontal; bukan pengujian keyboard fisik.


## Alur Tambah agen yang disederhanakan

Mengikuti referensi Fural yang dibaca saja (coding-agents-manager.tsx forceNew dan OrganizationAgentsNewPage addMode), Tambah agen selalu membuka formulir nama unik dan login Google baru. Modal tidak memeriksa/memakai akun server lama secara otomatis. Klik Login dengan Google, buka link resmi, dan tempel kode pada modal yang sama. Kelola akun Google menjadi bagian opsional yang bisa dibuka, bukan langkah wajib berpindah panel. Pendaftaran agen dengan akun baru masih bergantung pada gate login nyata dan pemetaan akun S26 berikutnya; pratinjau tidak pernah mengarahkan agen baru diam-diam ke akun lama.

Error Failed to save agent connection ditemukan setelah agen dihapus tetapi koneksi dengan ID deterministik conn_agent_dev masih tersimpan. Pendaftaran sekarang menggunakan ID koneksi unik baru; koneksi lama/history tidak dihapus. Targeted regression membuktikan buat–hapus–buat ulang tetap berhasil dengan koneksi lama dipertahankan. Ini bukti penyimpanan dengan fixture, bukan keberhasilan login/provider.
