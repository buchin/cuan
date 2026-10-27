=== CUAN SPRINT 2 (S02.5) EVIDENCE: OTOMASI SINKRONISASI README.MD REPOSITORI GITHUB & DIREKTORI ABADI VERIFIED ===
Timestamp: 2026-10-16T21:08:50+07:00
Task ID: ahodibvynxnjwllhzruakmeh
Title: S02.5 — Otomasi Sinkronisasi README.md Repositori GitHub & Direktori Abadi
Author: Buchin <buchin@dropsugar.com>
Repository: https://github.com/buchin/cuan
Fural Repo ID: RPcd484fc4229ae192d17a0f01

1. RUNNER & SERVICE INTERNAL OTOMATIS SINKRONISASI (BACKEND GO):
   - Package: `internal/sponsorship` (`syncer.go`, `syncer_test.go`)
   - Core Runner: `Syncer` struct yang mengorkestrasi pembaruan seksi `## Backers & Sponsors` pada file `README.md` dan direktori arsip abadi `sponsors/`.
   - Konfigurasi Fleksibel via Environment & SyncerConfig:
     * `CUAN_README_PATH`: Path absolut/relatif ke file `README.md` (default: `README.md`).
     * `CUAN_SPONSORS_ARCHIVE_DIR`: Path ke direktori arsip abadi (default: `sponsors`).
     * `CUAN_AUTO_SYNC_README`: Flag aktivasi sinkronisasi otomatis saat moderasi (default: `true`).
     * `CUAN_SYNC_GIT_COMMIT`: Flag otomasi Git commit jika dijalankan di environment repo mandiri.
     * `GIT_AUTHOR_NAME`: Nama author human (`Buchin`).
     * `GIT_AUTHOR_EMAIL`: Email author human (`buchin@dropsugar.com`).

2. PEMBARUAN README.MD IDEMPOTEN & PENANDA BATAS AMAN:
   - Marker Batas Komentar HTML:
     * Penanda Awal: `<!-- SPONSORS:START -->`
     * Penanda Akhir: `<!-- SPONSORS:END -->`
   - Logika Idempotensi `UpdateReadmeContent`:
     * Jika penanda sudah ada: Mengganti konten eksklusif hanya di antara penanda `<!-- SPONSORS:START -->` dan `<!-- SPONSORS:END -->`.
     * Jika penanda belum ada namun seksi `## Backers & Sponsors` ditemukan: Menyisipkan penanda dan tabel tepat di bawah judul seksi.
     * Jika seksi belum ada sama sekali: Menyisipkan seksi `## Backers & Sponsors` tepat sebelum `## License` dengan pembatas horizontal `---`.
     * Hasil pengujian uji coba 5x berturut-turut: Menghasilkan `changed == false` (zero-diff) ketika data sponsor tidak berubah. Seluruh seksi dokumen di luar penanda (Overview, Installation, Health Probes, Disaster Recovery, License) tetap utuh 100%.

3. KONSISTENSI URUTAN LEADERBOARD & KEAMANAN FORMAT MARKDOWN:
   - Aturan Urutan: Konsisten 100% dengan leaderboard web (`ORDER BY amount_idr DESC, paid_at ASC NULLS LAST, created_at ASC`).
   - Format Tabel Markdown Bersih & Responsif:
     * Kolom: `| Peringkat | Brand & Tautan | Slogan | Kontribusi | Tier |`
     * Peringkat Visual: `🥇 #1`, `🥈 #2`, `🥉 #3`, `⭐ #4`, `⭐ #5`, `#6+`.
     * Tier Badges:
       - Peringkat 1: `🌟 Platinum Founding Backer`
       - Peringkat 2: `🥇 Gold Founding Backer`
       - Peringkat 3: `🥈 Silver Founding Backer`
       - Peringkat 4-5: `🥉 Bronze Founding Backer`
       - Kontribusi >= Rp 1.000.000: `🥇 Gold Backer`
       - Kontribusi >= Rp 250.000: `🥈 Silver Backer`
       - Kontribusi >= Rp 50.000: `🥉 Bronze Backer`
       - Kontribusi standar: `🎖️ Founding Backer`
     * Keamanan Tautan & Teks:
       - Fungsi `FormatSafeMarkdownLink`: Mengharuskan skema `http://` atau `https://` (auto-prepend `https://`).
       - Memblokir tegas skema berbahaya seperti `javascript:`, `data:`, `vbscript:` (fallback ke teks tebal biasa `**Brand**`).
       - Karakter pipa (`|`) dan baris baru (`\n`) pada nama brand dan slogan dibersihkan secara otomatis untuk mencegah kerusakan struktur tabel Markdown.
     * Empty State: Menampilkan pesan informatif dan ajakan menjadi backer pertama jika batch belum memiliki sponsor terdaftar.

4. DIREKTORI PUBLIK & ARSIP ABADI /sponsors:
   - Direktori Fisik Repositori: `/sponsors`
   - Berkas Master Index: `sponsors/README.md`
     * Menjelaskan prinsip arsip abadi: Seluruh donatur/sponsor dicatat dalam riwayat Git repositori sehingga tetap abadi (*immutable proof of backing*).
     * Papan Integritas & Jaminan Anti-Judol: Kebijakan toleransi nol (*zero-tolerance*) terhadap judi online dan pornografi.
     * Tabel Agregat Batch: Daftar seluruh batch rilis Cuan, status aktif/tutup, tautan berkas arsip, jumlah sponsor terdaftar, dan total dana terhimpun.
   - Berkas Ledger Per-Batch: `sponsors/v1.0.md` (dan `{batch_slug}.md`)
     * Rincian metadata batch (slug, status, total sponsor, total dana, stempel waktu sinkronisasi).
     * Tabel daftar lengkap backer beserta tanggal verifikasi pembayaran lunas.
     * Mekanisme pencegahan timestamp flapping: Stempel waktu dipertahankan stabil jika tidak ada perubahan data sponsor.

