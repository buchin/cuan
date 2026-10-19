# Agen dan koneksinya

## Verifikasi isolasi S26.2

SM menerima S26.1 di ed97cfa. s26-2-before.json dan s26-2-after.json membuktikan mapping serta balasan pin1/pin2 bertahan setelah restart independen. Pin2 menjadi Belum diperiksa setelah lima menit tanpa menghapus login. Pin1 quota_limited tidak perlu login ulang.

s26-2-targeted.txt membuktikan dua jalur HTTP Inbox dan dua PTY OS memakai HOME masing-masing melalui CLI/credential fixture. s26-2-task-history.json hanya membuktikan peluncuran dan exit 0 CLI AGY nyata di staging, bukan isi jawaban atau observasi HOME child nyata. Bukti identitas Google dan nonce nyata tetap terpisah. Resolver menolak provider koneksi/profil yang berbeda dari jenis agen; hardening S26.2 kini sudah diterbitkan ke staging bersama S26.3. Pemeriksaan HTTP nyata menolak mismatch provider profil dengan 409 dan memulihkan metadata byte-for-byte; regresi staging mencakup mismatch koneksi/owner dan jalur fixture. Versi aset diperiksa persis dan smoke mobile lulus. Detail: s26-3-version.json, s26-3-live-mismatch.json, s26-3-staged-targeted.json dan s26-3-browser.json (tanpa overlay lokal). Production belum diperbarui. Detail DoD ada di docs/sprints/S26.md.

## Membaca status per agen

Status adalah hasil pemeriksaan provider, bukan keberadaan baris agen atau login tersimpan. Setiap baris menampilkan jenis provider, hasil, tanggal/jam diperiksa, tanggal/jam berhasil dan batas berlaku bila ada. Keberhasilan berlaku lima menit, lalu menjadi Belum diperiksa, termasuk saat halaman dibiarkan terbuka atau kembali dari latar belakang. Klik Periksa untuk hasil baru; tidak perlu mengulang consent.

- Terhubung: provider benar-benar merespons pada pemeriksaan terakhir yang masih berlaku; tidak menjamin request berikutnya pasti berhasil.
- Belum diperiksa: belum ada keberhasilan yang masih berlaku. Credential dan riwayat tidak dihapus saat hasil kedaluwarsa.
- Perlu login: login belum tersedia atau binding koneksi tidak valid. Hubungi pengelola bila koneksi tidak sesuai; jangan meminjam koneksi agen lain.
- Kuota habis: hasil faktual terakhir menunjukkan batas provider. Tunggu kuota pulih; login ulang atau rotasi akun bukan solusi. Hasil ini dipertahankan hingga pemeriksaan baru.
- Koneksi terganggu: pemeriksaan terakhir gagal menjangkau provider. Coba Periksa nanti. Pesan aman tidak memuat output CLI, path privat atau secret.

Pemeriksaan koneksi khusus juga menolak mismatch jenis provider/profil sebelum menjalankan probe. Label kompatibilitas provider lain tidak berarti adapter tersedia. Tidak ada panel akun terpisah.

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

`POST /api/v1/agents/:id/connection/verify` memeriksa koneksi milik agen dan mengembalikan `{agent_id, connection_state, message}` yang disanitasi. Roster GET agents menyertakan connection_state, last_checked_at dan verified_at bila tersedia. Pemeriksaan sukses tampil Terhubung berwarna hijau dengan ikon periksa ulang dan jam pemeriksaan. Hasil tersimpan setelah reload/restart; keberhasilan perlu diperiksa ulang setelah lima menit. Credential berubah/hilang membatalkan status keberhasilan lama. Metadata pemeriksaan koneksi lama tersimpan privat di accounts/connection-checks; sertakan direktori ini dalam backup profil. Request tidak sah 401, akun/agen tidak ditemukan 404, konflik login/binding 409, gangguan storage 500. Secret/path/owner profil tidak dimasukkan ke roster atau pesan error.

## Penyimpanan dan kesinambungan

