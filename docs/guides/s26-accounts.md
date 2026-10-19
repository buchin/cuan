# Agen dan koneksinya

## Model penggunaan

**Satu agen = satu koneksi.** Pengguna mengelola satu daftar agen. Jenis agen, login dan status koneksi tampil pada agen tersebut; tidak ada daftar akun Google terpisah, pemilihan akun bersama atau baris percobaan login yang dibatalkan.

Pratinjau saat ini: https://cuan-s25-dojo.fural.space/team. Production masih rilis S25. Antigravity adalah jenis login yang sudah diintegrasikan pada alur baru. Command Code, Claude Code dan jenis lain memerlukan adapter login/eksekusi sendiri pada pekerjaan berikutnya; tidak ditampilkan sebagai opsi yang sudah siap.

## Menambahkan agen

1. Klik Tambah agen, lalu beri nama unik seperti @penulis.
2. Klik Hubungkan Google. Buka Google, pilih akun yang ingin digunakan, lalu selesaikan login.
3. Kembali ke Cuan dan tempel kode pada dialog yang sama. Klik Konfirmasi login.
4. Setelah login tersimpan, Cuan otomatis menyimpan agen beserta satu koneksi khusus dan kembali ke daftar agen. Jika penyimpanan gagal, gunakan Simpan agen untuk mengulang penyimpanan tanpa mengirim ulang kode.

Masukkan kode hanya di Cuan. Link berlaku lima menit; Buat link baru memulai ulang sesi. Batal, Escape atau Tutup membatalkan sesi tanpa membuat baris agen. Percobaan yang belum selesai tetap privat dan tidak ditampilkan sebagai akun/agen terhubung. Credential dan riwayat agen lain tetap tersimpan.

Daftar agen menampilkan jenis dan status faktual. Periksa memeriksa koneksi agen tersebut. Terhubung memerlukan probe nyata dan kedaluwarsa menjadi Belum diperiksa; Kuota habis atau Koneksi terganggu tidak menghapus login. Agen dapat tersimpan saat kredensial sudah ada tetapi provider sedang membatasi kuota; ini tidak berarti provider siap menjawab. Tidak ada badge Connected hanya karena agen terdaftar.

## Kontrak koneksi

S26 menggunakan endpoint profil privat yang sudah ada sebagai detail internal login, tanpa UI inventaris akun. Semua endpoint login/probe memerlukan owner. Setelah kode dipertukarkan, UI mengirim `POST /api/v1/agents/connect-cli` dengan `{name, handle, provider:"agy", account_id:<opaque>}`. Request tanpa profil login ditolak 409 dan tidak meminjam akun server. Sukses menghasilkan satu Agent dan AgentConnection; SQL menyimpan tautan melalui kontrak connection_id/model_id yang sudah ada. ID koneksi baru menggunakan conn_profile_<UUID>.

Metadata profil mengikat tepat satu owner, agent_id dan connection_id. Percobaan memakai profil tersebut untuk agen lain ditolak. Mengulangi request yang sudah sukses hanya mengembalikan binding/percakapan yang sama; tidak membuat koneksi/agen kedua. Kegagalan penyimpanan melepas binding baru, tanpa menghapus credential. Profil hilang, agent berbeda, owner berbeda atau jenis yang tidak sesuai gagal tertutup; tidak kembali ke akun lain.

`POST /api/v1/agents/:id/connection/verify` memeriksa koneksi milik agen dan mengembalikan `{agent_id, connection_state, message}` yang disanitasi. Roster GET agents menyertakan connection_state. Request tidak sah 401, akun/agen tidak ditemukan 404, konflik login/binding 409, gangguan storage 500. Secret/path/owner profil tidak dimasukkan ke roster atau pesan error.

## Penyimpanan dan kesinambungan

Profil login tetap CLI-owned di service HOME/.local/share/cuan/accounts/<opaque>/home, dengan metadata privat di direktori induknya. Nama direktori accounts adalah detail penyimpanan historis, bukan fitur akun terpisah. Root/profile 0700; metadata 0600 ditulis atomik dengan fsync/rename. Binding dan credential bertahan setelah restart dan berada di luar direktori release. Backup privat harus mencakup direktori ini; pg_dump sendiri tidak mencakup file profil. Percobaan tanpa login yang dibuat sebelum penyederhanaan tetap tidak terlihat dan tidak dihapus secara massal.

Agen lama @pin1 tetap menggunakan koneksi/credential yang sudah ada; tidak dipindahkan atau dihapus. Agen baru wajib memakai profil khusus. Inbox dan task PTY menyelesaikan binding agen lalu memberikan HOME/XDG/config profil tersebut hanya ke child process. Tidak ada os.Setenv HOME global. Child environment menyaring secret organisasi; profile HOME tidak dicampur dengan login agen lain. Login/probe menggunakan direktori profil, sementara task tetap memakai workspace tugas dengan env profil yang sesuai. Mengubah binary task untuk memakai jenis lain pada koneksi Antigravity ditolak.

S26.1 tidak memerlukan migration baru; 19 migration lama tetap utuh. Staging diterbitkan melalui publisher native pada port 8081. Rollback aplikasi tidak menghapus profil, credential atau DB; gunakan source/bundle sebelumnya dengan mekanisme yang sama dan pertahankan file privat. Tidak ada down migration/restore DB untuk rollback kode.

## Bukti dan batas penerimaan

`docs/sprints/evidence/S26/one-agent-targeted.txt` membuktikan binding eksklusif/ownership, retry, penyimpanan rollback metadata, profil yang benar pada child chat dan task PTY, serta penolakan fallback. Credential pada pengujian ini adalah fixture, bukan akun nyata.

`one-agent-ui.json` dan gambar one-agent-<width>-<theme>-roster/login.png membuktikan daftar agen tanpa panel akun, tidak memuat inventaris akun lama, official isolated Google OAuth URL, cancel tanpa baris baru dan tanpa overflow pada 360/390/1280px. Pengujian auto-save/retry frontend menggunakan respons FIXTURE yang diberi label, tidak membuat credential/agen di backend. Native build lulus.

Gate yang masih terbuka: consent pemilik akun kedua, pertukaran kode/identitas dan balasan provider nyata dengan profile terpisah. Tidak ada keberhasilan akun/provider kedua, release S26 production atau penyelesaian sprint yang diklaim dari fixture. S26.2–S26.5 tetap mengikuti gate yang sudah disetujui ulang dalam S26.md.
