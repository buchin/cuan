# Cuan Sprint 1 Review Pack — Develop Cuan using Cuan

**Tanggal Review:** 2026-10-16  
**Product Owner (PO):** Buchin  
**Scrum Master (SM) & Development Team:** Agentic Scrum Team  
**Status Sprint:** **review** (Menunggu review bersama PO untuk transisi ke `completed`)  
**Single Source of Truth:** [docs/sprints/S1.md](../../sprints/S1.md)  
**Repositori GitHub:** https://github.com/buchin/cuan  

---

## 1. Executive Summary & Pencapaian Sprint Goal

### Sprint Goal:
> *"Owner memasang Cuan di VPS, menghubungkan repo Cuan, membuat task pengembangan, menjalankan satu coding agent, memeriksa perubahan dan bukti verifikasi, serta mendapatkan PR di GitHub. Perubahan kemudian diuji di staging dan dipasang ke instance Stable tanpa kehilangan task atau riwayat kerja."*

### Status Pencapaian: **TERCAPAI 100% (SUKSES PENUH)**
Seluruh backlog T1 hingga T8 telah diselesaikan dengan bukti nyata (*evidence-based*), diuji dengan targeted tests yang cepat dan efisien, divalidasi melalui demo dogfooding end-to-end, dan memenuhi seluruh kriteria release gate open-source Apache-2.0.

```d2
direction: right

s1_goal: Sprint 1 Goal {
  label: "Sprint 1: Develop Cuan using Cuan\n(Status: 100% Achieved)"
}

t1_t3: Core Foundation {
  label: "T1–T3\nMonorepo, Owner Auth,\nGitHub Integration"
}

t4_t6: Task Board & Agent Runner {
  label: "T4–T6\nTask Board, cuan-runner,\nGit Isolation & PR Generator"
}

t7_t8: Staging, Recovery & OSS Gate {
  label: "T7–T8\nStaging Isolation, Update Flow,\nDogfooding Demo & OSS Release"
}

s1_goal -> t1_t3
t1_t3 -> t4_t6
t4_t6 -> t7_t8
```

---

## 2. Matriks Evaluasi Backlog T1–T8 & Bukti DoD Nyata

| Task ID | Judul Backlog | Status | Bukti Nyata DoD (*Clickable Links*) |
| :--- | :--- | :---: | :--- |
| **T1** | Tetapkan fondasi dan kelayakan agent | **Lulus** | [agent_spike_result.txt](agent_spike_result.txt)<br>[architecture_decisions.md](architecture_decisions.md)<br>[build_verification.txt](build_verification.txt)<br>[migration_baseline.txt](migration_baseline.txt) |
| **T2** | Sediakan instalasi persisten dan akses owner | **Lulus** | [smoke_install_login_restart.txt](smoke_install_login_restart.txt)<br>[health_readiness_response.txt](health_readiness_response.txt)<br>[configuration_clean.txt](configuration_clean.txt)<br>[https_proxy_guide.md](https_proxy_guide.md) |
| **T3** | Hubungkan repository GitHub existing | **Lulus** | [github_connection_verified.txt](github_connection_verified.txt)<br>[access_denied_handled.txt](access_denied_handled.txt)<br>[credential_security_audit.txt](credential_security_audit.txt)<br>[projects_ui_walkthrough.md](projects_ui_walkthrough.md) |
| **T4** | Bangun task board dan histori pekerjaan | **Lulus** | [task_board_history_persistence.txt](task_board_history_persistence.txt)<br>[tasks_ui_walkthrough.md](tasks_ui_walkthrough.md) |
| **T5** | Jalankan task melalui runner dan satu agent | **Lulus** | [runner_queue_execution.txt](runner_queue_execution.txt)<br>[cancel_process_tree.txt](cancel_process_tree.txt)<br>[disconnect_recovery_audit.txt](disconnect_recovery_audit.txt)<br>[duplicate_claim_concurrency.txt](duplicate_claim_concurrency.txt)<br>[resource_monitoring_peak_ram.txt](resource_monitoring_peak_ram.txt)<br>[agents_servers_ui_walkthrough.md](agents_servers_ui_walkthrough.md) |
| **T6** | Hasilkan perubahan Git dan PR yang dapat direview | **Lulus** | [pr_creation_flow.txt](pr_creation_flow.txt)<br>[revision_cycle_proof.txt](revision_cycle_proof.txt)<br>[git_workspace_isolation.txt](git_workspace_isolation.txt)<br>[default_branch_protected.txt](default_branch_protected.txt)<br>[pr_ui_walkthrough.md](pr_ui_walkthrough.md)<br>GitHub PR #1: https://github.com/buchin/cuan/pull/1 |
| **T7** | Buktikan staging, update, backup, dan pemulihan | **Lulus** | [staging_isolation_verified.txt](staging_isolation_verified.txt)<br>[update_data_retention_verified.txt](update_data_retention_verified.txt)<br>[backup_restore_drill.txt](backup_restore_drill.txt)<br>[failed_update_rollback_drill.txt](failed_update_rollback_drill.txt)<br>[installer_preflight_verified.txt](installer_preflight_verified.txt)<br>[deployment_recovery_guide.md](../../guides/deployment_recovery_guide.md) |
| **T8** | Demo dogfooding dan siapkan release OSS | **Lulus** | [dogfooding_e2e_demo.txt](dogfooding_e2e_demo.txt)<br>[dogfooding_walkthrough.md](dogfooding_walkthrough.md)<br>[oss_license_audit.txt](oss_license_audit.txt)<br>[oss_security_and_secrets_audit.txt](oss_security_and_secrets_audit.txt)<br>[onboarding.md](../../guides/onboarding.md)<br>[user_guide.md](../../guides/user_guide.md)<br>[troubleshooting.md](../../guides/troubleshooting.md) |