Profil login tetap CLI-owned di service HOME/.local/share/cuan/accounts/<opaque>/home, dengan metadata privat di direktori induknya. Nama direktori accounts adalah detail penyimpanan historis, bukan fitur akun terpisah. Root/profile 0700; metadata 0600 ditulis atomik dengan fsync/rename. Binding dan credential bertahan setelah restart dan berada di luar direktori release. Backup privat harus mencakup direktori ini; pg_dump sendiri tidak mencakup file profil. Percobaan tanpa login yang dibuat sebelum penyederhanaan tetap tidak terlihat dan tidak dihapus secara massal.

Agen lama @pin1 tetap menggunakan koneksi/credential yang sudah ada; tidak dipindahkan atau dihapus. Agen baru wajib memakai profil khusus. Inbox dan task PTY menyelesaikan binding agen lalu memberikan HOME/XDG/config profil tersebut hanya ke child process. Tidak ada os.Setenv HOME global. Child environment menyaring secret organisasi; profile HOME tidak dicampur dengan login agen lain. Login/probe menggunakan direktori profil, sementara task tetap memakai workspace tugas dengan env profil yang sesuai. Mengubah binary task untuk memakai jenis lain pada koneksi Antigravity ditolak.

S26.1 tidak memerlukan migration baru; 19 migration lama tetap utuh. Staging diterbitkan melalui publisher native pada port 8081. Rollback aplikasi tidak menghapus profil, credential atau DB; gunakan source/bundle sebelumnya dengan mekanisme yang sama dan pertahankan file privat. Tidak ada down migration/restore DB untuk rollback kode.

## Bukti dan batas penerimaan

`docs/sprints/evidence/S26/one-agent-targeted.txt` membuktikan binding eksklusif/ownership, retry, penyimpanan rollback metadata, profil yang benar pada child chat dan task PTY, serta penolakan fallback. Credential pada pengujian ini adalah fixture, bukan akun nyata.

`one-agent-ui.json` dan gambar one-agent-<width>-<theme>-roster/login.png membuktikan daftar agen tanpa panel akun, tidak memuat inventaris akun lama, official isolated Google OAuth URL, cancel tanpa baris baru dan tanpa overflow pada 360/390/1280px. Pengujian auto-save/retry frontend menggunakan respons FIXTURE yang diberi label, tidak membuat credential/agen di backend. Native build lulus.

Consent dan login nyata @pin2 telah selesai. real-provider-identities.json membuktikan dua identitas Google berbeda tanpa merekam identitas/secret; pin2-real-nonce-reply.json membuktikan balasan nonce nyata yang tersimpan. real-post-restart.json membuktikan mapping/status/riwayat bertahan setelah restart, @pin1 tetap ada, dan 19 migration dipertahankan. Hasil faktual @pin1 tetap Kuota habis; keberhasilan @pin2 telah kedaluwarsa menjadi Belum diperiksa, tanpa menghapus login. Percobaan nonce awal yang hanya menghasilkan salam tidak dihitung sebagai bukti nonce. S26.1 siap dievaluasi SM; S26.2–S26.5 tetap mengikuti dependency dalam S26.md. Production masih S25; belum ada release S26 atau penyelesaian sprint.

## Inbox progress and safe recovery (S26.4)

Send a message normally from Inbox. The browser saves a private pending request key before sending; the server atomically saves the user message and request, then returns acceptance while the agent works. A progress bubble survives refresh/reopening Inbox. Progress and failure bubbles appear directly below their user message. The Inbox header says Terdaftar and links status guidance to Team; it does not claim provider readiness. Failures show the affected message reference and useful login, quota, provider or interruption guidance. Open that agent's status to diagnose its connection. No automatic account switching or login/consent is triggered.

If the connection drops, use **Segarkan riwayat** first. **Periksa pengiriman yang sama** reuses the original key, at most three attempts in the current view. It cannot launch accepted work again. Reload keeps the pending key in this browser's local storage. Do not clear browser storage or deliberately send a new message while delivery is uncertain. Pending content remains in local storage until reconciled; use a trusted browser profile. Browser storage must be available to send safely.

Provider execution has a 45-second limit; unfinished durable requests become interrupted after 60 seconds when read. A process restart can leave the provider outcome unknown. Cuan conservatively does not replay that work. Refresh checks the persisted history. After checking the outcome/status, a deliberately new message is new work and may invoke the provider again. Quota failures require waiting for provider recovery, never account rotation or repeated consent.

