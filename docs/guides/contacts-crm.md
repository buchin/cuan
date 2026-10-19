# Contacts dan live chat

Buka **Contacts** di sidebar atau navigasi bawah. Pengunjung yang membuka widget
otomatis tersimpan sebagai kontak. Gunakan pencarian nama/email/produk/tag,
filter tag, pilih kontak, lalu **Buka percakapan** untuk membalas dari Inbox.
Pada HP, detail tampil sebelum daftar dan dapat ditutup dengan **Kembali ke daftar
kontak**. Tag dipisahkan koma, maksimal 20 tag masing-masing 40 karakter;
**Simpan tag** memberikan konfirmasi atau error dengan opsi mencoba kembali.

## Pasang widget

Pasang sebelum penutup `body`; panduan singkat juga tersedia langsung di halaman
Contacts melalui **Hubungkan live chat ke website**.

```html
<script src="https://cuan.dojo.cc/chat/widget.js"
        data-product="website-anda" defer></script>
```

`data-product` memisahkan identitas visitor per produk. Browser menyimpan guest ID
acak dan sesi per origin Cuan + produk. Pengunjung pasif yang belum pernah
membuka chat tidak membuat kontak. Sesi lama yang telah terhubung dilanjutkan
saat reload. Membersihkan penyimpanan browser menghasilkan kontak anonim baru.

## Email identifier (opsional)

Integrator dapat memasang `data-email` dan `data-name` dari pengguna login pada
script tag, dengan HTML escaping dari framework integrator. Alternatif setelah
widget tersedia:

```js
await window.$dojoChat.identify({ email: 'customer@example.com', name: 'Nama pelanggan' })
```

Shim Dojo yang sudah ada tetap tersedia (`open`, `close`, `toggle`, `setMessage`,
`startTopic`). Shim Crisp menerima antrean sebelum script dimuat maupun perubahan
setelahnya:

```js
window.$crisp = window.$crisp || []
window.$crisp.push(['set', 'user:email', ['customer@example.com']])
window.$crisp.push(['set', 'user:nickname', ['Nama pelanggan']])
```

Identify sebelum chat dibuka menyiapkan metadata; sesudah dibuka memperbarui kontak
browser/produk yang sama. Mengirim nilai kosong mempertahankan metadata tersimpan.
Email dinormalisasi dan divalidasi di server. Email ini **diklaim, belum
terverifikasi**; tidak menjadi autentikasi, tidak menggabungkan kontak browser
lain, dan tidak membuka riwayat hanya karena email sama. Saat logout/pergantian
akun pada browser bersama, jangan anggap widget mendukung pemisahan akun: reset
identitas/logout aman dan verifikasi server masih pekerjaan lanjutan.

## Persistensi dan batas

Kontak, identitas visitor/sesi, email dan tag tersimpan di PostgreSQL. Session UUID
acak adalah capability rahasia untuk percakapan visitor: jangan dipublikasikan.
Visitor tidak dapat memilih room agent/internal melalui session payload. API
internal `GET /api/v1/contacts` dan `PATCH /api/v1/contacts/:id` membutuhkan login.
Cuan saat ini menggunakan cakupan owner instalasi; produk adalah label sumber,
bukan isolasi multi-tenant. Directory ini masih memuat daftar seluruh kontak;
paginasi server diperlukan untuk skala besar.

Room lama tetap dapat dibuka oleh operator melalui Inbox. Tidak menebak identitas
atau melakukan backfill CRM dari judul room lama yang dapat berisi diskusi agent.
Widget menggunakan sesi visitor baru yang terdaftar; legacy widget yang sebelumnya
gagal inisialisasi tidak dianggap sudah memiliki identitas CRM.

Broadcast, opt-in/opt-out, deduplikasi lintas perangkat, verifikasi email, penggabungan
kontak, dan pemisahan pengguna yang bergantian pada browser belum tersedia.
Tag merupakan fondasi segmentasi, bukan persetujuan menerima broadcast.

## Deploy, smoke dan rollback

Migration `000022_contacts_crm` hanya menambahkan tabel/index; lakukan backup privat
tervalidasi dan pastikan Inbox/task tidak aktif sebelum migrasi dan restart.
Uji SQL pada schema terisolasi di database `cuan_s25`, lalu terapkan migration22
secara eksplisit dengan migrator standar. Auto-migrate production tetap nonaktif.
Aktivasi native `scripts/fural-deploy.sh --branch sprint26-agents` memerlukan schema22,
memverifikasi manifest, readiness dan aset, serta mendukung rollback otomatis.
Untuk rollback aplikasi, pertahankan schema22/data kontak dan jalankan activator:

```sh
sudo python3 /home/ubuntu/.local/share/cuan-s25/activate-release.py --rollback --schema-count 22
```

Jangan jalankan down migration atau restore database untuk rollback aplikasi biasa.
Smoke: Contacts pada desktop/tablet/HP, buka detail/tag, link ke chat, widget identity,
retry pengiriman dan reload. Pengujian fixture: `node docs/evidence/contacts-crm/browser-check.cjs`
dan `node docs/evidence/contacts-crm/widget-check.cjs`. Bukti production dicatat terpisah.