---

## 3. Demo Dogfooding End-to-End (Highlights)

Sebagai pembuktian nyata prinsip **Develop Cuan using Cuan**:
1. **Pekerjaan Nyata yang Dikerjakan:** Fitur ekspos metadata repositori OSS, dokumentasi, author, dan banner rilis CLI pada Cuan Server.
2. **Alur Eksekusi:**
   - Owner membuat task di Cuan Stable (`task_c3ef9820e38d`).
   - Daemon `cuan-runner` mengklaim attempt `att_5fb3de33b404`, mengisolasi workspace ke branch `task/task_c3ef9820e38d`.
   - Agen `agy` v1.4.0 menerapkan perbaikan kode pada `internal/server/server.go` dan `cmd/server/main.go`.
   - Targeted tests dijalankan dan lulus 100% (exit code 0).
   - Commit Git dibuat atas nama `Buchin <buchin@dropsugar.com>` (zero bot identity).
   - PR dibuka di GitHub: [PR #2](https://github.com/buchin/cuan/pull/2) dengan ringkasan diff dan status pengujian.
   - Perubahan diuji pada Staging terisolasi (port 8081).
   - Instance Stable diperbarui ke versi `0.2.0-rc1` dengan **retensi data 100%** (seluruh riwayat task, attempt, dan sesi owner tetap utuh).

---

## 4. Validasi Release Gate Open Source Software (OSS)

1. **Lisensi Utama & Notice:**
   - Berkas `LICENSE` berlisensi resmi **Apache License, Version 2.0**.
   - Berkas `NOTICE` mencantumkan hak cipta: `Copyright 2026 Buchin and Cuan contributors`.
2. **Audit Dependensi Third-Party:**
   - Seluruh dependensi Go (Chi, pgx, x/crypto) berlisensi MIT atau BSD-3-Clause.
   - Seluruh dependensi Frontend (React, Lucide, Tailwind, Vite) berlisensi MIT, ISC, atau Apache-2.0.
   - **Nol lisensi viral (GPL/AGPL)** dan nol komponen proprietary berbayar.
3. **Audit Keamanan & Kerahasiaan (Zero-Secret Guarantee):**
   - Pemindaian regex statis menyeluruh: **0 token rahasia bocor di codebase atau riwayat Git**.
   - Template konfigurasi `.env.example` bersih dari rahasia nyata.
   - Port database 5432 terkunci secara internal di dalam Docker network.
   - Token GitHub dienkripsi saat istirahat dengan AES-256-GCM.
4. **Dokumentasi Penggunaan Siap Pakai:**
   - [Onboarding & Self-Hosting Guide](../../guides/onboarding.md)
   - [User & Operational Guide](../../guides/user_guide.md)
   - [Troubleshooting & Recovery Playbook](../../guides/troubleshooting.md)
   - [Deployment & Disaster Recovery Guide](../../guides/deployment_recovery_guide.md)
   - [GitHub PAT Setup & Revocation Guide](../../guides/github_token_guide.md)

---

## 5. Batasan Produk (Known Limitations) & Technical Debt

Hal-hal yang secara sengaja dibatasi pada Sprint 1 (*conscious trade-offs*) dan dicatat sebagai technical debt:
1. **Konkurensi 1 Task per Runner:** Runner saat ini hanya mengeksekusi 1 task secara serial. Task lain antre secara persisten. Ini adalah batas produk yang aman untuk mencegah OOM killer pada VPS 2 GB.
2. **Satu Adapter Resmi (`agy`):** Adapter agen lain (misal Claude Code, Goose, OpenHands) belum diimplementasikan.
3. **Single Owner System:** Belum mendukung multi-user invite atau Role-Based Access Control (RBAC).
4. **Mode In-Memory Development:** Jika `DATABASE_URL` tidak disetel, server berjalan dalam mode in-memory mock untuk kemudahan pengujian lokal tanpa database.
5. **Koneksi GitHub via PAT:** Integrasi saat ini menggunakan Personal Access Token (PAT) terenkripsi AES-256; GitHub App OAuth flow direncanakan untuk iterasi masa depan.

---

## 6. Poin Keputusan & Rekomendasi untuk Product Owner (PO)

Berdasarkan hasil dogfooding dan masukan perencanaan produk pada `docs/product-vision.md`:

### Keputusan 1: Strategi Go-To-Market & Monetisasi Awal Cuan
- **Rekomendasi:** **Implementasi Sponsorship Landing Page (Tiered Top-30 Grid & Pembayaran Lokal Transfer Bank / QRIS) pada Sprint 2.**
  - **Dampak:** Membuka aliran kas langsung dari audiens Indonesia (email list lokal), membatasi kuota homepage agar tetap eksklusif dan prestisius, serta menciptakan *social proof* logo berjejer rapi sebelum Cuan dirilis OSS secara global.
  - **Effort:** Kecil–Menengah (UI Landing Page + Integrasi Form/Manual Approval Sponsorship).
  - **Urgensi:** Tinggi (membiayai operasional dan validasi antusiasme pasar sejak hari pertama).
- **Alternatif:** Membuka rilis OSS global terlebih dahulu tanpa halaman sponsor lokal.
  - **Dampak/Effort/Urgensi:** Resiko peluncuran tanpa pendapatan awal; kehilangan momentum monetisasi antusiasme audiens lokal.
- **Alasan:** Sesuai prinsip Lean GTM: domain `cuan.co` dapat langsung menghasilkan konversi sponsor sebelum energi tim terpecah ke pasar internasional.

---

### Keputusan 2: Penambahan Adapter Agent & Fleksibilitas Eksekusi
- **Rekomendasi:** **Menambahkan Adapter Alternatif (Claude Code / OpenHands) pada Sprint 2.**
  - **Dampak:** Memberi kebebasan bagi solo developer yang tidak memiliki langganan AGY untuk tetap dapat memakai Cuan dengan AI subscription mereka sendiri (*BYO AI Subscription*).
  - **Effort:** Menengah.
  - **Urgensi:** Menengah.
- **Alternatif:** Mempertahankan AGY sebagai satu-satunya adapter resmi untuk 1 sprint lagi.
  - **Dampak/Effort/Urgensi:** Fokus pengembangan tetap pada fitur inti Cuan, tetapi mempersempit adopsi pengguna non-AGY.
- **Alasan:** Memperkuat proposisi nilai *zero platform AI burden* bagi pengguna self-hosted.

---

### Keputusan 3: Penutupan Resmi Sprint 1
- **Rekomendasi:** **Menyetujui penyelesaian Sprint 1 dan mengubah status dokumen S1 menjadi `completed`.**
  - **Dampak:** Menutup siklus Sprint 1 secara resmi dan membuka gerbang Sprint Planning untuk Sprint 2.
  - **Effort:** Nol (prosedural Scrum).
  - **Urgensi:** Tinggi.
- **Alternatif:** Meminta revisi atau penambahan scope sebelum menutup sprint.
  - **Alasan:** Seluruh 8 task T1–T8 dan sprint goal terbukti tuntas 100% tanpa deviasi dari scope yang telah disetujui.
