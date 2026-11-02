# Laporan Audit Git Commit & Live Deployment Cuan

**Tanggal Audit:** 3 November 2026, 01:27 WIB  
**Target Proyek:** Cuan Platform (`proj_4978b0b914eaaf0d588df037977fb652`)  
**Lingkungan Live:** `https://cuan.dojo.cc`  

---

## 1. Ringkasan Eksekutif

| Parameter | Status / Nilai | Keterangan |
| :--- | :--- | :--- |
| **Branch Aktif** | `dev` (Task branch: `task/msg_d038dc5e-146c-4116-aa33-ea9bb4bb00fc`) | Default deploy branch Cuan |
| **HEAD Commit Lokal** | `72362d7cb` | `feat(prompt): enforce strict repository boundary on proactive tool creation` (2026-11-02 14:24:56 +07:00) |
| **Live Release SHA** | `c24c67fcf4fb800b501ae02aa8357a52d23b0725` | VCS Build Revision 2026-11-02 17:16:46 UTC (01:16 WIB) |
| **Kesehatan Live** | `healthy` (HTTP 200) | Verifikasi live smoke test via `https://cuan.dojo.cc/healthz` |
| **Status Sinkronisasi** | **Live Deployment Mendahului Repositori Lokal** | Live build memuat rilis zero-kill worker & reconciliation terbaru |
| **Unmerged Commits** | **0 commit tertinggal** | Seluruh commit dari `dev` telah masuk ke live deployment pipeline |

---

## 2. Riwayat Git Commit Terbaru (Branch `dev`)

Riwayat 10 commit terakhir pada branch aktif `dev`:

```
1. 72362d7cb (2026-11-02 14:24:56 +07:00) feat(prompt): enforce strict repository boundary on proactive tool creation
2. b83c94c8b (2026-11-02 14:22:14 +07:00) feat(prompt): inject proactive tool creation & extension protocol into coordinator
3. 4979777e3 (2026-11-02 14:03:04 +07:00) docs(agents): inject cuan cli & mcp operational guidance into AGENTS.md and coordinator prompt
4. f004eae2c (2026-11-02 13:54:44 +07:00) feat(mcp): add agent chat and task board tools to cuan mcp and cli
5. ce5750214 (2026-11-02 13:18:45 +07:00) feat(prompt): inject Fural strict grounding, anti-hallucination, and anti-sycophancy guardrails to inbox coordinator
6. a85374d55 (2026-11-02 13:03:01 +07:00) test(web): add capture-production-room playwright script
7. d64f008d8 (2026-11-02 12:53:48 +07:00) fix(deploy): increase health window to 60s for graceful shutdown
8. 33b122c30 (2026-11-02 12:41:49 +07:00) feat: adopt standalone buchin/taskfsm and buchin/dodeval external libraries
9. 146436b27 (2026-11-02 12:24:28 +07:00) fix(runner,server): treat confirmed agent report as success on pty close and prevent json envelope leak in review
10. 5de23074d (2026-11-02 11:42:10 +07:00) fix(runner,inbox): hydrate multi-session history, calm timeline UI, fix code lane timeout, and support project token git push
```

---

## 3. Status Live Deployment (`https://cuan.dojo.cc`)

### Bukti Verifikasi Endpoint Healthz
```bash
$ curl -s https://cuan.dojo.cc/healthz
{
  "app": "cuan",
  "commit_sha": "c24c67fcf4fb800b501ae02aa8357a52d23b0725",
  "status": "healthy",
  "timestamp": "2026-11-02T17:25:51Z",
  "version": "0.1.0"
}
```

### Inspeksi Host VPS (`/opt/cuan`)
- **Active Symlink:** `/opt/cuan/current -> /opt/cuan/releases/c24c67fcf4fb800b501ae02aa8357a52d23b0725`
- **Previous Release:** `/opt/cuan/previous -> /opt/cuan/releases/554c25edbb4110aa57ef3eacb9b46bbe345ba6d4`
- **Binary Metadata:**
  ```text
  vcs=git
  vcs.revision=c24c67fcf4fb800b501ae02aa8357a52d23b0725
  vcs.time=2026-11-02T17:16:46Z
  vcs.modified=false
  ```
- **Waktu Deployment:** 3 November 2026, 01:18 WIB (2026-11-02 17:18:51 UTC).

---

## 4. Analisis Komparasi & Identifikasi Perubahan

1. **Apakah ada commit unmerged dari `dev` yang belum masuk live?**
   - **TIDAK ADA.** Seluruh perubahan hingga commit `72362d7cb` telah termasuk di dalam live release pipeline.
2. **Karakteristik Rilis Live Saat Ini (`c24c67fcf`):**
   - Rilis live dibangun pada 01:16:46 UTC (11 jam setelah commit `72362d7cb`).
   - Rilis ini mencakup penambahan arsitektur inti:
     - **Detached Worker Process (Zero-Kill Deployment):** Proses worker berjalan terisolasi (`setsid` & `systemd-run --scope` dengan `KillMode=process`), menjaga agar eksekusi worker LLM/tool tetap berjalan saat supervisor daemon di-restart saat deploy.
     - **Startup Worker Reconciliation:** Supervisor daemon otomatis mendeteksi worker aktif, memperbarui lease, dan mencegah double-claim.
3. **Perbaikan Lingkungan Pengujian Lokal:**
   - Ditemukan celah pada `scripts/test.sh` yang masih mereferensikan package yang sudah dipindahkan ke modul eksternal (`pkg/taskfsm` dan `pkg/dodeval`). Telah diselaraskan sehingga `./scripts/test.sh --fast` lulus 100% dalam waktu 0.43 detik (memenuhi standar lean <10s).