5. INTEGRASI OTOMATIS SAAT MODERASI OWNER & ENDPOINT MANUAL:
   - Integrasi Otomatis saat Approve:
     * Endpoint: `POST /api/admin/sponsors/:id/approve`
     * Saat Owner mengeklik Approve, sponsor berstatus `approved` dan backend secara otomatis memanggil `SyncReadmeAndArchive(...)`.
     * Response JSON menyertakan flag konfirmasi: `"readme_synced": true`.
   - Integrasi Otomatis saat Reject:
     * Endpoint: `POST /api/admin/sponsors/:id/reject`
     * Jika sponsor sebelumnya disetujui lalu dibatalkan/ditolak karena pelanggaran judol, backend otomatis melakukan re-sync untuk menghapus sponsor dari `README.md` dan arsip seketika.
   - Endpoint Sinkronisasi Manual:
     * Endpoint: `POST /api/admin/sponsors/sync-readme` (alias: `POST /api/v1/admin/sponsors/sync-readme`)
     * Terproteksi autentikasi single-owner (`auth.Middleware`). Akses tanpa token menghasilkan 401 Unauthorized.
     * Mengembalikan objek `SyncResult`: `sponsors_count`, `files_modified`, `readme_updated`, `archive_updated`, `message`.
   - Dukungan CLI Server:
     * Command: `cuan-server --sync-sponsors`
     * Memungkinkan operator/admin menjalankan sinkronisasi offline atau terjadwal via cron daemon.
   - Tombol Sinkronisasi pada UI Dashboard Owner:
     * Di panel `AdminSponsorPanel.tsx`, tersedia tombol biru **Sync README & Arsip** berdampingan dengan tombol Refresh Data.

6. BUKTI PENGUJIAN TARGETED TESTS GO (LULUS 100%):

Command:
go test -v ./internal/sponsorship -run "TestFormatIDR|TestRankAndTierBadges|TestFormatSafeMarkdownLink|TestRenderSponsorsMarkdown|TestUpdateReadmeContent|TestSyncer"
Output:
=== RUN   TestFormatIDR
--- PASS: TestFormatIDR (0.00s)
=== RUN   TestRankAndTierBadges
--- PASS: TestRankAndTierBadges (0.00s)
=== RUN   TestFormatSafeMarkdownLink
--- PASS: TestFormatSafeMarkdownLink (0.00s)
=== RUN   TestRenderSponsorsMarkdown
--- PASS: TestRenderSponsorsMarkdown (0.00s)
=== RUN   TestUpdateReadmeContent_Idempotency
--- PASS: TestUpdateReadmeContent_Idempotency (0.00s)
=== RUN   TestSyncer_EndToEndSync
--- PASS: TestSyncer_EndToEndSync (0.00s)
=== RUN   TestSyncer_GitCommit
--- PASS: TestSyncer_GitCommit (0.05s)
PASS
ok  	github.com/buchin/cuan/internal/sponsorship	0.061s

Command:
go test -v ./internal/server -run "TestReadmeSync"
Output:
=== RUN   TestReadmeSync_AdminApproveTriggersSync
--- PASS: TestReadmeSync_AdminApproveTriggersSync (0.06s)
=== RUN   TestReadmeSync_ManualSyncEndpointAndAuthGuard
--- PASS: TestReadmeSync_ManualSyncEndpointAndAuthGuard (0.06s)
PASS
ok  	github.com/buchin/cuan/internal/server	0.127s

Full Fast Test Suite:
Command: ./scripts/test.sh --fast
Result: PASS (Seluruh package Go internal/auth, internal/db, internal/git, internal/sponsorship, internal/server, internal/task, internal/runner, serta verifikasi build React Vite frontend lulus 100%).

7. BUKTI VERIFIKASI BUILD BINARY & FRONTEND ASSETS:
   - Binary Backend:
     * `bin/cuan-server`: Built OK
     * `bin/cuan-runner`: Built OK
     * `bin/cuan-migrate`: Built OK
   - Frontend Bundle:
     * `web/dist/index.html`: 0.55 kB
     * `web/dist/assets/index-gfuQDoXZ.css`: 48.17 kB
     * `web/dist/assets/index-TZOOGGYW.js`: 373.51 kB
     * Zero TypeScript errors, zero lint violations.

8. BUKTI EKSEKUSI PEMBARUAN FILE REAL DI REPOSITORI:
   - File Terperbarui:
     * `README.md`: Seksi `## Backers & Sponsors` telah ditambahkan di atas `## License` dengan penanda komentar `<!-- SPONSORS:START -->` dan `<!-- SPONSORS:END -->`.
     * `sponsors/README.md`: Master arsip direktori abadi dengan tabel batch dan jaminan anti-judol.
     * `sponsors/v1.0.md`: Ledger permanen untuk batch `v1.0 Founding Backers`.
