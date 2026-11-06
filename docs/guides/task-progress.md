# Progres pekerjaan

Pie kecil di samping judul percakapan atau nama project menunjukkan bagian tugas
yang selesai. Pie muncul setelah ada tugas selesai. Di Inbox, percakapan dengan
satu task tidak memakai pie karena statusnya sudah tersedia pada task card.

Ketuk atau klik pie untuk membuka rincian jumlah tugas. Di desktop, mengarahkan
pointer juga membuka rincian. Angka tersedia untuk pembaca layar dan tombol dapat
dibuka dengan Enter, ditutup dengan Escape atau ketukan di luar panel.

- Hanya tugas **Selesai** mengisi pie. Tugas yang masih direviu belum selesai.
- Tugas dibatalkan dikeluarkan dari pembagi, tetapi tetap tercantum dalam rincian.
- Semua tugas selesai tanpa pembatalan menjadi centang hijau lembut.
- Jika pekerjaan tersisa selesai tetapi ada pembatalan, indikator menjadi lingkaran
  penuh netral. Rinciannya mengatakan **Semua pekerjaan tersisa selesai** dan
  menunjukkan jumlah tugas dibatalkan.
- Progres 0%, tanpa tugas, atau tanpa data tidak menampilkan pie.
- **Perlu perhatian** tetap terlihat meski progres 0%. Status bekerja dan review
  juga tetap tersedia. Pembatalan ditampilkan sebagai dibatalkan, bukan kegagalan.
- Menambah atau membuka kembali pekerjaan dapat menurunkan progres.

Perhitungan memakai ringkasan seluruh tugas, termasuk yang belum dimuat dalam
halaman. Pie bukan perkiraan waktu atau persentase langkah agent.

Cuplikan Inbox merangkum laporan sistem dengan bahasa singkat. Rincian laporan
asli tetap tersedia dalam percakapan. Eksekusi yang dibatalkan bisa menyimpan hasil
parsial sebagai review; indikator memakai kondisi eksekusi terbaru agar pembatalan
tersebut tidak keliru ditampilkan sebagai tugas yang menunggu review.

Menu tindakan percakapan tersedia lewat hover desktop, tekan lama di mobile, atau
navigasi keyboard. Ketukan biasa tetap membuka percakapan; mengetuk pie hanya
membuka rincian progres.

Dalam chat, ekor kecil muncul pada bubble pertama dari pengirim yang sama.
Pergantian pengirim, hari, atau jeda lima menit memulai rangkaian baru. Preview
pekerjaan dan pesan yang hanya berisi lampiran tidak memakai ekor.

Execution progress always belongs to the latest attempt. Retrying a task clears
its current progress until that attempt reports or emits useful stdout. Earlier
reports remain in its history. A queued attempt never borrows a previous run's
“finished” message. Detail and list use the same progress source.

### Result identity when publication fails

The native runner retains the independently observed clean workspace commit even
when production verification fails. This identifies the saved result for release
reconciliation; it does not establish publication or task acceptance. Dirty or
unreadable workspaces do not gain this identity. The original failed attempt and
publication error remain visible, and a verified deployment receipt or independent
registered release observation is still required before acceptance.