For API clients, persist a UUID `request_id` before `POST /api/v1/chat/conversations/{id}/messages`, with `message`. Retrying the same conversation/key/content returns the original request/message/reply; changing content under that key returns HTTP 409. New keyed work returns HTTP 202 while running; GET on the same URL returns `requests` with state `running`, `completed`, `failed` or `interrupted`, safe error, message/reply UUID and timestamps. Poll at a modest interval (UI: 3.5 seconds); reconcile rather than replay. Legacy clients without request_id retain synchronous responses but cannot safely retry a lost initial response: migrate clients to explicit keys. Request IDs are correlation values, not credentials; existing owner authentication still applies.

### Schema/configuration and rollback

S26.4 needs additive migration 000020 (`inbox_requests`). Existing message columns cannot provide atomic unique request correlation and execution state. Keep `CUAN_AUTO_MIGRATE=false`; inspect `go run ./cmd/migrate --dry-run`, take a private database backup and explicitly apply pending up migrations with the migrator and a private `DATABASE_URL`. Never place URLs/passwords in shell history, docs or command arguments. Keep all original 19 migrations unchanged. Staging is the original `cuan-s25` sandbox at https://cuan-s25-dojo.fural.space, isolated database `cuan_s25`, HTTP port 8081. S26.5 authorizes production only after all approved staging, compatibility, private-backup and actual rollback gates pass; earlier tasks did not authorize it.

Application rollback uses the existing native atomic release procedure in the deployment guide; leave additive schema/data in place, keep auto-migrate disabled, and never run a down migration during ordinary rollback. Older application versions ignore the new table but lack safe retry UX: finish/reconcile pending work before rollback and avoid resending uncertain messages. Health/readiness, exact served assets, retained reply UUIDs, account mapping and private credential hashes must be verified after staging restart. No new provider/config secrets are needed.


## Native S26 release and rollback

Use `scripts/fural-deploy.sh` through supported `fural-agent repos ship`, after the approved branch has been integrated into stable main and all SSOT gates pass. A push alone is not deployment. The activator defaults to exact schema20 readiness; `--schema-count 19` is only for an installation still on schema19. Application rollback on schema20 must continue requiring **20**, even when the selected S25 binary knows only 19 migrations. Older code ignores the additive table. No automatic migration, down migration or DB restore belongs to activation/rollback.

Before any production schema operation, privately back up the production database with `pg_dump --format=custom --no-owner --no-privileges` and validate its archive with `pg_restore --list`; keep it 0600 in a 0700 directory outside web/static/release payloads. Also back up the service HOME's account profiles, connection-check metadata and legacy CLI credential privately; a database dump does not contain those files. Never copy another owner's profile. Retain staging and production credentials separately. Do not expose credential values or fingerprints in reports.

Quiesce/reconcile pending Inbox work before a release or rollback; old clients lack keyed replay. Keep application writes and all retained profiles/history; backup restoration is a separate disaster-recovery operation. Inspect the pending additive 000020 against the original 19 unchanged files, apply explicitly using a private environment, then require schema20. An old S25 process must remain usable against schema20 before production is allowed.

Activation validates the exact manifest file set and every checksum under an exclusive nonblocking host lock, validates/creates a private database archive, swaps the current symlink atomically, restarts the service and verifies health, database readiness/schema and exact HTML. Failed readiness restores the old pointer and restarts. Explicit rollback validates the retained payload too. Verify public JS/CSS bytes, owner API, factual statuses, mapping and stored reply UUIDs independently after activation; do not run consent/provider probes to manufacture readiness during quota limits.

The activator supports `--base`, `--service`, `--port`, and `--schema-count` for an isolated native drill using a separate systemd service and the isolated staging DB. Never point a drill at the production service/base/database. On production use `sudo python3 /home/ubuntu/.local/share/cuan-s25/activate-release.py --rollback --schema-count 20`; reactivate with `--release <full SHA> --schema-count 20`. Retain the previous release until the actual two-direction drill and continuity checks pass. Record outcomes and limitations in S26.md, then present the Sprint Review demo to PO; do not mark the sprint completed automatically.
