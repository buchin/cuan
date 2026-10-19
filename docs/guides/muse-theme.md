# Tema Muse

Cuan memakai tema **Muse** dari [tweakcn](https://tweakcn.com/themes/cmu77w83m000104kxbep145by?p=dashboard).
Aksen biru, permukaan netral, radius dasar 1.25rem, dan shadow transparan mengikuti
[registry resmi](https://tweakcn.com/r/themes/cmu77w83m000104kxbep145by).
Tema diterapkan pada halaman Cuan yang ada; parameter dashboard pada referensi
adalah contoh tampilan tema, bukan penambahan fitur dashboard baru.

## Penggunaan

Gunakan tombol tema pada sidebar untuk beralih light/dark. Pilihan yang sudah
tersimpan tetap berlaku. Chat, tabel, form, kartu dan navigasi menggunakan token
tema yang sama. Warna status seperti sukses/gagal tetap membedakan maknanya.

Geist Variable dan Geist Mono Variable disajikan sebagai aset lokal dari paket
Fontsource berlisensi SIL Open Font License. Font tidak memerlukan Google Fonts
atau koneksi ke tweakcn saat aplikasi dibuka. Teks dengan karakter yang tidak
tersedia menggunakan fallback sistem. Token serif asli dipertahankan sebagai
fallback dan tidak dipakai oleh komponen aplikasi saat ini.

## Pemeliharaan

Token OKLCH berada di `web/src/index.css`; kanal warna tetap mengikuti sumber
resmi tanpa konversi ke HSL. Konfigurasi Tailwind 3 menggunakan placeholder
`<alpha-value>` agar utilitas seperti `bg-primary/10` tetap berfungsi. Alias
`sidebar-background` memetakan token `sidebar` dari registry. Font, radius,
chart dan shadow dikonfigurasi di `web/tailwind.config.js`. CSS Markdown memakai
kanal OKLCH yang sama. Snapshot sumber: `docs/evidence/muse-theme/source.json`.

Tidak ada perubahan schema atau data untuk tema ini. Rollback aplikasi memakai
prosedur native release pada panduan lifecycle pesan, tetap pada schema 21.

## Verifikasi

Build TypeScript/Vite dan Chromium memverifikasi semua token warna light/dark
terhadap registry, radius, pemuatan font lokal, transparansi warna, serta regresi
chat Markdown/clipboard dan viewport 360/390/1280. Jalankan fixture terisolasi:

```sh
node docs/evidence/muse-theme/browser-check.cjs
```

Bukti fixture tersimpan di `docs/evidence/muse-theme/browser.json`.
