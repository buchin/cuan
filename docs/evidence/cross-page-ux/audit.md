# Audit dan perbaikan UI/UX lintas halaman

Audit read-only production mencakup 21 pasangan route/viewport (1440, 820, 390 px):
Contacts, Tasks, Projects, Settings, Agents, Integrations dan Tampilan.
Tidak ada overflow halaman pada data production saat audit, tetapi beberapa
masalah terlihat dari screenshot dan inspeksi komponen.

| Area | Temuan | Perbaikan |
| --- | --- | --- |
| Inbox | Identitas kepeng hilang di header mobile | Mark 24px di kiri Inbox, tinggi tetap56px |
| Contacts | Padding shell dan halaman bertumpuk; judul mobile24px | Satu padding shell, skala judul20/24px yang konsisten |
| Contacts kosong | Pencarian/filter tidak berguna tanpa kontak | Fokus ke penjelasan dan Hubungkan live chat |
| Tasks mobile | Kolom Code mempersempit judul; judul panjang bisa memenuhi layar | Code pada metadata, judul16px maksimal3baris; detail menampilkan lengkap |
| Tasks kosong | Selalu menyebut filter meski belum ada task | Empty state membedakan belum ada vs tidak cocok, CTA yang relevan |
| Form task | Bukan modal native, kontrol/label kecil, fokus bisa keluar | Native dialog + focus trap, label terlihat,44px controls, scroll, Escape/focus restore |
| Projects | CTA berada setelah pencarian; hasil kosong mengklaim belum ada project | CTA dekat judul, search/tool row ringkas, empty search akurat + Clear search |
| Projects list | Baris hanya clickable div | Tombol asli untuk nama dan buka project, mendukung keyboard |
| Settings | Link bagian membungkus dua baris di390px | Navigasi satu baris, scroll horizontal bila perlu, target44px |

Integrations dan Agents tetap memakai alur saat ini; tidak ditambahkan pemeriksaan
provider otomatis. Tidak ada perubahan API/schema atau mekanisme eksekusi task.
Form task tetap menyimpan ke Todo seperti sebelumnya, dan tidak menjalankan agent.

Verifikasi fixture memakai build production nyata dan mencakup64 pasangan
route/viewport/tema, data task panjang, dialog keyboard/focus, simpan gagal/retry,
project search/list keyboard, Contacts detail/empty CTA. Screenshot sebelum
menggunakan data production; screenshot fixture sesudah tidak merepresentasikan
data pelanggan. Browser memakai Chromium emulation, belum sertifikasi Safari
atau perangkat fisik.
