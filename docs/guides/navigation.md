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
- <768 px: navigasi bawah Inbox/Tasks/Projects, header untuk Settings/menu akun,
  serta drawer navigasi. Dalam chat, Back mengembalikan daftar dan navigasi bawah.
- Drawer tertutup tidak ada di urutan Tab. Drawer/task detail membatasi fokus,
  Escape menutup, dan fokus dikembalikan jika elemen asal masih tersedia.
- Safe-area HP dan perubahan visible viewport dipertahankan untuk composer.

## Contacts ditunda

Tidak ada menu Contacts kosong pada rilis ini. Sesuai arahan PO, Contacts akan
menjadi CRM untuk pengunjung live chat, email identifier widget, tag, dan dasar
broadcast pada fase berikutnya. Kebutuhan dicatat di
[Contacts CRM](../plans/contacts-crm.md); belum diimplementasikan pada rilis UI ini.

## Verifikasi

`node docs/evidence/ui-refresh/browser-check.cjs` memakai build production nyata
serta API fixture terisolasi. Pemeriksaan mencakup 35 pasangan halaman/viewport,
legacy redirect, status agent, project-scoped tasks, archive/restore, modal/drawer,
Back/reload, preferensi tema/navigasi, dan error/retry. Tidak memanggil provider.
Bukti ada pada `docs/evidence/ui-refresh/`; audit awal pada
`docs/evidence/ui-ux-audit/`. Viewport Chromium bukan pengujian perangkat fisik.

## Production

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
