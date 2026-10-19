# Navigasi dan tampilan responsif Cuan

## Mulai bekerja

1. Buka **Inbox** dan pilih percakapan. Di desktop lebar, daftar dan chat tampil
   berdampingan. Di HP/tablet, pilih percakapan dari daftar lalu gunakan Back
   untuk kembali. Pencarian dan filter daftar tetap tersedia.
2. Ketik pesan. Pesan dan balasan memakai teks 16 px; tombol kirim 48 px.
   Untuk membuat task, gunakan **⋯ Task** di bawah pesan. Pada desktop tindakan
   ini tampil saat hover/fokus keyboard; task yang sudah tertaut tetap terlihat.
3. Buka **Tasks** untuk backlog global, atau **Projects → nama project** untuk
   board yang hanya memuat task project tersebut. Detail task dapat ditutup
   dengan Escape dan kembali ke project asal.

## Settings dan agent

Pengelolaan agent dipindahkan ke **Settings → Agents** (`/settings/agents`).
Tautan lama `/team` diarahkan ke lokasi baru. Proses tambah/login/periksa agent
memakai koneksi yang sama; perpindahan menu tidak menjalankan login atau provider.
Dari header chat, tombol **⋯** berlabel **Kelola koneksi agen** membuka agent yang
sesuai. **Kembali ke percakapan** mengembalikan Anda ke chat asal.

Status **Terhubung** hanya tampil bila pemeriksaan berhasil, memiliki timestamp
valid dan belum melewati masa berlaku lima menit. Status lain membedakan perlu
login, kuota terbatas, koneksi terganggu, belum diperiksa dan dinonaktifkan.
Pemeriksaan tidak dijalankan otomatis untuk membuat indikator menjadi hijau.
Detail waktu pemeriksaan dapat dibuka di pengaturan agent.

**Settings → Tampilan** menyimpan pilihan light/dark/sistem pada browser.
Navigasi desktop dapat diciutkan; pilihan tersebut juga tersimpan.

## Pengaturan project

Buka **Settings → Umum → nama project**, atau **Projects → nama project →
Project settings**. URL `/projects/:id/settings` dapat dibuka langsung/reload.
Detail repository ditampilkan dari data tersimpan. Form edit URL/branch,
domain/server/env disembunyikan karena API edit tersebut belum tersedia; tidak
ada tombol Save yang mengklaim menyimpan konfigurasi yang tidak didukung.

**Arsipkan project** mempertahankan data dan repository. Buka **Projects →
Archived → Unarchive** untuk memulihkan. Kedua tindakan meminta konfirmasi dan
melaporkan kegagalan; arsip tidak diberi label Delete.

## Perangkat dan keyboard

- ≥1200 px: navigasi berlabel 200 px, bisa diciutkan menjadi 64 px. Panel daftar
  percakapan 280 px hanya pada Inbox dan bisa disembunyikan.
- 768–1199 px: navigasi 64 px, tanpa panel kedua permanen. Inbox memakai satu
  tampilan daftar/chat agar area kerja tetap luas.
- <768 px: navigasi bawah Inbox/Contacts/Tasks/Projects, header untuk Settings/menu akun,
  tanpa hamburger/drawer duplikat. Dalam chat, Back mengembalikan daftar dan navigasi bawah.
- HP memakai empat tab bawah serta Settings/menu akun di header. Drawer navigasi
  dihapus. Task detail tetap membatasi fokus, mendukung Escape dan pengembalian fokus.
- Safe-area HP dan perubahan visible viewport dipertahankan untuk composer.

## Contacts CRM

Contacts tersedia sebagai direktori CRM pengunjung live chat, terpisah dari
Settings → Agents. Mendukung email identifier widget, tag, pencarian/filter dan
Buka percakapan. Lihat [panduan Contacts](contacts-crm.md). Broadcast belum tersedia.

## Verifikasi

`node docs/evidence/ui-refresh/browser-check.cjs` memakai build production nyata
serta API fixture terisolasi. Pemeriksaan mencakup 35 pasangan halaman/viewport,
legacy redirect, status agent, project-scoped tasks, archive/restore, modal/drawer,
Back/reload, preferensi tema/navigasi, dan error/retry. Tidak memanggil provider.
Bukti ada pada `docs/evidence/ui-refresh/`; audit awal pada
`docs/evidence/ui-ux-audit/`. Viewport Chromium bukan pengujian perangkat fisik.

