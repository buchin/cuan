# Contacts CRM — kebutuhan lanjutan

Arahan PO 2026-10-19: **Contacts ditunda** dari rilis perbaikan navigasi/responsif.
Contacts nantinya menjadi CRM pelanggan, terpisah dari konfigurasi AI agent.

## Kebutuhan yang sudah dinyatakan

- Orang yang menghubungi melalui live chat otomatis menjadi kontak.
- Widget dapat menerima email identifier dari integrator, mengikuti pola widget
  Dojo yang sudah ada; kontrak aktual Dojo perlu dipelajari sebelum implementasi.
- Kontak memiliki tag untuk pengelompokan.
- Data dan tag menjadi dasar fitur broadcast pada fase berikutnya. Broadcast
  belum termasuk implementasi sekarang dan belum boleh dianggap siap digunakan.

## Perilaku yang perlu dirancang sebelum implementasi

- Hubungkan visitor/session/conversation dengan contact secara persisten;
  reconnect dan chat berulang tidak boleh membuat duplikat sembarangan.
- Bedakan identifier email yang diklaim widget dari identitas yang terverifikasi.
  Jangan menggabungkan akun atau memberikan akses riwayat hanya karena string
  email sama; kontrak autentikasi integrator perlu diputuskan.
- Pertahankan visitor anonim dan tentukan aturan penggabungan setelah identitas
  diketahui. Tentukan normalisasi, collision, tenant/owner scope dan asal kontak.
- Tentukan kontrak tag, pencarian, filter, timeline percakapan, dan akses data.
- Definisikan channel, opt-in/opt-out, segmentasi, antrean, batas pengiriman dan
  hasil delivery saat broadcast benar-benar masuk scope.

## Batas rilis UI sekarang

Tidak menambahkan menu Contacts kosong, mengganti daftar agent menjadi CRM,
atau menganggap API daftar percakapan sebagai penyimpanan CRM lengkap.
Navigasi saat ini berfokus pada Inbox, Tasks, Projects dan Settings → Agents.