## Rilis UI sebelumnya

Release `48c2d68` aktif di https://cuan.dojo.cc sejak 2026-10-19. Schema tetap 21,
auto-migrate nonaktif. Backup privat tervalidasi sebelum native atomic activation.
Chromium production memverifikasi 35 pasangan route/viewport, status koneksi,
Settings → Agents dan kembali ke chat, direct URL project settings/reload,
Back chat, drawer keyboard, modal task, preferensi navigasi/tema, serta semua aset
publik identik dengan build. Tidak mengubah project, mengirim pesan baru atau
menjalankan provider saat smoke test. Checkpoint dan screenshot tersimpan di
`docs/evidence/ui-refresh/production-browser.json` dan `release.json`.

Untuk rollback aplikasi, pertahankan schema21 dan gunakan activator native dari
panduan lifecycle. Release sebelumnya `9708a0d` tetap dipertahankan.

Rilis Contacts berikutnya menggunakan schema22. Panduan terbaru: [Contacts CRM](contacts-crm.md).

## Integrations dan tema HP

Setup widget berada di **Settings → Integrations → Live Chat**
(`/settings/integrations/live-chat`), dengan daftar integrasi pada
`/settings/integrations`. Contacts fokus pada data pelanggan; tombol pemasangan
hanya muncul pada empty state dan menuju halaman pengaturan tersebut.

Header HP tidak memiliki toggle tema duplikat. Gunakan **Settings → Tampilan**
(`/settings/appearance`) untuk Light, Dark atau Ikuti sistem. Pilihan disimpan di
browser dan berlaku setelah reload. Shortcut tema desktop tetap tersedia.

Rilis `40938ea` aktif di production, schema tetap22 tanpa migration baru. Pemeriksaan
production16 pasangan halaman/viewport lulus, termasuk tombol terbawah yang tidak
tertutup navigasi HP, deep link/reload integrasi, salin kode dan persistensi tema.
Bukti: `docs/evidence/integrations-navigation/production-browser.json`.

## Navigasi HP tanpa drawer

Di bawah768px, gunakan Inbox/Contacts/Tasks/Projects pada menu bawah. Header
menyediakan Settings dan menu akun. Saat chat terbuka, Back mengembalikan daftar
dan menu bawah. Tidak ada hamburger, drawer atau item Tab tersembunyi. Sidebar
64px tablet dan sidebar desktop yang dapat dilipat tetap tersedia.

Rilis `4d62cfb` aktif di production. Build dan fixture48 pasangan halaman/viewport
serta production48 pasangan halaman/viewport lulus. Semua tab bawah, Settings, akun,
Back chat, resize dan sidebar desktop/tablet terverifikasi tanpa product write
atau pemanggilan provider. Schema22 tidak berubah; bukti `docs/evidence/mobile-navigation/`.

## Daftar agen yang ringkas

Settings → Agents menampilkan nama, status singkat, tombol **Periksa koneksi**
dan menu **⋮**. Status yang belum pernah diperiksa dibedakan dari hasil yang
kedaluwarsa. Halaman ini tidak otomatis menjalankan provider atau login ulang.
Detail pemeriksaan hanya tampil bila ada waktu pemeriksaan/hasil tersimpan;
penjelasan status dan waktu berada di dalam detail tersebut.

Menu tindakan menyediakan **Hapus** dengan konfirmasi. Batal tidak mengirim
permintaan hapus; error ditampilkan agar dapat dicoba kembali. Menu dapat ditutup
dengan Escape, klik di luar, atau memindahkan fokus keluar.

Header HP menyembunyikan ikon Settings saat sedang di Settings. Tombol kembali
ke percakapan hanya muncul bila halaman dibuka dengan parameter returnTo yang
valid dan mengarah ke Inbox di aplikasi yang sama. Navigasi Settings biasa
menampilkan daftar agen tanpa tombol kembali yang tidak memiliki sumber.
